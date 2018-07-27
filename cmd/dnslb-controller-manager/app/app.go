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

package app

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/groups"
)

// NewCommandStartDNSLBControllerManager creates a *cobra.Command object with default parameters.
func NewCommandStartDNSLBControllerManager(out, errOut io.Writer, ctx context.Context) *cobra.Command {

	ctx, cli := config.NewCLIConfig(ctx)

	cmd := &cobra.Command{
		Use:   "dnslb-controller-manager",
		Short: "Launch the DNS Loadbalancer Controller Manager",
		Long: `This manager manages DNS LB endpoint resources for DNS Loadbalancer
resources based on annotations in services and ingresses. Based on
those endpoints a second controller manages DNS entries. The endpoint
sources may reside in different kubernetes clusters than the one
hosting the DNS loadbalancer and endpoint resources.`,
		RunE: func(c *cobra.Command, args []string) error {
			return run(ctx)
		},
	}

	cmd.PersistentFlags().StringVarP(&cli.Kubeconfig, "kubeconfig", "", "", "path to the kubeconfig file")
	cmd.PersistentFlags().StringVarP(&cli.Watches, "watches", "", "", "config file for watches")
	cmd.PersistentFlags().StringVarP(&cli.Ident, "identity", "", "GardenRing", "DNS record identifer")
	cmd.PersistentFlags().StringVarP(&cli.Controllers, "controllers", "", "all", "Comma separated list of controllers to start (<name>,source,target,all)")
	cmd.PersistentFlags().StringVarP(&cli.Cluster, "cluster", "", "", "Cluster identity")
	cmd.PersistentFlags().Int64VarP(&cli.TTL, "ttl", "", 60, "DNS record ttl in seconds")
	cmd.PersistentFlags().StringVarP(&cli.Providers, "providers", "", "dynamic", "Selection mode for DNS providers (static,dynamic,all,<type name>)")
	cmd.PersistentFlags().IntVarP(&cli.Duration, "duration", "", 0, "Runtime before stop (in seconds)")
	cmd.PersistentFlags().BoolVarP(&cli.DryRun, "dry-run", "", false, "Dry run for DNS controller")
	cmd.PersistentFlags().BoolVarP(&cli.Once, "once", "", false, "only one update instread of loop")
	cmd.PersistentFlags().StringVarP(&cli.LevelString, "log-level", "D", "info", "log level")
	cmd.PersistentFlags().IntVarP(&cli.Interval, "interval", "", 30, "DNS check/update interval in seconds")
	cmd.PersistentFlags().IntVarP(&cli.Port, "port", "", 0, "http server endpoint port for health-check (default: 0=no server)")
	cmd.PersistentFlags().StringVarP(&cli.PluginDir, "plugin-dir", "", "", "directory containing go plugins for DNS provider types")

	groups.ConfigureCommand(cmd, cli)
	return cmd
}

func run(ctx context.Context) error {

	err := Validate(config.Get(ctx))
	if err != nil {
		return err
	}
	logrus.SetLevel(logrus.Level(config.Get(ctx).LogLevel))
	cm, err := NewControllerManager(ctx)
	if err != nil {
		return err
	}
	cm.Run()
	return nil
}

func Validate(this *config.CLIConfig) error {
	if !this.HasConfigs() && this.Cluster != "" {
		return fmt.Errorf("cluster identity not possible when not using a separate clusters")
	}
	if this.HasConfigs() && this.Cluster == "" {
		return fmt.Errorf("cluster identity (for local cluster) required when using a separate clusters")
	}
	var err error
	this.EffectiveControllers, err = groups.GetControllerNames(this.Controllers)
	logrus.Infof("selected controllers: %v", this.EffectiveControllers)
	if err != nil {
		return err
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
