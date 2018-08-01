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

package groups

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	restclient "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gardener/dnslb-controller-manager/pkg/config"

	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"
)

type Controller func(clientset.Interface, context.Context) error
type Activator func(group *Group, ctx context.Context) (GroupData, context.Context, error)

type GroupType struct {
	name        string
	controllers map[string]Controller
	configopt   string
	activator   Activator
}

type GroupData interface {
}

type Group struct {
	gtype     *GroupType
	active    []string
	clientset clientset.Interface

	data GroupData
}

var types = map[string]*GroupType{}
var lock sync.Mutex

func ConfigureCommand(cmd *cobra.Command, cli_config *config.CLIConfig) {
	for n, t := range types {
		if t.configopt != "" {
			logrus.Infof("config option '%s' for '%s'", t.configopt, n)
			p, new := cli_config.AddConfig(t.configopt)
			if new {
				cmd.PersistentFlags().StringVarP(&p.Path, t.configopt, "", "", "path to the "+n+" kubeconfig file")
			}
		}
	}
}

func GetTypes() map[string]*GroupType {
	lock.Lock()
	defer lock.Unlock()

	result := map[string]*GroupType{}
	for n, t := range types {
		result[n] = t
	}
	return result
}

func GetType(name string) *GroupType {
	lock.Lock()
	defer lock.Unlock()

	t := types[name]
	if t == nil {
		t = &GroupType{name: name, controllers: map[string]Controller{}}
		types[name] = t
	}
	return t
}

func (this *GroupType) GetName() string {
	return this.name
}

func (this *GroupType) GetConfigOption() string {
	return this.configopt
}

func (this *GroupType) GetControllers() map[string]Controller {
	active := map[string]Controller{}
	for n, c := range this.controllers {
		active[n] = c
	}
	return active
}

func (this *GroupType) SetActivator(activator Activator) *GroupType {
	this.activator = activator
	return this
}

func (this *GroupType) SetConfigOption(opt string) *GroupType {
	this.configopt = opt
	return this
}

func (this *GroupType) AddController(name string, controller Controller) *GroupType {
	logrus.Infof("register controller %s/%s", this.name, name)
	this.controllers[name] = controller
	return this
}

func (this *GroupType) Activate(active []string) *Group {
	a := []string{}
	for _, n := range active {
		_, ok := this.controllers[n]
		if ok {
			a = append(a, n)
		}
	}
	return &Group{gtype: this, active: a}
}

/////////////////////////////////////////////////////////////////////////////////

type Groups struct {
	groups   map[string]*Group
	inactive map[string]*Group
	sets     []*StartupSet
	ctx      context.Context
}

func (this *Groups) SetupClientsets(cli_config *config.CLIConfig) error {
	var kubeconfig *restclient.Config
	var err error

	// use the current context in kubeconfig
	if cli_config.Kubeconfig == "" {
		cli_config.Kubeconfig = os.Getenv("KUBECONFIG")
	}

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
		return err
	}

	// create the clientset
	logrus.Infof("creating clientset")
	defaultset, err := clientset.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}

	clientsetByOption := map[string]clientset.Interface{}
	clientsetByPath := map[string]clientset.Interface{}
	clientsetByOption[""] = defaultset
	if cli_config.Kubeconfig != "" {
		clientsetByPath[cli_config.Kubeconfig] = defaultset
	}

	var cs clientset.Interface
	for n, t := range GetTypes() {
		cs = defaultset
		g := this.GetGroup(n)
		opt := t.GetConfigOption()
		if opt != "" {
			cfg := cli_config.GetConfig(opt)
			if cfg.Path != "" {
				logrus.Infof("separate config for %s cluster: %s", n, cfg.Path)
				cs = clientsetByOption[opt]
				if cs == nil {
					cs = clientsetByPath[cfg.Path]
					if cs == nil {
						target, err := clientcmd.BuildConfigFromFlags("", cfg.Path)
						if err != nil {
							return err
						}
						cs, err = clientset.NewForConfig(target)
						if err != nil {
							return err
						}
						clientsetByPath[cfg.Path] = cs
					}
					clientsetByOption[opt] = cs
				}
			}
		}
		g.SetClientset(cs)
	}
	return nil
}

func Activate(active []string) *Groups {
	lock.Lock()
	defer lock.Unlock()
	logrus.Infof("activating %+v", active)
	grps := &Groups{groups: map[string]*Group{}, inactive: map[string]*Group{}, sets: []*StartupSet{}}
	for n, t := range types {
		g := t.Activate(active)
		if g.IsActive() {
			logrus.Infof("group %s activated", n)
			grps.groups[n] = g
		} else {
			logrus.Infof("group %s inactive", n)
			grps.inactive[n] = g
		}
	}
	return grps
}

