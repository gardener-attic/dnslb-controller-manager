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

package configfile

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/model"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/source"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
)

func init() {
	source.Register(&SourceType{})
}

type SourceType struct {
}

func (this *SourceType) ConfigureCommand(cmd *cobra.Command, cli_config *config.CLIConfig) {
	cmd.PersistentFlags().StringVarP(&cli_config.Watches, "watches", "", "", "config file for watches")
}

func (this *SourceType) Create(acc source.Access, clientset clientset.Interface, cli_config *config.CLIConfig, ctx context.Context) (source.Source, error) {
	if cli_config.Watches == "" {
		return nil, nil
	}
	_, err := model.ReadConfig(cli_config.Watches)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate watch config '%s': 5s", cli_config.Watches, err)
	}
	return &Source{acc.NewLogContext("source", "file"), acc, cli_config.Watches}, nil
}

type Source struct {
	log.LogCtx
	access  source.Access
	watches string
}

var _ source.Source = &Source{}

func (this *Source) Setup(stopCh <-chan struct{}) error {
	return nil
}

func (this *Source) Get() ([]*model.Watch, error) {
	if this.watches == "" {
		return nil, nil
	}
	c, err := model.ReadConfig(this.watches)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate watch config '%s': 5s", this.watches, err)
	}
	return c.Watches, nil
}

func CreateSources(acc source.Access, cli_config *config.CLIConfig) *source.Sources {
	s := &Source{acc.NewLogContext("source", "file"), acc, cli_config.Watches}
	return source.NewSources(s)
}
