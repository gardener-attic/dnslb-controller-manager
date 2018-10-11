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

package model

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	"github.com/gardener/dnslb-controller-manager/pkg/log"

	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/provider"
)

type DNSSets map[string]*DNSSet

type Model struct {
	log.LogCtx
	lock  sync.Mutex
	ttl   int64
	ident string

	applied  map[string]*DNSSet
	sets     map[string]DNSSets
	requests map[string][]*ChangeRequest

	recorder  controller.EventRecorder
	clientset clientset.Interface

	providers map[string]*Registration

	ForRegistrations RegistrationIterator
}

func NewModel(cfg *config.CLIConfig, recorder controller.EventRecorder, clientset clientset.Interface,
	logctx log.LogCtx) *Model {
	m := &Model{}
	m.LogCtx = logctx
	m.ttl = cfg.TTL
	m.ident = cfg.Ident
	m.recorder = recorder
	m.clientset = clientset
	m.ForRegistrations = ForRegistrations
	return m
}

func (this *Model) Reset() {
	this.Lock()
	defer this.Unlock()
	this.requests = map[string][]*ChangeRequest{}
	this.sets = map[string]DNSSets{}
	this.applied = map[string]*DNSSet{}
	this.providers = map[string]*Registration{}

	this.ForRegistrations(func(reg *Registration) error {
		this.providers[reg.GetName()] = reg
		return nil
	})
}
func (this *Model) Lock() {
	this.lock.Lock()
}
func (this *Model) Unlock() {
	this.lock.Unlock()
}

func (this *Model) Check(name string, obj metav1.Object, done DoneHandler, targets ...Target) (bool, error) {
	return this.Exec(false, name, obj, done, targets...)
}
func (this *Model) Apply(name string, obj metav1.Object, done DoneHandler, targets ...Target) (bool, error) {
	return this.Exec(true, name, obj, done, targets...)
}
func (this *Model) Exec(apply bool, name string, obj metav1.Object, done DoneHandler, targets ...Target) (bool, error) {
	if len(targets) == 0 {
		return false, nil
	}

	this.Lock()
	defer this.Unlock()

	if apply {
		this.applied[name] = nil
	}
	info, reg := this.lookupProvider(name)
	if info == nil {
		done.SetInvalid()
		return false, fmt.Errorf("no provider found for '%s'", name)
	}

	if obj != nil && !reg.ValidFor(obj) {
		if apply {
			delete(this.applied, name)
		}
		done.SetInvalid()
		return false, fmt.Errorf("provider '%s' not valid for namespace '%s'", reg.GetName(), obj.GetNamespace())
	}

	sets, err := this.setupProvider(reg)
	if err != nil {
		return false, fmt.Errorf("Cannot get DNS records for '%s': %s", name, err)
	}

	dnsset := sets[name]
	this.Debugf("applying %d targets for %s", len(targets), name)
	newset := this.NewDNSSetForTargets(name, dnsset, this.ttl, info, done, targets...)
	mod := false
	if dnsset != nil {
		for ty, rset := range newset.Sets {
			currset := dnsset.Sets[ty]
			if currset == nil {
				if apply {
					this.addCreateRequest(reg, newset, ty)
				}
				mod = true
			} else {
				if MapToProvider(ty, dnsset) == MapToProvider(ty, newset) {
					if !match(currset, rset) {
						if apply {
							this.addUpdateRequest(reg, newset, ty)
						}
						mod = true
					} else {
						if apply {
							this.Debugf("records type %s up to date for %s", ty, name)
						}
					}
				} else {
					mod = true
					if apply {
						this.addCreateRequest(reg, newset, ty)
						this.addDeleteRequest(reg, dnsset, ty)
					}
				}
			}
		}
		for ty := range dnsset.Sets {
			if ty != "TXT" {
				if _, ok := newset.Sets[ty]; !ok {
					if apply {
						this.addDeleteRequest(reg, dnsset, ty)
					}
					mod = true
				}
			}
		}
	} else {
		if apply {
			for ty := range newset.Sets {
				this.addCreateRequest(reg, newset, ty)
			}
		}
		mod = true
	}
	if apply {
		this.applied[name] = newset
	}
	return mod, nil
}

