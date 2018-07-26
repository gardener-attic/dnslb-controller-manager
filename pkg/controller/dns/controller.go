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
	"reflect"
	"sync"
	"time"

	// builtin provider type initialization
	_ "github.com/gardener/dnslb-controller-manager/pkg/controller/dns/provider/aws"

	lbscheme "github.com/gardener/dnslb-controller-manager/pkg/client/clientset/versioned/scheme"
	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	. "github.com/gardener/dnslb-controller-manager/pkg/controller/dns/model"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/dns/provider"
	. "github.com/gardener/dnslb-controller-manager/pkg/controller/dns/util"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"
	"github.com/gardener/dnslb-controller-manager/pkg/tools/workqueue"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbinformers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions"
	lbv1beta1informers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions/loadbalancer/v1beta1"
	lblisters "github.com/gardener/dnslb-controller-manager/pkg/client/listers/loadbalancer/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const controllerAgentName = "dns-loadbalancer-controller"
const threadiness = 2

type Controller struct {
	log.LogCtx
	watches    string
	lock       sync.Mutex
	clientset  *controller.Clientset
	cli_config *config.CLIConfig

	prInformer lbv1beta1informers.DNSProviderInformer
	prSynced   cache.InformerSynced
	prLister   lblisters.DNSProviderLister

	lbInformer lbv1beta1informers.DNSLoadBalancerInformer
	lbSynced   cache.InformerSynced
	lbLister   lblisters.DNSLoadBalancerLister

	epInformer lbv1beta1informers.DNSLoadBalancerEndpointInformer
	epSynced   cache.InformerSynced
	epLister   lblisters.DNSLoadBalancerEndpointLister

	recorder record.EventRecorder

	workqueue workqueue.RateLimitingInterface
	started   time.Time
}

func NewController(clientset *controller.Clientset, ctx context.Context) *Controller {
	cli_config := config.Get(ctx)
	logctx := log.NewLogContext("controller", "dns")

	if cli_config.Watches == "" {
		logctx.Infof("using in cluster scan for load balancer resources")
		lbInformerFactory := ctx.Value("lbInformerFactory").(lbinformers.SharedInformerFactory)
		lbInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSLoadBalancers()
		epInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSLoadBalancerEndpoints()
		prInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSProviders()

		recorder := controller.NewEventRecorder(logctx, controllerAgentName, clientset, lbscheme.AddToScheme)

		controller := &Controller{
			LogCtx:     logctx,
			clientset:  clientset,
			cli_config: cli_config,

			workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DNSProviders"),

			prSynced:   prInformer.Informer().HasSynced,
			prLister:   prInformer.Lister(),
			prInformer: prInformer,

			lbSynced:   lbInformer.Informer().HasSynced,
			lbLister:   lbInformer.Lister(),
			lbInformer: lbInformer,

			epSynced:   epInformer.Informer().HasSynced,
			epLister:   epInformer.Lister(),
			epInformer: epInformer,

			recorder: recorder,
		}

		lbInformerFactory.Start(ctx.Done())
		return controller

	} else {
		logctx.Infof("using load balancer config from config file '%s'", cli_config.Watches)
		controller := &Controller{
			LogCtx:     logctx,
			clientset:  clientset,
			cli_config: cli_config,
			watches:    cli_config.Watches,
		}
		return controller
	}

}

func (this *Controller) IsSingleton(lb *lbapi.DNSLoadBalancer) (bool, error) {
	singleton := false
	if lb.Spec.Singleton != nil {
		singleton = *lb.Spec.Singleton
		if lb.Spec.Type != "" {
			newlb := lb.DeepCopy()
			newlb.Status.State = "Error"
			newlb.Status.Message = "invalid load balancer type: singleton and type specicied"
			this.UpdateLB(lb, newlb)
			return false, fmt.Errorf("invalid load balancer type: singleton and type specicied")
		}
	}
	switch lb.Spec.Type {
	case lbapi.LBTYPE_EXCLUSIVE:
		singleton = true
	case lbapi.LBTYPE_BALANCED:
		singleton = false
	case "": // fill-in default
		newlb := lb.DeepCopy()
		if singleton {
			newlb.Spec.Type = lbapi.LBTYPE_EXCLUSIVE
		} else {
			newlb.Spec.Type = lbapi.LBTYPE_BALANCED
		}
		newlb.Spec.Singleton = nil
		this.Infof("adapt lb type for %s/%s", newlb.GetNamespace(), newlb.GetName())
		this.UpdateLB(lb, newlb)
	default:
		msg := "invalid load balancer type"
		if lb.Status.Message != msg || lb.Status.State != "Error" {
			newlb := lb.DeepCopy()
			newlb.Status.State = "Error"
			newlb.Status.Message = msg
			this.UpdateLB(lb, newlb)
		}
		return false, fmt.Errorf(msg)
	}
	return singleton, nil
}

func (this *Controller) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	this.recorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

func (this *Controller) UpdateProvider(pr *lbapi.DNSProvider) (*lbapi.DNSProvider, error) {
	return this.clientset.LoadbalancerV1beta1().DNSProviders(pr.GetNamespace()).Update(pr)
}

func (this *Controller) GetProvider(namespace, name string) (*lbapi.DNSProvider, error) {
	return this.prLister.DNSProviders(namespace).Get(name)
}

