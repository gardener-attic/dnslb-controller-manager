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

package dns

import (
	"context"
	"fmt"
	"sync"
	"time"

	// builtin provider type initialization
	_ "github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/provider/aws"

	lbscheme "github.com/gardener/dnslb-controller-manager/pkg/client/clientset/versioned/scheme"
	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/groups"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/model"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/provider"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/source"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/source/configfile"

	"github.com/gardener/dnslb-controller-manager/pkg/log"
	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"
	"github.com/gardener/dnslb-controller-manager/pkg/server/metrics"
	"github.com/gardener/dnslb-controller-manager/pkg/tools/workqueue"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbinformers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions"
	lbv1beta1informers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions/loadbalancer/v1beta1"
	lblisters "github.com/gardener/dnslb-controller-manager/pkg/client/listers/loadbalancer/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

const controllerAgentName = "dns-loadbalancer-controller"
const threadiness = 2

func init() {
	groups.GetType("target").AddController("dns", Run)
}

type Controller struct {
	source.Access

	sources *source.Sources

	lock       sync.Mutex
	clientset  clientset.Interface
	cli_config *config.CLIConfig

	prInformer lbv1beta1informers.DNSProviderInformer
	prSynced   cache.InformerSynced
	prLister   lblisters.DNSProviderLister

	workqueue workqueue.RateLimitingInterface
	started   time.Time
}

type Access struct {
	log.LogCtx
	controller.EventRecorder
}

func NewController(clientset clientset.Interface, ctx context.Context) (*Controller, error) {
	var err error
	var sources *source.Sources

	cli_config := config.Get(ctx)
	logctx := log.NewLogContext("controller", "dns")

	lbInformerFactory := ctx.Value("targetInformerFactory").(lbinformers.SharedInformerFactory)

	if lbInformerFactory == nil {
		return nil, fmt.Errorf("targetInformerFactory not set in context")
	}
	prInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSProviders()
	recorder := controller.NewEventRecorder(logctx, controllerAgentName, clientset, lbscheme.AddToScheme)

	access := &Access{logctx, recorder}

	if cli_config.Watches == "" {
		logctx.Infof("using source plugins for determining DNS load balancers")
		sources, err = source.CreateSources(access, clientset, cli_config, ctx)
		if err != nil {
			return nil, err
		}
	} else {
		logctx.Infof("using load balancer config from config file '%s'", cli_config.Watches)
		sources = configfile.CreateSources(access, cli_config)
	}

	controller := &Controller{
		Access: access,

		sources:    sources,
		clientset:  clientset,
		cli_config: cli_config,

		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DNSProviders"),

		prSynced:   prInformer.Informer().HasSynced,
		prLister:   prInformer.Lister(),
		prInformer: prInformer,
	}

	lbInformerFactory.Start(ctx.Done())
	return controller, nil
}

func (this *Controller) UpdateProvider(pr *lbapi.DNSProvider) (*lbapi.DNSProvider, error) {
	return this.clientset.LoadbalancerV1beta1().DNSProviders(pr.GetNamespace()).Update(pr)
}

func (this *Controller) UpdateSecret(pr *corev1.Secret) (*corev1.Secret, error) {
	return this.clientset.CoreV1().Secrets(pr.GetNamespace()).Update(pr)
}

func (this *Controller) GetProvider(namespace, name string) (*lbapi.DNSProvider, error) {
	return this.prLister.DNSProviders(namespace).Get(name)
}

func (this *Controller) GetSecret(ref *corev1.SecretReference) (*corev1.Secret, error) {
	return this.clientset.CoreV1().Secrets(ref.Namespace).Get(ref.Name, metav1.GetOptions{})
}

func (this *Controller) GetWatches() *WatchConfig {

	watches, err := this.sources.Get()
	if err != nil {
		return nil
	}
	return &WatchConfig{Watches: watches}
}

func (this *Controller) registerDefaultProvider(reg *provider.TypeRegistration) error {
	name := "default-" + reg.GetName()
	typectx := this.NewLogContext("type", reg.GetName())
	logctx := typectx.NewLogContext("provider", name)
	p, err := reg.NewDefaultProvider(this.cli_config, logctx)
	if err != nil {
		this.Errorf("no default provider for provider type %s: %s", reg.GetName(), err)
	} else {
		this.Infof("registering default provider for aws")
		provider.RegisterProvider(name, p, nil)
	}
	return nil
}

func (this *Controller) Run(stopCh <-chan struct{}) error {
	providers := StringSet{}
	providers.AddAll(this.cli_config.Providers)

	if providers.Contains("all") || providers.Contains("static") {
		this.Infof("registering static DNS providers...")
		// register default DNS providers
		provider.ForTypeRegistrations(this.registerDefaultProvider)
	} else {
		// register selected default DNS providers
		for p := range providers {
			switch p {
			case "static", "dynamic", "all":
			default:
				reg := provider.GetTypeRegistration(p)
				if reg != nil {
					this.Infof("registering static '%s' provider...", p)
					this.registerDefaultProvider(reg)
				} else {
					this.Errorf("ignoring invalid provider type '%s'", p)
				}
			}
		}
	}

	if ok := cache.WaitForCacheSync(stopCh, this.prSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	this.sources.Setup(stopCh)

	if providers.Contains("all") || providers.Contains("dynamic") {
		this.Infof("registering dynamic DNS providers...")
		sleep := time.Duration(5)
		for sleep != 0 {
			prlist, err := this.prLister.DNSProviders("").List(labels.Everything())
			if err == nil {
				sleep = 0
				this.runFor(prlist)
			} else {
				this.Errorf("cannot get DNS provider list: %s", err)
				if sleep < 5*time.Minute {
					sleep += 10 * time.Second
				}
				time.Sleep(sleep)
			}
		}
	}

	this.Infof("running dns controller")

	this.Infof("Starting workers")
	for i := 0; i < threadiness; i++ {
		this.startWorker(i, stopCh)
	}

	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    this.handleAddObject,
		UpdateFunc: this.handleUpdateObject,
		DeleteFunc: this.handleDeleteObject,
	}
	this.prInformer.Informer().AddEventHandler(handlers)
	this.started = time.Now()

	return this.runDNSUpdater(stopCh)
}

func (this *Controller) startWorker(no int, stopCh <-chan struct{}) {
	go wait.Until(func() { NewWorker(this, no).Run() }, time.Second, stopCh)
}

func (this *Controller) runDNSUpdater(stopCh <-chan struct{}) error {
	model := NewModel(this.cli_config, this.Access, this.clientset, this.Access)
	initial := make(chan time.Time, 1)
	initial <- time.Now()

	var timeout <-chan time.Time = initial

	for {
		select {
		case _, ok := <-stopCh:
			if !ok {
				this.Infof("Terminating DNS Controller")
				return nil
			}
		case _, ok := <-timeout:
			if ok {
				metrics.ReportStartDNSReconcile()
				this.UpdateDNS(model)
				if this.cli_config.Once {
					return nil
				}
				d := this.cli_config.Interval - metrics.ReportDoneDNSReconcile()
				if d <= 0 {
					d = 1
				}
				healthz.Tick("dns")
				timeout = time.After(time.Duration(d) * time.Second)
			} else {
				return nil
			}
		}
	}
}

func (this *Controller) UpdateDNS(model *Model) {
	config := this.GetWatches()
	if config == nil {
		this.Infof("cannot get watches: skip")
		return
	}
	this.lock.Lock()
	defer this.lock.Unlock()
	model.Reset()
	for _, watch := range config.Watches {
		watch.Handle(model)
	}
	err := model.Update()
	if err != nil {
		this.Errorf("%s", err)
	}
}

/////////////////////////////////////////////////////////////////////////////////
// Controller main function

func Run(clientset clientset.Interface, ctx context.Context) error {

	c, err := NewController(clientset, ctx)
	if err != nil {
		return err
	}
	return c.Run(ctx.Done())
}
