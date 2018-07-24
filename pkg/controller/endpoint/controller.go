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

package endpoint

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/k8s"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbscheme "github.com/gardener/dnslb-controller-manager/pkg/client/clientset/versioned/scheme"
	lbinformers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions"
	lbv1beta1informers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions/loadbalancer/v1beta1"
	lblisters "github.com/gardener/dnslb-controller-manager/pkg/client/listers/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/endpoint/source"
	. "github.com/gardener/dnslb-controller-manager/pkg/controller/endpoint/util"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"
	"github.com/gardener/dnslb-controller-manager/pkg/tools/workqueue"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const controllerAgentName = "dnselb-endpoint-controller"
const threadiness = 3

type Controller struct {
	log.LogCtx
	clusterid  string
	clientset  *controller.Clientset
	virtualset *controller.Clientset

	lbInformer lbv1beta1informers.DNSLoadBalancerInformer
	lbSynced   cache.InformerSynced
	lbLister   lblisters.DNSLoadBalancerLister

	epInformer lbv1beta1informers.DNSLoadBalancerEndpointInformer
	epSynced   cache.InformerSynced
	epLister   lblisters.DNSLoadBalancerEndpointLister

	sources source.Sources

	recorder record.EventRecorder

	workqueue workqueue.RateLimitingInterface
}

func NewController(clientset *controller.Clientset, ctx context.Context) *Controller {
	cli_config := config.Get(ctx)
	logctx := log.NewLogContext("controller", "enpoint")
	virtualset := ctx.Value("virtualset").(*controller.Clientset)
	kubeInformerFactory := ctx.Value("kubeInformerFactory").(kubeinformers.SharedInformerFactory)
	lbInformerFactory := ctx.Value("lbInformerFactory").(lbinformers.SharedInformerFactory)
	lbInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSLoadBalancers()
	epInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSLoadBalancerEndpoints()

	sources := source.NewSources(clientset, kubeInformerFactory)
	recorder := controller.NewEventRecorder(logctx, controllerAgentName, clientset, lbscheme.AddToScheme)

	controller := &Controller{
		LogCtx:     logctx,
		clusterid:  cli_config.Cluster,
		clientset:  clientset,
		virtualset: virtualset,
		workqueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EndpointSources"),
		sources:    sources,

		lbSynced:   lbInformer.Informer().HasSynced,
		lbLister:   lbInformer.Lister(),
		lbInformer: lbInformer,

		epSynced:   epInformer.Informer().HasSynced,
		epLister:   epInformer.Lister(),
		epInformer: epInformer,

		recorder: recorder,
	}
	kubeInformerFactory.Start(ctx.Done())
	lbInformerFactory.Start(ctx.Done())
	return controller
}

func (this *Controller) handleAddObject(new interface{}) {
	s, _ := this.sources.GetSource(new)
	f := HasFinalizer(s)
	n, err := GetLoadBalancerRef(s)
	if err != nil {
		this.recorder.Eventf(s, corev1.EventTypeWarning, "sync", err.Error())
		this.Errorf("%s : %s", source.Desc(s), err)
	}
	if f || n != nil {
		this.Debugf("-> new %s for dns loadbalancer %s", source.Desc(s), n)
		this.enqueue(s, true)
	}
}

func (this *Controller) handleDeleteObject(old interface{}) {
	s, _ := this.sources.GetSource(old)
	f := HasFinalizer(s)
	n, err := GetLoadBalancerRef(s)
	if err != nil {
		this.recorder.Eventf(s, corev1.EventTypeWarning, "delete", err.Error())
		this.Errorf("%s", source.Desc(s), err)
	}
	if f || n != nil {
		this.Debugf("-> delete %s for dns loadbalancer %s", source.Desc(s), n)
		this.enqueue(s, true)
	}
}

func (this *Controller) handleUpdateObject(old, new interface{}) {
	newS, _ := this.sources.GetSource(new)
	oldS, _ := this.sources.GetSource(old)

	f := HasFinalizer(newS)
	o, _ := GetLoadBalancerRef(oldS)

	n, err := GetLoadBalancerRef(newS)
	if err != nil {
		this.Errorf("%s: %s", source.Desc(newS), err)
	}
	if f || n != nil || o != nil {
		if newS.GetResourceVersion() == oldS.GetResourceVersion() {
			// Periodic resync will send update events for all known Resources.
			// Two different versions of the same Resource will always have different RVs.
			this.Debugf("-> reconcile %s for dns loadbalancer %s", source.Desc(oldS), n)
			this.enqueue(newS, false)
		} else {
			this.Debugf("-> update %s for dns loadbalancer %s", source.Desc(oldS), n)
			this.enqueue(newS, true)
		}
	}
}

// enqueue adds an object to the working queue.
// true: object has changed, ignore actual error state and rate limit
// false: add object rate limited, if last processing dediced to Forget
//        this just adds it to the queue
func (this *Controller) enqueue(obj source.Source, renew bool) {
	var key string
	var err error
	if key, err = k8s.ObjectKeyFunc(obj); err != nil {
		this.Error(err)
		return
	}
	if renew {
		this.workqueue.AddChanged(key)
	} else {
		this.workqueue.AddRateLimited(key)
	}
}