func (this *Controller) GetSecret(ref *corev1.SecretReference) (*corev1.Secret, error) {
	return this.clientset.CoreV1().Secrets(ref.Namespace).Get(ref.Name, metav1.GetOptions{})
}

func (this *Controller) UpdateLB(old, new *lbapi.DNSLoadBalancer) {
	if !reflect.DeepEqual(old, new) {
		this.Infof("updating dns load balancer for %s/%s", new.GetNamespace(), new.GetName())
		_, err := this.clientset.LoadbalancerV1beta1().DNSLoadBalancers(new.GetNamespace()).Update(new)
		if err != nil {
			this.Errorf("cannot update dns load balancer for %s/%s: %s", new.GetNamespace(), new.GetName(), err)
		}
	}
}

func (this *Controller) GetWatches() (*WatchConfig, error) {
	if this.watches != "" {
		return ReadConfig(this.watches)
	}

	config := &WatchConfig{}

	lblist, err := this.lbLister.DNSLoadBalancers("").List(labels.Everything())
	if err != nil {
		return nil, err
	}

	watches := map[ObjectRef]*Watch{}
	for _, lb := range lblist {
		singleton, err := this.IsSingleton(lb)
		if err != nil {
			continue
		}
		w := &Watch{DNS: lb.Spec.DNSName,
			HealthPath: lb.Spec.HealthPath,
			Singleton:  singleton,
			StatusCode: lb.Spec.StatusCode,
			DNSLB:      lb.DeepCopy(),
		}
		this.Debugf("found DNS LB for '%s'", w.DNS)
		watches[GetObjectRef(lb)] = w
		config.Watches = append(config.Watches, w)
	}

	eplist, err := this.epLister.DNSLoadBalancerEndpoints("").List(labels.Everything())
	if err != nil {
		return nil, err
	}
	this.Debugf("found %d endpoints", len(eplist))

	now := metav1.Now()
	for _, ep := range eplist {
		var w *Watch
		t := &Target_{IPAddress: ep.Spec.IPAddress, Name: ep.Spec.CName, DNSEP: ep}
		if t.IsValid() {
			ref := ObjectRef{Namespace: ep.Namespace, Name: ep.Spec.LoadBalancer}
			w = watches[ref]
			if now.Time.Before(this.started.Add(3*time.Minute)) || !this.handleCleanup(ep, w, &now) {
				if w != nil {
					w.Targets = append(w.Targets, t)
					this.Debugf("found %s target '%s' for '%s'", t.GetRecordType(), t.GetHostName(), ref)
				} else {
					this.Errorf("no lb found for '%s'", ref)
				}
			}
		} else {
			this.Warnf("invalid %s", t)
		}
	}
	return config, nil
}

func (this *Controller) handleCleanup(ep *lbapi.DNSLoadBalancerEndpoint, w *Watch, threshold *metav1.Time) bool {
	del := false
	if ep.Status.ValidUntil != nil {
		if ep.Status.ValidUntil.Before(threshold) {
			del = true
		}
	} else {
		if w == nil {
			del = true
		}
	}
	if del {
		this.clientset.LoadbalancerV1beta1().DNSLoadBalancerEndpoints(ep.GetNamespace()).Delete(ep.GetName(), &metav1.DeleteOptions{})
		if w != nil {
			this.recorder.Eventf(w.DNSLB, corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s/%s deleted", ep.GetNamespace(), ep.GetName())
		}
		this.Infof("outdated dns load balancer endpoint %s/%s deleted", ep.GetNamespace(), ep.GetName())
	}
	return del
}

func (this *Controller) registerDefaultProvider(reg *provider.TypeRegistration) error {
	name := "default-" + reg.GetName()
	typectx := this.LogCtx.NewLogContext("type", reg.GetName())
	logctx := typectx.NewLogContext("provider", name)
	p, err := reg.NewDefaultProvider(this.cli_config, logctx)
	if err != nil {
		this.Errorf("no default provider for provider type %s: %s", reg.GetName(), err)
	} else {
		this.Infof("registering default provider for aws")
		provider.RegisterProvider(name, p)
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

	if this.watches == "" {
		this.Infof("Waiting for informer caches to sync")
		if ok := cache.WaitForCacheSync(stopCh, this.prSynced, this.lbSynced, this.epSynced); !ok {
			return fmt.Errorf("failed to wait for caches to sync")
		}
	}

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
		go wait.Until(func() { this.runWorker(i) }, time.Second, stopCh)
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

func (this *Controller) runDNSUpdater(stopCh <-chan struct{}) error {
	model := NewModel(this.cli_config, this.recorder, this.clientset, this.LogCtx)
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
				this.UpdateDNS(model)
				if this.cli_config.Once {
					return nil
				}
				healthz.Tick("dns")
				timeout = time.After(time.Duration(this.cli_config.Interval) * time.Second)
			} else {
				return nil
			}
		}
	}
}

func (this *Controller) UpdateDNS(model *Model) {
	config, err := this.GetWatches()
	if err == nil {
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
	} else {
		this.Errorf("cannot read watch config '%s'\n", this.cli_config.Watches)
	}
}

func Run(clientset *controller.Clientset, ctx context.Context) error {

	c := NewController(clientset, ctx)
	return c.Run(ctx.Done())
}
