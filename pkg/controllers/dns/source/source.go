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

package source

import (
	"context"
	"reflect"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/model"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
)

type Access interface {
	log.LogCtx
	controller.EventRecorder
}

type Source interface {
	Get() ([]*model.Watch, error)
	Setup(stopCh <-chan struct{}) error
}

type SourceType interface {
	ConfigureCommand(cmd *cobra.Command, cli_config *config.CLIConfig)

	Create(acc Access, clientset clientset.Interface, cli_config *config.CLIConfig, ctx context.Context) (Source, error)
}

var types = map[reflect.Type]SourceType{}

func Register(st SourceType) {
	logrus.Infof("register DNS source type %T", st)
	types[reflect.TypeOf(st)] = st
}

func ConfigureCommand(cmd *cobra.Command, cli_config *config.CLIConfig) {
	for _, t := range types {
		t.ConfigureCommand(cmd, cli_config)
	}
}

func GetTypes() []SourceType {
	src := []SourceType{}
	for _, s := range types {
		src = append(src, s)
	}
	return src
}

type Sources struct {
	srcs []Source
}

func NewSources(s ...Source) *Sources {
	return &Sources{s}
}

func CreateSources(acc Access, clientset clientset.Interface, cli_config *config.CLIConfig, ctx context.Context) (*Sources, error) {
	acc.Infof("selecting dns sources...")
	srcs := NewSources()
	for _, t := range GetTypes() {
		s, err := t.Create(acc, clientset, cli_config, ctx)
		if err != nil {
			return nil, err
		}
		if s != nil {
			acc.Infof("using dns source %T", s)
			srcs.srcs = append(srcs.srcs, s)
		}
	}
	return srcs, nil
}

func (this *Sources) Setup(stopCh <-chan struct{}) error {
	for _, s := range this.srcs {
		err := s.Setup(stopCh)
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *Sources) Get() ([]*model.Watch, error) {
	watches := []*model.Watch{}
	for _, s := range this.srcs {
		l, err := s.Get()
		if err != nil {
			return nil, err
		}
		for _, w := range l {
			watches = append(watches, w)
		}
	}
	return watches, nil
}
