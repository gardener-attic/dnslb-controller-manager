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

	informers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions"
	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/dns"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/dns/model"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/endpoint"
	"github.com/gardener/dnslb-controller-manager/pkg/server"
	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"

	kubeinformers "k8s.io/client-go/informers"
	restclient "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ControllerManager struct {
	cli_config *config.CLIConfig
	ctx        context.Context

	clientset *controller.Clientset
	targetset *controller.Clientset

	sourcecontrollers []string
	targetcontrollers []string
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
	}
	sourcecontrollers := cli_config.GetSourceControllers()
	targetcontrollers := cli_config.GetTargetControllers()

	if len(sourcecontrollers)+len(targetcontrollers) == 0 {
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
	clientset, err := controller.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	target := kubeconfig
	targetset := clientset
	if cli_config.TargetKube != "" {
		logrus.Infof("separate config for target cluster: %s", cli_config.TargetKube)
		target, err = clientcmd.BuildConfigFromFlags("", cli_config.TargetKube)
		if err != nil {
			return nil, err
		}
		targetset, err = controller.NewForConfig(target)
		if err != nil {
			return nil, err
		}
	}

	err = config.RegisterCrds(targetset)
	if err != nil {
		return nil, err
	}

	if cli_config.Duration > 0 {
		ctx, _ = context.WithTimeout(ctx, time.Duration(cli_config.Duration)*time.Second)
	}

	return &ControllerManager{
		cli_config:        cli_config,
		ctx:               config.CancelContext(ctx),
		clientset:         clientset,
		sourcecontrollers: sourcecontrollers,
		targetset:         targetset,
		targetcontrollers: targetcontrollers,
	}, nil
}

func (this *ControllerManager) Run() {

	if this.cli_config.PluginDir != "" {
		controller.LoadPlugins(this.cli_config.PluginDir)
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(this.clientset, 30*time.Second)
	lbInformerFactory := informers.NewSharedInformerFactory(this.targetset, 30*time.Second)

	ctx := context.WithValue(this.ctx, "kubeInformerFactory", kubeInformerFactory)
	ctx = context.WithValue(ctx, "lbInformerFactory", lbInformerFactory)
	ctx = context.WithValue(ctx, "clientset", this.clientset)
	ctx = context.WithValue(ctx, "virtualset", this.targetset)
	if this.targetset != this.clientset {
		if len(this.targetcontrollers) > 0 {
			config.StartWithLease("target kube", this.targetset, ctx, this.startOnTargetKube)
		}
		if len(this.sourcecontrollers) > 0 {
			config.StartWithLease("source kube", this.clientset, ctx, this.startOnSourceKube)
		}
	} else {
		config.StartWithLease("kube", this.clientset, this.ctx, this.startOnBoth)
	}

	healthz.SetTimeout(time.Duration(this.cli_config.Interval*2+120) * time.Second)
	if this.cli_config.Port > 0 {
		server.Serve(this.ctx, "", this.cli_config.Port)
	}
	<-this.ctx.Done()
	logrus.Infof("controller manager stopped")
}

func (this *ControllerManager) startOnTargetKube(clientset *controller.Clientset, ctx context.Context) {
	logrus.Infof("starting controllers for target cluster...")
	switch {
	case ContainsString(this.targetcontrollers, "dns"):
		this.StartController(clientset, dns.Run, ctx)
	}
}
func (this *ControllerManager) startOnSourceKube(clientset *controller.Clientset, ctx context.Context) {
	logrus.Infof("starting controllers for source cluster...")
	switch {
	case ContainsString(this.sourcecontrollers, "endpoint"):
		this.StartController(clientset, endpoint.Run, ctx)
	}
}
func (this *ControllerManager) startOnBoth(clientset *controller.Clientset, ctx context.Context) {
	this.startOnTargetKube(clientset, ctx)
	this.startOnSourceKube(clientset, ctx)
}

func (this *ControllerManager) StartController(
	clientset *controller.Clientset,
	controller func(*controller.Clientset, context.Context) error,
	ctx context.Context) {
	if controller == nil {
		return
	}
	config.SyncPointAdd(ctx)
	go func() {
		err := controller(clientset, ctx)
		if err != nil {
			logrus.Error(err)
		} else {
			logrus.Infof("controller stopped")
		}
		config.Cancel(ctx)
		config.SyncPointDone(ctx)
	}()
}