func (this *Model) Update() error {
	this.Lock()
	defer this.Unlock()

	err := this.ForRegistrations(func(reg *Registration) error {
		sets, err := this.setupProvider(reg)
		if err != nil {
			this.Errorf("Cannot get DNS records for provider '%s': %s", reg.GetName(), err)
			return err
		}
		for _, s := range sets {
			_, ok := this.applied[s.Name]
			if !ok {
				if this.Owns(s) {
					this.Infof("found unapplied managed set '%s'", s.Name)
					for ty := range s.Sets {
						this.addDeleteRequest(reg, s, ty)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	failed := false
	for n, reqs := range this.requests {
		this.Infof("update provider %s", n)
		err := this.getProvider(n).ExecuteRequests(reqs)
		if err != nil {
			this.Errorf("update failed for provider %s: %s", n, err)
			failed = true
		}
	}
	if failed {
		return fmt.Errorf("update failed for some provider(s)")
	}
	return nil
}

func (this *Model) getProvider(name string) DNSProvider {
	reg := this.providers[name]
	if reg != nil {
		return reg.GetProvider()
	}
	return nil
}

func (this *Model) lookupProvider(dns string) (ProviderInfo, *Registration) {
	var found ProviderInfo
	var reg *Registration
	match := -1
	for _, p := range this.providers {
		info, n := p.Match(dns)
		if info != nil {
			if match < n {
				found = info
				reg = p
			}
		}
	}
	return found, reg
}

func (this *Model) setupProvider(reg *Registration) (DNSSets, error) {
	var err error

	sets := this.sets[reg.GetName()]
	if sets == nil {
		sets, err = reg.GetDNSSets()
		if err != nil {
			return nil, err
		}
		this.sets[reg.GetName()] = sets
	}
	return sets, nil
}

func (this *Model) addCreateRequest(reg *Registration, dnsset *DNSSet, rtype string) {
	this.addChangeRequest(reg, R_CREATE, dnsset, rtype)
}
func (this *Model) addUpdateRequest(reg *Registration, dnsset *DNSSet, rtype string) {
	this.addChangeRequest(reg, R_UPDATE, dnsset, rtype)
}
func (this *Model) addDeleteRequest(reg *Registration, dnsset *DNSSet, rtype string) {
	this.addChangeRequest(reg, R_DELETE, dnsset, rtype)
}
func (this *Model) addChangeRequest(reg *Registration, action string, dnsset *DNSSet, rtype string) {
	r := NewChangeRequest(action, rtype, dnsset)
	if action == R_DELETE {
		this.requests[reg.GetName()] = append([]*ChangeRequest{r}, this.requests[reg.GetName()]...)

	} else {
		this.requests[reg.GetName()] = append(this.requests[reg.GetName()], r)
	}
}

/////////////////////////////////////////////////////////////////////////////////
// DNSSets

func (this *Model) Owns(set *DNSSet) bool {
	return set.IsOwnedBy(this.ident)
}

func (this *Model) NewDNSSetForTargets(name string, base *DNSSet, ttl int64, info ProviderInfo, done DoneHandler, targets ...Target) *DNSSet {
	set := NewDNSSet(name, info, done)
	if base != nil {
		txt := base.Sets["TXT"]
		if txt != nil {
			set.Sets["TXT"] = txt.Clone()
		}
	}

	if base == nil || this.Owns(base) {
		set.SetOwner(this.ident)
		set.SetAttr(ATTR_PREFIX, TxtPrefix)
	}

	targetsets := set.Sets
	cnames := []string{}
	for _, t := range targets {
		ty := t.GetRecordType()
		if ty == "CNAME" && len(targets) > 1 {
			cnames = append(cnames, t.GetHostName())
			addrs, err := net.LookupHost(t.GetHostName())
			if err == nil {
				for _, addr := range addrs {
					AddRecord(targetsets, "A", addr, ttl)
				}
			} else {
				this.Errorf("cannot lookup '%s': %s", t.GetHostName(), err)
			}
			this.Debugf("mapping target '%s' to A records: %s", t.GetHostName(), strings.Join(addrs, ","))
		} else {
			AddRecord(targetsets, ty, t.GetHostName(), ttl)
		}
	}
	set.Sets = targetsets
	if len(cnames) > 0 && this.Owns(set) {
		sort.Strings(cnames)
		set.SetAttr(ATTR_CNAMES, strings.Join(cnames, ","))
	}
	return set
}

func AddRecord(targetsets RecordSets, ty string, host string, ttl int64) {
	rs := targetsets[ty]
	if rs == nil {
		rs = NewRecordSet(ty, ttl, nil)
		targetsets[ty] = rs
	}
	rs.Records = append(rs.Records, &Record{host})
}

/////////////////////////////////////////////////////////////////////////////////
// Utilities

func match(new, old *RecordSet) bool {
	if len(new.Records) != len(old.Records) {
		return false
	}
	for _, r := range new.Records {
		found := false
		for _, t := range old.Records {
			if t.Value == r.Value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
