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

package dnslb

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cobra"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/model"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/source"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/util"
	"github.com/gardener/dnslb-controller-manager/pkg/log"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbinformers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions"
	lbv1beta1informers "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions/loadbalancer/v1beta1"
	lblisters "github.com/gardener/dnslb-controller-manager/pkg/client/listers/loadbalancer/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func init() {
	source.Register(&SourceType{})
}

type SourceType struct {
}

func (this *SourceType) ConfigureCommand(cmd *cobra.Command, cli_config *config.CLIConfig) {
}

func (this *SourceType) Create(acc source.Access, clientset clientset.Interface, cli_config *config.CLIConfig, ctx context.Context) (source.Source, error) {
	acc.Infof("using in cluster scan for load balancer resources")
	lbInformerFactory := ctx.Value("targetInformerFactory").(lbinformers.SharedInformerFactory)

	if lbInformerFactory == nil {
		return nil, fmt.Errorf("targetInformerFactory not set in context")
	}

	lbInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSLoadBalancers()
	epInformer := lbInformerFactory.Loadbalancer().V1beta1().DNSLoadBalancerEndpoints()

	return &Source{
		LogCtx:    acc.NewLogContext("source", "dnslbs"),
		access:    acc,
		clientset: clientset,

		lbSynced:   lbInformer.Informer().HasSynced,
		lbLister:   lbInformer.Lister(),
		lbInformer: lbInformer,

		epSynced:   epInformer.Informer().HasSynced,
		epLister:   epInformer.Lister(),
		epInformer: epInformer,
	}, nil
}

type Source struct {
	log.LogCtx
	access    source.Access
	clientset clientset.Interface

	lbInformer lbv1beta1informers.DNSLoadBalancerInformer
	lbSynced   cache.InformerSynced
	lbLister   lblisters.DNSLoadBalancerLister

	epInformer lbv1beta1informers.DNSLoadBalancerEndpointInformer
	epSynced   cache.InformerSynced
	epLister   lblisters.DNSLoadBalancerEndpointLister

	started time.Time
}

var _ source.Source = &Source{}

func (this *Source) Setup(stopCh <-chan struct{}) error {
	this.Infof("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, this.lbSynced, this.epSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	this.started = time.Now()
	return nil
}

func (this *Source) Get() ([]*model.Watch, error) {
	config := &model.WatchConfig{}
	watches := map[ObjectRef]*model.Watch{}

	this.Debugf("getting dnslb resource watches")
	lblist, err := this.lbLister.DNSLoadBalancers("").List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, lb := range lblist {
		singleton, err := this.IsSingleton(lb)
		if err != nil {
			continue
		}
		w := &model.Watch{DNS: lb.Spec.DNSName,
			HealthPath: lb.Spec.HealthPath,
			Singleton:  singleton,
			StatusCode: lb.Spec.StatusCode,
			DNSLB:      lb.DeepCopy(),
		}
		this.Debugf("found DNS LB for '%s'", w.DNS)
		watches[GetObjectRef(lb)] = w
		config.Watches = append(config.Watches, w)
	}

	this.Debugf("found %d watches", len(config.Watches))
	eplist, err := this.epLister.DNSLoadBalancerEndpoints("").List(labels.Everything())
	if err != nil {
		return nil, err
	}
	this.Debugf("found %d endpoints", len(eplist))

	now := metav1.Now()
	for _, ep := range eplist {
		var w *model.Watch
		t := &model.Target_{IPAddress: ep.Spec.IPAddress, Name: ep.Spec.CName, DNSEP: ep}
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
	return config.Watches, nil
}

func (this *Source) IsSingleton(lb *lbapi.DNSLoadBalancer) (bool, error) {
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

func (this *Source) handleCleanup(ep *lbapi.DNSLoadBalancerEndpoint, w *model.Watch, threshold *metav1.Time) bool {
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
			this.access.Eventf(w.DNSLB, corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s/%s deleted", ep.GetNamespace(), ep.GetName())
		}
		this.Infof("outdated dns load balancer endpoint %s/%s deleted", ep.GetNamespace(), ep.GetName())
	}
	return del
}

func (this *Source) UpdateLB(old, new *lbapi.DNSLoadBalancer) {
	if !reflect.DeepEqual(old, new) {
		this.Infof("updating dns load balancer for %s/%s", new.GetNamespace(), new.GetName())
		_, err := this.clientset.LoadbalancerV1beta1().DNSLoadBalancers(new.GetNamespace()).Update(new)
		if err != nil {
			this.Errorf("cannot update dns load balancer for %s/%s: %s", new.GetNamespace(), new.GetName(), err)
		}
	}
}
