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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1/scope"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"
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

type DNSSet struct {
	Name string
	Sets map[string]*RecordSet
	Info ProviderInfo
	Done DoneHandler
}

func (this *DNSSet) IsOwnedBy(ownerid string) bool {
	txt := this.Sets["TXT"]
	ownerid = fmt.Sprintf("\"%s\"", ownerid)
	if txt != nil {
		for _, r := range txt.Records {
			if r.Value == ownerid {
				return true
			}
		}
	}
	return false
}

func (this *DNSSet) SetOwner(ownerid string) *DNSSet {
	this.SetRecordSet("TXT", 600, fmt.Sprintf("\"%s\"", ownerid))
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
