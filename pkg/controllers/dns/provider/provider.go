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

package provider

import (
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1/scope"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"
	//	"github.com/sirupsen/logrus"
)

////////////////////////////////////////////////////////////////////////////////
// DNS Providers
////////////////////////////////////////////////////////////////////////////////

type ProviderInfo interface{}

type DNSProvider interface {
	GetType() DNSProviderType
	GetConfig() Properties
	GetDomains() StringSet
	Match(dns string) (ProviderInfo, int)
	GetDNSSets() (map[string]*DNSSet, error)
	ExecuteRequests(reqs []*ChangeRequest) error
}

type Registration struct {
	name string
	DNSProvider
	access scope.AccessControl
}

func (this *Registration) GetName() string {
	return this.name
}
func (this *Registration) GetProvider() DNSProvider {
	return this.DNSProvider
}
func (this *Registration) SetAccessControl(access scope.AccessControl) {
	this.access = access
}

func (this *Registration) ValidFor(obj metav1.Object) bool {
	return this.access == nil || this.access.ValidFor(obj)
}

var _ DNSProvider = &Registration{}

var (
	lock      sync.Mutex
	providers = map[string]*Registration{}
)

func NewRegistration(name string, provider DNSProvider) *Registration {
	return &Registration{name, provider, nil}
}

func RegisterProvider(name string, provider DNSProvider, access scope.AccessControl) *Registration {
	lock.Lock()
	defer lock.Unlock()

	if provider == nil {
		return nil
	}
	reg := &Registration{name, provider, nil}
	reg.SetAccessControl(access)
	providers[name] = reg
	return reg
}
func UnregisterProvider(name string) *Registration {
	lock.Lock()
	defer lock.Unlock()

	old := providers[name]
	if old != nil {
		delete(providers, name)
	}
	return old
}

type RegistrationIterator func(func(*Registration) error) error

func ForRegistrations(f func(*Registration) error) error {
	for _, r := range providers {
		err := f(r)
		if err != nil {
			return err
		}
	}
	return nil
}

var _ RegistrationIterator = ForRegistrations

func GetRegistrations() map[string]*Registration {
	return providers
}

func GetRegistration(name string) *Registration {
	lock.Lock()
	defer lock.Unlock()
	return providers[name]
}

func GetProvider(name string) DNSProvider {
	lock.Lock()
	defer lock.Unlock()
	return providers[name].GetProvider()
}

////////////////////////////////////////////////////////////////////////////////
// Record Sets
////////////////////////////////////////////////////////////////////////////////

type Record struct {
	Value string
}

func (this *Record) Clone() *Record {
	return &Record{this.Value}
}

type RecordSet struct {
	Type    string
	TTL     int64
	Records []*Record
}

func NewRecordSet(rtype string, ttl int64, records []*Record) *RecordSet {
	if records == nil {
		records = []*Record{}
	}
	return &RecordSet{Type: rtype, TTL: ttl, Records: records}
}

func (this *RecordSet) Clone() *RecordSet {
	set := &RecordSet{this.Type, this.TTL, nil}
	for _, r := range this.Records {
		set.Records = append(set.Records, r.Clone())
	}
	return set
}

func (this *RecordSet) Add(records ...*Record) *RecordSet {
	for _, r := range records {
		this.Records = append(this.Records, r)
	}
	return this
}

func (this *RecordSet) RecordString() string {
	line := ""
	for _, r := range this.Records {
		line = fmt.Sprintf("%s %s", line, r.Value)
	}
	if line == "" {
		return "no records"
	}
	return line
}

type DoneHandler interface {
	SetInvalid()
	Failed(err error)
	Succeeded()
}

type RecordSets map[string]*RecordSet

type DNSSet struct {
	Name string
	Sets RecordSets
	Info ProviderInfo
	Done DoneHandler
}

const (
	ATTR_OWNER  = "owner"
	ATTR_PREFIX = "prefix"
	ATTR_CNAMES = "cnames"
)

