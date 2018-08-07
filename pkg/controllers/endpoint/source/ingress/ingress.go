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

package ingress

import (
	"fmt"

	extv1beta1 "k8s.io/api/extensions/v1beta1"

	kubeinformers "k8s.io/client-go/informers"
	informers "k8s.io/client-go/informers/extensions/v1beta1"
	listers "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/endpoint/source"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/endpoint/util"

	"github.com/sirupsen/logrus"
)

func init() {
	Register(&extv1beta1.Ingress{}, &IngressType{})
}

type IngressHandler struct {
	clientset clientset.Interface
	informer  informers.IngressInformer
	synced    cache.InformerSynced
	lister    listers.IngressLister
}

var _ SourceTypeHandler = &IngressHandler{}

func IngressAsSource(e *extv1beta1.Ingress) Source {
	o := &Ingress{e}
	o.Kind = "Ingress"
	return o
}

func (this *IngressHandler) GetSource(obj interface{}) (Source, error) {
	switch s := obj.(type) {
	case *extv1beta1.Ingress:
		return IngressAsSource(s), nil
	case SourceId:
		if s.GetKind() != "Ingress" {
			return nil, fmt.Errorf("ingress cannot handle kind '%s'", s.GetKind())
		}
		e, err := this.lister.Ingresses(s.GetNamespace()).Get(s.GetName())
		if err != nil {
			return nil, err
		}
		return IngressAsSource(e), nil
	default:
		return nil, fmt.Errorf("unsupported type '%T' for source object", obj)
	}
}

func (this *IngressHandler) AddEventHandler(stopCh <-chan struct{}, handlers cache.ResourceEventHandlerFuncs) error {
	logrus.Infof("adding handler for ingresses")
	if ok := cache.WaitForCacheSync(stopCh, this.synced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	this.informer.Informer().AddEventHandler(handlers)
	return nil
}

/////////////////////////////////////////////////////////////////////////////////

type IngressType struct {
}

var _ SourceType = &IngressType{}

func (this *IngressType) GetKind() string {
	return "Ingress"
}

func (this *IngressType) NewHandler(clientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory) SourceTypeHandler {
	informer := kubeInformerFactory.Extensions().V1beta1().Ingresses()

	handler := &IngressHandler{
		clientset: clientset,
		synced:    informer.Informer().HasSynced,
		lister:    informer.Lister(),
		informer:  informer,
	}

	return handler
}

type Ingress struct {
	*extv1beta1.Ingress
}

var _ Source = &Ingress{}

func (this *Ingress) GetKind() string {
	return "Ingress"
}

func (this *Ingress) DeepCopy() Source {
	return &Ingress{this.Ingress.DeepCopy()}
}

func (this *Ingress) GetEndpoint(lb *lbapi.DNSLoadBalancer) (ip, cname string) {
	for _, l := range this.Status.LoadBalancer.Ingress {
		if l.IP != "" {
			ip = l.IP
		}
		if l.Hostname != "" {
			cname = l.Hostname
		}
	}
	if cname == "" && ip == "" {
		for _, i := range this.Spec.Rules {
			if i.Host != "" && i.Host != lb.Spec.DNSName {
				cname = i.Host
				return
			}
		}
	}
	return
}

func (this *Ingress) Update(clientset clientset.Interface) error {
	_, err := clientset.ExtensionsV1beta1().Ingresses(this.GetNamespace()).Update(this.Ingress)
	return err
}

func (this *Ingress) Validate(lb *lbapi.DNSLoadBalancer) (bool, error) {
	dns := false
	for _, i := range this.Spec.Rules {
		if i.Host != "" {
			if i.Host == lb.Spec.DNSName {
				dns = true
			}
		}
	}
	if !dns {
		return false, fmt.Errorf("load balancer host '%s' not configured as host rule for '%s'", lb.Spec.DNSName, Ref(this))
	}
	cname, ip := this.GetEndpoint(lb)
	if cname == "" && ip == "" {
		return false, fmt.Errorf("no host rule or loadbalancer status defined for '%s'", Ref(this))
	}
	return true, nil
}
