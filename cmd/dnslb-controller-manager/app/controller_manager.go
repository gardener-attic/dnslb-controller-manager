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
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/groups"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/model"
	"github.com/gardener/dnslb-controller-manager/pkg/server"
	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"

	restclient "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ControllerManager struct {
	cli_config *config.CLIConfig
	ctx        context.Context
	grps       *groups.Groups

	//sourcecontrollers *groups.Group
	//targetcontrollers *groups.Group
}

func NewControllerManager(ctx context.Context) (*ControllerManager, error) {
	cli_config := config.Get(ctx)

	logrus.Infof("CONFIG: %#v", cli_config)
	if cli_config.Watches != "" {
		cfg, err := model.ReadConfig(cli_config.Watches)
		if err != nil {
			return nil, fmt.Errorf("cannot read watch config '%s':%s", cli_config.Watches, err)
		}
		for _, w := range cfg.Watches {
			logrus.Infof("watch %s", w.DNS)
		}
	} else {
		logrus.Infof("no watches specified on command line")
	}

	grps := groups.Activate(cli_config.EffectiveControllers)
	if !grps.HasActive() {
		return nil, fmt.Errorf("no controller selected")
	}
	logrus.Info("setting up controller manager...")

	// use the current context in kubeconfig
	if cli_config.Kubeconfig == "" {
		cli_config.Kubeconfig = os.Getenv("KUBECONFIG")
	}

	var kubeconfig *restclient.Config
	var err error
	if cli_config.Kubeconfig == "" {
		logrus.Infof("no config -> using in cluster config")
		kubeconfig, err = restclient.InClusterConfig()
	} else {
		logrus.Infof("using explicit config '%s'", cli_config.Kubeconfig)
		var config *clientcmdapi.Config
		config, err = clientcmd.LoadFromFile(cli_config.Kubeconfig)
		if err == nil {
			kubeconfig, err = clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
		}
	}
	if err != nil {
		logrus.Infof("cannot setup kube rest client: %s", err)
		return nil, err
	}

	// create the clientset
	logrus.Infof("creating clientset")
	defaultset, err := clientset.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	for n := range groups.GetTypes() {
		g := grps.GetGroup(n)
		cfg := cli_config.GetConfig(n)
		if cfg.Path != "" {
			logrus.Infof("separate config for %s cluster: %s", n, cfg.Path)
			target, err := clientcmd.BuildConfigFromFlags("", cfg.Path)
			if err != nil {
				return nil, err
			}
			targetset, err := clientset.NewForConfig(target)
			if err != nil {
				return nil, err
			}
			g.SetClientset(targetset)
		} else {
			g.SetClientset(defaultset)
		}
	}
	tgrp := grps.GetGroup("target")
	if tgrp != nil {
		targetset := tgrp.GetClientset()

		err = config.RegisterCrds(targetset)
		if err != nil {
			return nil, err
		}
	}

	if cli_config.Duration > 0 {
		ctx, _ = context.WithTimeout(ctx, time.Duration(cli_config.Duration)*time.Second)
	}
	ctx = config.CancelContext(ctx)

	_, err = grps.Setup(ctx)
	if err != nil {
		logrus.Errorf("controller manager setup failed: %s", err)
		return nil, err
	}

	return &ControllerManager{
		cli_config: cli_config,
		ctx:        ctx,
		grps:       grps,
	}, nil
}

func (this *ControllerManager) Run() {

	if this.cli_config.PluginDir != "" {
		controller.LoadPlugins(this.cli_config.PluginDir)
	}

	this.grps.Start()

	healthz.SetTimeout(time.Duration(this.cli_config.Interval*2+120) * time.Second)
	if this.cli_config.Port > 0 {
		server.Serve(this.ctx, "", this.cli_config.Port)
	}

	<-this.ctx.Done()
	logrus.Infof("controller manager stopped")
}