func (this *DNSSet) GetAttr(name string) string {
	txt := this.Sets["TXT"]
	if txt != nil {
		prefix := fmt.Sprintf("\"%s=", name)
		for _, r := range txt.Records {
			if strings.HasPrefix(r.Value, prefix) {
				return r.Value[len(prefix) : len(r.Value)-1]
			}
		}
		if name == ATTR_OWNER {
			for _, r := range txt.Records {
				if !strings.Contains(r.Value, "=") {
					return r.Value[1 : len(r.Value)-1]
				}
			}
		}
	}
	return ""
}

func (this *DNSSet) SetAttr(name string, value string) {
	txt := this.Sets["TXT"]
	if txt == nil {
		records := []*Record{&Record{Value: fmt.Sprintf("\"%s=%s\"", name, value)}}
		this.Sets["TXT"] = &RecordSet{"TXT", 600, records}
	} else {
		prefix := fmt.Sprintf("\"%s=", name)
		for _, r := range txt.Records {
			if !strings.Contains(r.Value, "=") {
				if name == ATTR_OWNER {
					r.Value = fmt.Sprintf("\"%s=%s\"", ATTR_OWNER, value)
					return
				} else {
					r.Value = fmt.Sprintf("\"%s=%s\"", ATTR_OWNER, r.Value[1:len(r.Value)-1])
				}
			}
		}
		for _, r := range txt.Records {
			if strings.HasPrefix(r.Value, prefix) {
				//logrus.Infof("replace attr %s=$s", name, value)
				r.Value = fmt.Sprintf("\"%s=%s\"", name, value)
				return
			}
		}
		//logrus.Infof("add attr %s=$s", name, value)
		r := &Record{Value: fmt.Sprintf("\"%s=%s\"", name, value)}
		txt.Records = append(txt.Records, r)
	}
}

func (this *DNSSet) IsOwnedBy(ownerid string) bool {
	o := this.GetAttr(ATTR_OWNER)
	return o != "" && o == ownerid
}

func (this *DNSSet) SetOwner(ownerid string) *DNSSet {
	this.SetAttr(ATTR_OWNER, ownerid)
	return this
}

func (this *DNSSet) SetRecordSet(rtype string, ttl int64, values ...string) {
	records := make([]*Record, len(values))
	for i, r := range values {
		records[i] = &Record{Value: r}
	}
	this.Sets[rtype] = &RecordSet{rtype, ttl, records}
}

func NewDNSSet(name string, info ProviderInfo, done DoneHandler) *DNSSet {
	return &DNSSet{Name: name, Info: info, Sets: map[string]*RecordSet{}, Done: done}
}

////////////////////////////////////////////////////////////////////////////////
// Requests
////////////////////////////////////////////////////////////////////////////////

const (
	R_CREATE = "create"
	R_UPDATE = "update"
	R_DELETE = "delete"
)

type ChangeRequest struct {
	Action string
	Type   string
	DNS    *DNSSet
}

func NewChangeRequest(action string, rtype string, dns *DNSSet) *ChangeRequest {
	return &ChangeRequest{Action: action, Type: rtype, DNS: dns}
}

////////////////////////////////////////////////////////////////////////////////
// Text Record Name Mapping
////////////////////////////////////////////////////////////////////////////////

var TxtPrefix = "comment-"

func MapToProvider(rtype string, dnsset *DNSSet) string {
	name := dnsset.Name
	prefix := dnsset.GetAttr(ATTR_PREFIX)
	if rtype == "TXT" && prefix != "" {
		add := ""
		if strings.HasPrefix(name, "*.") {
			add = "*."
			name = name[2:]
		}
		return add + prefix + name
	}
	return name
}

func MapFromProvider(rtype string, dnsset *DNSSet) string {
	name := dnsset.Name
	if rtype == "TXT" {
		prefix := dnsset.GetAttr(ATTR_PREFIX)
		if prefix != "" {
			add := ""
			if strings.HasPrefix(name, "*.") {
				add = "*."
				name = name[2:]
			}
			if strings.HasPrefix(name, prefix) {
				return add + name[len(prefix):]
			} else {
				return add + name
			}
		}
	}
	return name
}