func (this *Controller) GetSource(id source.SourceId) (source.Source, error) {
	return this.sources.GetSource(id)
}

func (this *Controller) UpdateSource(src source.Source) error {
	return this.sources.UpdateSource(src)
}

func (this *Controller) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	this.recorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

func (this *Controller) CreateEndpoint(ep *lbapi.DNSLoadBalancerEndpoint) (*lbapi.DNSLoadBalancerEndpoint, error) {
	return this.virtualset.LoadbalancerV1beta1().DNSLoadBalancerEndpoints(ep.GetNamespace()).Create(ep)
}

func (this *Controller) UpdateEndpoint(ep *lbapi.DNSLoadBalancerEndpoint) (*lbapi.DNSLoadBalancerEndpoint, error) {
	return this.virtualset.LoadbalancerV1beta1().DNSLoadBalancerEndpoints(ep.GetNamespace()).Update(ep)
}

func (this *Controller) DeleteEndpoint(namespace, name string) error {
	return this.virtualset.LoadbalancerV1beta1().DNSLoadBalancerEndpoints(namespace).Delete(name, &metav1.DeleteOptions{})
}

/////////////////////////////////////////////////////////////////////////////////

// Worker describe a single threaded worker entity synchronously working
// on requests provided by the controller workqueue
// It is basically a single go routine with a state for subsequenet methods
// called from this go routine
type Worker struct {
	log.LogCtx
	ctx        log.LogCtx
	controller *Controller
	workqueue  workqueue.RateLimitingInterface
}

func (this *Controller) runWorker(no int) {
	w := &Worker{}

	w.ctx = this.NewLogContext("worker", strconv.Itoa(no))
	w.controller = this
	w.workqueue = this.workqueue

	for w.processNextWorkItem() {
	}
}

func (this *Worker) internalErr(obj interface{}, err error) bool {
	this.workqueue.Forget(obj)
	this.ctx.Error(err)
	return true
}

func (this *Worker) processNextWorkItem() bool {
	obj, shutdown := this.workqueue.Get()

	if shutdown {
		return false
	}
	healthz.Tick("endpoint")

	defer this.workqueue.Done(obj)
	key, ok := obj.(string)
	if !ok {
		return this.internalErr(obj, fmt.Errorf("expected string in workqueue but got %#v", obj))
	}
	kind, namespace, name, err := k8s.SplitObjectKey(key)
	if err != nil {
		return this.internalErr(obj, fmt.Errorf("error syncing '%s': %s", key, err))
	}
	this.LogCtx = this.ctx.NewLogContext("resource", key)
	id := source.NewSourceId(kind, namespace, name)

	s, err := this.controller.GetSource(id)
	if err != nil {
		// The Service resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			return this.internalErr(obj, fmt.Errorf("%s in work queue no longer exists", key))
		} else {
			this.Errorf("error syncing '%s': %s", key, err)
			this.workqueue.AddRateLimited(key)
			return true
		}
	} else {

		s = s.DeepCopy()
		if s.GetDeletionTimestamp() == nil {
			ok, err = this.handleReconcile(s)
		} else {
			ok, err = this.handleDelete(s)

		}
		if err != nil {
			this.controller.Eventf(s, corev1.EventTypeWarning, "sync", err.Error())
			//runtime.HandleError(fmt.Errorf("error syncing '%s': %s", key, err))
			if ok {
				// some problem reported, but valid state -> rate limit
				this.Errorf("problem syncing '%s': %s", key, err)
				this.workqueue.AddRateLimited(key)
			} else {
				// object config error -> wait for new change of object
				this.Errorf("wait for new change '%s': %s", key, err)
				this.workqueue.WaitForChange(obj)
			}
		} else {
			if ok {
				// no error, and everything valid -> just reset rate limter
				this.workqueue.Forget(obj)
			} else {
				// operation temporarily failed (no error) -> just redo operation
				this.Infof("redo reconcile for '%s':", key)
				this.workqueue.Add(obj)
			}
		}
	}
	this.Debugf("done with %s", key)
	return true
}

func (this *Controller) Run(stopCh <-chan struct{}) error {

	defer runtimeutil.HandleCrash()
	defer this.workqueue.ShutDown()

	this.Debugf("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(func() { this.runWorker(i) }, time.Second, stopCh)
	}

	err := this.sources.AddEventHandler(stopCh, cache.ResourceEventHandlerFuncs{
		AddFunc:    this.handleAddObject,
		UpdateFunc: this.handleUpdateObject,
		DeleteFunc: this.handleDeleteObject,
	})
	if err != nil {
		return err
	}
	<-stopCh
	this.Infof("Shutting down workers")
	return nil
}

func Run(clientset *controller.Clientset, ctx context.Context) error {
	logrus.Infof("running loadbalancer endpoint controller")
	c := NewController(clientset, ctx)
	return c.Run(ctx.Done())
}