func (this *Groups) HasActive() bool {
	return len(this.groups) > 0
}

func (this *Groups) GetGroup(name string) *Group {
	g, ok := this.groups[name]
	if ok {
		return g
	}
	return this.inactive[name]
}

func (this *Groups) GetActive() map[string]*Group {
	grps := map[string]*Group{}
	for n, g := range this.groups {
		grps[n] = g
	}
	return grps
}

func (this *Groups) Setup(ctx context.Context) (context.Context, error) {
	var err error

	for _, g := range this.groups {
		ctx, err = g.Setup(ctx)
		if err != nil {
			return ctx, err
		}
		this.addGroup(g)
	}

	for _, g := range this.inactive {
		ctx, err = g.Setup(ctx)
		if err != nil {
			return ctx, err
		}
	}

	this.ctx = ctx
	return ctx, nil
}

func (this *Groups) Start() {
	if len(this.sets) == 1 {
		name := "kube"
		if len(this.sets[0].groups) == 1 {
			name = this.sets[0].groups[0].GetName()
		}
		this.sets[0].Start(name, this.ctx)
	} else {
		for _, s := range this.sets {
			s.Start(s.GetName(), this.ctx)
		}
	}
}

func (this *Groups) getSet(clientset clientset.Interface) *StartupSet {

	for _, s := range this.sets {
		if s.clientset == clientset {
			return s
		}
	}
	set := &StartupSet{
		clientset:   clientset,
		groups:      []*Group{},
		controllers: map[string]Controller{},
	}
	this.sets = append(this.sets, set)
	return set
}

func (this *Groups) addGroup(g *Group) {
	set := this.getSet(g.clientset)
	for _, f := range set.groups {
		if f == g {
			return
		}
	}
	set.groups = append(set.groups, g)
	for n, c := range g.GetControllers() {
		set.controllers[n] = c
	}
}

/////////////////////////////////////////////////////////////////////////////////

func (this *Group) GetName() string {
	return this.gtype.GetName()
}

func (this *Group) IsActive() bool {
	return len(this.active) > 0
}

func (this *Group) GetClientset() clientset.Interface {
	return this.clientset
}

func (this *Group) GetControllers() map[string]Controller {
	active := map[string]Controller{}
	for _, n := range this.active {
		active[n] = this.gtype.controllers[n]
	}
	return active
}

func (this *Group) SetClientset(clientset clientset.Interface) *Group {
	this.clientset = clientset
	return this
}

func (this *Group) Setup(ctx context.Context) (context.Context, error) {
	ctx = context.WithValue(ctx, this.GetName()+"Clientset", this.clientset)
	if this.gtype.activator != nil {
		d, ctx, err := this.gtype.activator(this, ctx)
		this.data = d
		return ctx, err
	}
	return ctx, nil
}

/////////////////////////////////////////////////////////////////////////////////

type StartupSet struct {
	clientset   clientset.Interface
	groups      []*Group
	controllers map[string]Controller
}

func (this *StartupSet) GetName() string {
	txt := ""
	for _, g := range this.groups {
		txt = fmt.Sprintf("%s %s", txt, g.GetName())
	}
	return txt
}

func (this *StartupSet) Start(txt string, ctx context.Context) {
	f := func(clientset clientset.Interface, ctx context.Context) {
		logrus.Infof("starting %s controllers", txt)
		for n, c := range this.controllers {
			logrus.Infof("starting controller %s", n)
			this.startController(c, ctx)
		}
	}
	config.StartWithLease(txt, this.clientset, ctx, f)
}

func (this *StartupSet) startController(controller Controller, ctx context.Context) {
	if controller == nil {
		return
	}
	config.SyncPointAdd(ctx)
	go func() {
		err := controller(this.clientset, ctx)
		if err != nil {
			logrus.Error(err)
		} else {
			logrus.Infof("controller stopped")
		}
		config.Cancel(ctx)
		config.SyncPointDone(ctx)
	}()
}

/////////////////////////////////////////////////////////////////////////////////

func GetControllerNames(controllers string) ([]string, error) {
	names := []string{}
	for _, c := range strings.Split(controllers, ",") {
		switch c {
		case "all":
			for _, t := range types {
				for n := range t.controllers {
					names, _ = AppendString(names, n)
				}
			}
		default:
			t := types[c]
			if t != nil {
				for n := range t.controllers {
					names, _ = AppendString(names, n)
				}
			} else {
				for _, t := range types {
					if t.controllers[c] != nil {
						names, _ = AppendString(names, c)
					} else {
						return nil, fmt.Errorf("unknown controller or group '%s'", c)
					}
				}
			}
		}
	}
	return names, nil
}
