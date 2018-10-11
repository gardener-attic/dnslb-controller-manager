// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"strings"
	"sync"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/provider"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"

	"github.com/gardener/dnslb-controller-manager/pkg/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

var maxChangeCount = 20

type AWSProviderInfo struct {
	zoneid string
}

func (this *AWSProviderInfo) String() string {
	return this.zoneid
}

type Change struct {
	*route53.Change
	Done DoneHandler
}

type AWSProvider struct {
	log.LogCtx
	ptype  DNSProviderType
	config Properties

	dryrun bool
	lock   sync.Mutex
	sess   *session.Session
	r53    *route53.Route53
	zones  map[string]*route53.HostedZone

	changes map[string]map[string][]*Change
}

func New(cfg *config.CLIConfig, logctx log.LogCtx) (*AWSProvider, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewEnvCredentials(),
	})
	if err != nil {
		return nil, err
	}
	return (&AWSProvider{}).new(cfg, sess, logctx)
}

func NewForSession(sess *session.Session, cfg *config.CLIConfig, logctx log.LogCtx) (*AWSProvider, error) {
	return (&AWSProvider{}).new(cfg, sess, logctx)
}

func (a *AWSProvider) new(cfg *config.CLIConfig, sess *session.Session, logctx log.LogCtx) (*AWSProvider, error) {
	a.LogCtx = logctx
	a.sess = sess
	a.r53 = route53.New(sess)
	zones, err := a.Zones()
	if err == nil {
		a.Infof("found %d zone(s)", len(zones))
		for n, z := range zones {
			a.Infof("found zone '%s': %s", n, aws.StringValue(z.Name))
		}
	} else {
		zones = map[string]*route53.HostedZone{}
		a.Errorf("cannot get zones: %s", err)
	}
	a.zones = zones
	a.dryrun = cfg.DryRun
	a.config = Properties{}
	return a, err
}

func (a *AWSProvider) Lock() {
	a.lock.Lock()
}
func (a *AWSProvider) Unlock() {
	a.lock.Unlock()
}

func (a *AWSProvider) GetConfig() Properties {
	return a.config
}
func (a *AWSProvider) GetType() DNSProviderType {
	return a.ptype
}
func (a *AWSProvider) GetDomains() StringSet {
	set := StringSet{}
	for _, zone := range a.zones {
		name := aws.StringValue(zone.Name)
		set.Add(name[:len(name)-1])
	}
	return set
}

func (a *AWSProvider) Match(dns string) (ProviderInfo, int) {
	zoneid, n := a.getZoneId(dns)
	if zoneid == "" {
		return nil, -1
	}
	return &AWSProviderInfo{zoneid}, n
}

func (a *AWSProvider) GetDNSSets() (map[string]*DNSSet, error) {
	dnssets := map[string]*DNSSet{}
	cnt := 0

	for zoneid := range a.zones {
		err := a.AddAllRecords(zoneid, dnssets)
		if err != nil {
			return nil, err
		}
		a.Debugf("found %d entries in zone %s", len(dnssets)-cnt, zoneid)
		cnt = len(dnssets)
	}

	return dnssets, nil
}

func (a *AWSProvider) ExecuteRequests(reqs []*ChangeRequest) error {
	a.Reset()

	a.Lock()
	defer a.Unlock()

	for _, r := range reqs {
		switch r.Action {
		case R_CREATE:
			a.addChange(route53.ChangeActionCreate, r)
		case R_UPDATE:
			a.addChange(route53.ChangeActionUpsert, r)
		case R_DELETE:
			a.addChange(route53.ChangeActionDelete, r)
		}
	}
	if a.dryrun {
		a.Infof("no changes in dryrun mode for AWS")
		return nil
	}
	return a.submitChanges()
}

func (a *AWSProvider) addChange(action string, req *ChangeRequest) {
	rtype := req.Type
	name := alignHostname(MapToProvider(rtype, req.DNS))
	set := req.DNS.Sets[rtype]
	zoneid := req.DNS.Info.(*AWSProviderInfo).zoneid
	if len(set.Records) == 0 {
		return
	}
	a.Infof("%s %s record set %s[%s]: %s", action, rtype, name, zoneid, set.RecordString())
	change := &route53.Change{
		Action: aws.String(action),
		ResourceRecordSet: &route53.ResourceRecordSet{
			Name: aws.String(name),
		},
	}

	change.ResourceRecordSet.Type = aws.String(rtype)
	change.ResourceRecordSet.TTL = aws.Int64(set.TTL)
	change.ResourceRecordSet.ResourceRecords = make([]*route53.ResourceRecord, len(set.Records))
	for i, r := range set.Records {
		change.ResourceRecordSet.ResourceRecords[i] = &route53.ResourceRecord{
			Value: aws.String(r.Value),
		}
	}

	if a.changes == nil {
		a.changes = map[string]map[string][]*Change{}
	}
	names := a.changes[zoneid]
	if names == nil {
		names = map[string][]*Change{}
		a.changes[zoneid] = names
	}
	names[name] = append(names[name], &Change{Change: change, Done: req.DNS.Done})
}

