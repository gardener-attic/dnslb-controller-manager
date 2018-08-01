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

package config

import (
	"context"

	"github.com/sirupsen/logrus"
)

type ConfigPath struct {
	Path string
}

type CLIConfig struct {
	Kubeconfig  string
	Configs     map[string]*ConfigPath
	Ident       string
	TTL         int64
	Providers   string
	Watches     string
	DryRun      bool
	Once        bool
	Duration    int
	Controllers string
	Cluster     string
	PluginDir   string

	Interval int // DNS check interval
	Port     int // server port for http endpoints

	LevelString string

	LogLevel             logrus.Level
	EffectiveControllers []string
}

func NewCLIConfig(ctx context.Context) (context.Context, *CLIConfig) {
	cli := &CLIConfig{}

	cli.Configs = map[string]*ConfigPath{}
	cli.Ident = "GardenRing"
	cli.TTL = 60
	ctx = context.WithValue(ctx, "Config", cli)
	return ctx, cli
}

func (this *CLIConfig) GetConfig(name string) *ConfigPath {
	return this.Configs[name]
}

func (this *CLIConfig) AddConfig(name string) (*ConfigPath, bool) {
	c := this.Configs[name]
	if c == nil {
		c = &ConfigPath{}
		this.Configs[name] = c
		return c, true
	}
	return c, false
}

func (this *CLIConfig) HasConfigs() bool {
	for _, c := range this.Configs {
		if c.Path != "" {
			return true
		}
	}
	return false
}

func Get(ctx context.Context) *CLIConfig {
	return ctx.Value("Config").(*CLIConfig)
}
