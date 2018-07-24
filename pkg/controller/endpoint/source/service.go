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

package source

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	kubeinformers "k8s.io/client-go/informers"
	informers "k8s.io/client-go/informers/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controller"
	. "github.com/gardener/dnslb-controller-manager/pkg/controller/endpoint/util"

	"github.com/sirupsen/logrus"
)

func init() {
	Register(&corev1.Service{}, &ServiceType{})
}

type ServiceHandler struct {
	clientset *controller.Clientset
	informer  informers.ServiceInformer
	synced    cache.InformerSynced
	lister    listers.ServiceLister
}

var _ SourceTypeHandler = &ServiceHandler{}

func ServiceAsSource(e *corev1.Service) Source {
	o := &Service{e}
	o.Kind = "Service"
	return o
}

func (this *ServiceHandler) GetSource(obj interface{}) (Source, error) {
	switch s := obj.(type) {
	case *corev1.Service:
		return ServiceAsSource(s), nil
	case SourceId:
		if s.GetKind() != "Service" {
			return nil, fmt.Errorf("service cannot handle kind '%s'", s.GetKind())
		}
		e, err := this.lister.Services(s.GetNamespace()).Get(s.GetName())
		if err != nil {
			return nil, err
		}
		return ServiceAsSource(e), nil
	default:
		return nil, fmt.Errorf("unsupported type '%T' for source object", obj)
	}
}

func (this *ServiceHandler) AddEventHandler(stopCh <-chan struct{}, handlers cache.ResourceEventHandlerFuncs) error {
	logrus.Infof("adding handler for services")
	if ok := cache.WaitForCacheSync(stopCh, this.synced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	this.informer.Informer().AddEventHandler(handlers)
	return nil
}

/////////////////////////////////////////////////////////////////////////////////

type ServiceType struct {
}

var _ SourceType = &ServiceType{}

func (this *ServiceType) GetKind() string {
	return "Service"
}

func (this *ServiceType) NewHandler(clientset *controller.Clientset,
	kubeInformerFactory kubeinformers.SharedInformerFactory) SourceTypeHandler {
	informer := kubeInformerFactory.Core().V1().Services()

	handler := &ServiceHandler{
		clientset: clientset,
		synced:    informer.Informer().HasSynced,
		lister:    informer.Lister(),
		informer:  informer,
	}

	return handler
}

/////////////////////////////////////////////////////////////////////////////////

type Service struct {
	*corev1.Service
}

var _ Source = &Service{}

func (this *Service) GetKind() string {
	return "Service"
}

func (this *Service) DeepCopy() Source {
	return &Service{this.Service.DeepCopy()}
}

func (this *Service) GetEndpoint(lb *lbapi.DNSLoadBalancer) (ip, cname string) {
	for _, i := range this.Status.LoadBalancer.Ingress {
		if i.Hostname != "" {
			cname = i.Hostname
		}
		if i.IP != "" {
			ip = i.IP
		}
	}
	return
}

func (this *Service) Update(clientset *controller.Clientset) error {
	_, err := clientset.CoreV1().Services(this.GetNamespace()).Update(this.Service)
	return err
}

func (this *Service) Validate(lb *lbapi.DNSLoadBalancer) (bool, error) {
	ok, err := HasLoadBalancer(this.Service)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, fmt.Errorf("load balancer not yet assigned for '%s'", Ref(this))
	}
	return true, nil
}

func HasLoadBalancer(svc *corev1.Service) (bool, error) {
	if svc.Spec.Type != "LoadBalancer" {
		return false, fmt.Errorf("service %s/%s is not of type LoadBalancer",
			svc.Namespace, svc.Name)
	}
	for _, i := range svc.Status.LoadBalancer.Ingress {
		if i.IP != "" || i.Hostname != "" {
			return true, nil
		}
	}
	return false, nil
}