func (a *AWSProvider) Reset() {
	a.Lock()
	defer a.Unlock()
	a.changes = nil
}

func (a *AWSProvider) Zones() (map[string]*route53.HostedZone, error) {
	zones := make(map[string]*route53.HostedZone)

	aggr := func(resp *route53.ListHostedZonesOutput, lastPage bool) bool {
		for _, zone := range resp.HostedZones {
			id := strings.Split(aws.StringValue(zone.Id), "/")
			zones[id[len(id)-1]] = zone
		}
		return true
	}

	err := a.r53.ListHostedZonesPages(&route53.ListHostedZonesInput{}, aggr)
	if err != nil {
		return nil, err
	}
	return zones, nil
}

func alignHostname(host string) string {
	if strings.HasSuffix(host, ".") {
		return host
	}
	return host + "."
}

func (a *AWSProvider) getZoneId(hostname string) (string, int) {
	match := ""
	found := ""
	hostname = alignHostname(hostname)
	for zoneid, zone := range a.zones {
		name := aws.StringValue(zone.Name)
		if strings.HasSuffix(hostname, name) {
			if name == hostname || strings.HasSuffix(hostname, "."+name) {
				if len(match) < len(name) {
					match = name
					found = zoneid
				}
			}
		}
	}
	return found, len(match) - 1
}

func (a *AWSProvider) submitChanges() error {
	if len(a.changes) == 0 {
		return nil
	}

	for zone, changes := range a.changes {
		limitedChanges := limitChangeSet(changes, maxChangeCount)
		for i, changes := range limitedChanges {
			a.Infof("processing batch %d for zone", i+1, zone)
			for _, c := range changes {
				a.Infof("desired change: %s %s %s", *c.Action, *c.ResourceRecordSet.Name, *c.ResourceRecordSet.Type)
			}

			params := &route53.ChangeResourceRecordSetsInput{
				HostedZoneId: aws.String(zone),
				ChangeBatch: &route53.ChangeBatch{
					Changes: mapChanges(changes),
				},
			}

			if _, err := a.r53.ChangeResourceRecordSets(params); err != nil {
				a.Error(err)
				for _, c := range changes {
					if c.Done != nil {
						c.Done.Failed(err)
					}
				}
				continue
			} else {
				for _, c := range changes {
					if c.Done != nil {
						c.Done.Succeeded()
					}
				}
				a.Infof("%d records in zone %s were successfully updated", len(changes), zone)
			}
		}
	}
	return nil
}

func limitChangeSet(changesByName map[string][]*Change, max int) [][]*Change {
	batches := [][]*Change{}

	// add deleteion requests
	batch := make([]*Change, 0)
	for _, changes := range changesByName {
		for _, change := range changes {
			if aws.StringValue(change.Change.Action) == route53.ChangeActionDelete {
				batch = addLimited(change, batch, batches, max)
			}
		}
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
		batch = make([]*Change, 0)
	}

	// add non-deletion requests

	for _, changes := range changesByName {
		for _, change := range changes {
			if aws.StringValue(change.Change.Action) != route53.ChangeActionDelete {
				batch = addLimited(change, batch, batches, max)
			}
		}
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}

	return batches
}

func addLimited(change *Change, batch []*Change, batches [][]*Change, max int) []*Change {
	if len(batch) >= max {
		batches = append(batches, batch)
		batch = make([]*Change, 0)
	}
	return append(batch, change)
}

func mapChanges(changes []*Change) []*route53.Change {
	dest := []*route53.Change{}
	for _, c := range changes {
		dest = append(dest, c.Change)
	}
	return dest
}

func (a *AWSProvider) AddAllRecords(zoneid string, dnssets map[string]*DNSSet) error {
	info := &AWSProviderInfo{zoneid}
	inp := (&route53.ListResourceRecordSetsInput{}).SetHostedZoneId(zoneid)

	aggr := func(resp *route53.ListResourceRecordSetsOutput, lastPage bool) (shouldContinue bool) {
		for _, r := range resp.ResourceRecordSets {
			//logrus.Infof("got %s %s", aws.StringValue(r.Type), aws.StringValue(r.Name))
			rtype := aws.StringValue(r.Type)
			if !supportedRecordType(rtype) {
				continue
			}

			name := aws.StringValue(r.Name)
			if strings.HasSuffix(name, ".") {
				name = name[:len(name)-1]
			}

			tmp := NewDNSSet(name, info, nil)
			rs := NewRecordSet(rtype, aws.Int64Value(r.TTL), nil)
			tmp.Sets[rtype] = rs
			for _, rr := range r.ResourceRecords {
				rs.Add(&Record{Value: aws.StringValue(rr.Value)})
			}

			name = MapFromProvider(rtype, tmp)

			dnsset := dnssets[name]
			if dnsset == nil {
				tmp.Name = name
				dnssets[name] = tmp
			} else {
				dnsset.Sets[rtype] = rs
			}
		}
		return true
	}

	if err := a.r53.ListResourceRecordSetsPages(inp, aggr); err != nil {
		return err
	}
	return nil
}

func supportedRecordType(t string) bool {
	switch t {
	case "CNAME", "A", "TXT":
		return true
	}
	return false
}
