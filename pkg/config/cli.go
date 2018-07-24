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
	"fmt"
	"strconv"
	"strings"

	. "github.com/gardener/dnslb-controller-manager/pkg/utils"
	"github.com/sirupsen/logrus"
)

var SourceControllers = []string{
	"endpoint",
}

var TargetControllers = []string{
	"dns",
}

type CLIConfig struct {
	Kubeconfig  string
	TargetKube  string
	Ident       string
	TTL         int64
	Providers   string
	Watches     string
	DryRun      bool
	Once        bool
	Duration    int
	Controllers string
	Cluster     string

	Interval int // DNS check interval
	Port     int // server port for http endpoints

	LevelString string

	LogLevel logrus.Level
}

func NewCLIConfig(ctx context.Context) (context.Context, *CLIConfig) {
	cli := &CLIConfig{}

	cli.Ident = "GardenRing"
	cli.TTL = 60
	ctx = context.WithValue(ctx, "Config", cli)
	return ctx, cli
}

func Get(ctx context.Context) *CLIConfig {
	return ctx.Value("Config").(*CLIConfig)
}

func (this *CLIConfig) Validate() error {
	if this.TargetKube == "" && this.Cluster != "" {
		return fmt.Errorf("cluster identity not possible when not using a separate target cluster")
	}
	if this.TargetKube != "" && this.Cluster == "" {
		return fmt.Errorf("cluster identity (for local cluster) required when using a separate target cluster")
	}
	controllers := this.GetControllers()
	for _, c := range controllers {
		if !(ContainsString(SourceControllers, c) || ContainsString(TargetControllers, c)) {
			return fmt.Errorf("unknown controller '%s'", c)
		}
	}

	l, err := strconv.Atoi(this.LevelString)
	if err != nil {
		this.LogLevel, err = logrus.ParseLevel(this.LevelString)
		if err != nil {
			return err
		}
	} else {
		if l < 0 || l > 5 {
			return fmt.Errorf("log level must be in the range 0-5")
		}
		this.LogLevel = logrus.Level(l)
	}
	return nil
}

func (this *CLIConfig) GetControllers() []string {
	switch this.Controllers {
	case "all":
		return append(SourceControllers, TargetControllers...)
	case "source":
		return SourceControllers
	case "target":
		return TargetControllers
	default:
		return strings.Split(this.Controllers, ",")
	}
}

func (this *CLIConfig) GetSourceControllers() []string {
	return this.filterControllers(SourceControllers)
}
func (this *CLIConfig) GetTargetControllers() []string {
	return this.filterControllers(TargetControllers)
}

func (this *CLIConfig) filterControllers(filter []string) []string {
	list := []string{}
	for _, c := range this.GetControllers() {
		if ContainsString(filter, c) {
			list = append(list, c)
		}
	}
	return list
}
