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
	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"
)

////////////////////////////////////////////////////////////////////////////////
// DNS Provider Types
////////////////////////////////////////////////////////////////////////////////

type DNSProviderType interface {
	NewDefaultProvider(cli_cfg *config.CLIConfig, logctx log.LogCtx) (DNSProvider, error)

	NewProvider(name string,
		cli_cfg *config.CLIConfig,
		cfg Properties, logctx log.LogCtx) (DNSProvider, error)
}

type TypeRegistration struct {
	name string
	DNSProviderType
}

func (this *TypeRegistration) GetName() string {
	return this.name
}
func (this *TypeRegistration) GetProviderType() DNSProviderType {
	return this.DNSProviderType
}

func (this *TypeRegistration) NewDefaultProvider(cli_cfg *config.CLIConfig,
	logctx log.LogCtx) (DNSProvider, error) {

	return this.DNSProviderType.NewDefaultProvider(cli_cfg, logctx)
}

func (this *TypeRegistration) NewProvider(name string,
	cli_cfg *config.CLIConfig,
	cfg Properties,
	logctx log.LogCtx) (DNSProvider, error) {

	return this.DNSProviderType.NewProvider(name, cli_cfg, cfg, logctx)
}

var _ DNSProviderType = &TypeRegistration{}

var (
	types = map[string]*TypeRegistration{}
)

func RegisterProviderType(name string, provider_type DNSProviderType) {
	lock.Lock()
	defer lock.Unlock()

	types[name] = &TypeRegistration{name, provider_type}
}

func ForTypeRegistrations(f func(*TypeRegistration) error) error {
	for _, r := range types {
		err := f(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetTypeRegistration(name string) *TypeRegistration {
	lock.Lock()
	defer lock.Unlock()
	return types[name]
}

func GetProviderType(name string) DNSProviderType {
	lock.Lock()
	defer lock.Unlock()
	return types[name].DNSProviderType
}
