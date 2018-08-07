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
	"reflect"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
	"github.com/gardener/dnslb-controller-manager/pkg/k8s"

	"github.com/sirupsen/logrus"
)

type SourceId struct {
	kind      string
	namespace string
	name      string
}

func (this *SourceId) GetKind() string      { return this.kind }
func (this *SourceId) GetNamespace() string { return this.namespace }
func (this *SourceId) GetName() string      { return this.name }

func NewSourceId(kind, namespace, name string) SourceId {
	return SourceId{kind: kind, namespace: namespace, name: name}
}

/////////////////////////////////////////////////////////////////////////////////

type SourceType interface {
	GetKind() string
	NewHandler(clientset.Interface, kubeinformers.SharedInformerFactory) SourceTypeHandler
}

var types = map[reflect.Type]SourceType{}

func Register(key interface{}, stype SourceType) {
	if t, ok := key.(reflect.Type); ok {
		logrus.Infof("register endpoint source type %T by type", stype)
		types[t] = stype
	} else {
		logrus.Infof("register endpoint source type %T by elem type %T", stype, key)
		types[reflect.TypeOf(key)] = stype
	}
}

/////////////////////////////////////////////////////////////////////////////////

func Desc(s Source) string {
	return fmt.Sprintf("%s %s/%s", s.GetKind(), s.GetNamespace(), s.GetName())
}

/////////////////////////////////////////////////////////////////////////////////

// Validation Contract:
// true,  nil: valid source, everything ok, just do reconcile
// true,  err: valid source, but required state not reached, redo rate limited
// false, err: invalid source by configuration
// false, nil: operation temporarily failed, just redo

type Source interface {
	k8s.Object
	DeepCopy() Source

	GetKind() string
	GetEndpoint(lb *lbapi.DNSLoadBalancer) (IPAddress, CName string)
	Validate(lb *lbapi.DNSLoadBalancer) (bool, error)
	Update(clientset.Interface) error
}

type SourceTypeHandler interface {
	GetSource(interface{}) (Source, error)
	AddEventHandler(stopCh <-chan struct{}, eventHandlers cache.ResourceEventHandlerFuncs) error
}

/////////////////////////////////////////////////////////////////////////////////
// Gneeric Resource Handling

type Sources interface {
	SourceTypeHandler
	UpdateSource(Source) error
}

type GenericHandler struct {
	clientset      clientset.Interface
	handlersByType map[reflect.Type]SourceTypeHandler
	handlersByKind map[string]SourceTypeHandler
}

var _ Sources = &GenericHandler{}

func NewSources(clientset clientset.Interface,
	informerfactory kubeinformers.SharedInformerFactory) Sources {

	h := &GenericHandler{}
	return h.new(clientset, informerfactory)
}

func (this *GenericHandler) new(clientset clientset.Interface,
	informerfactory kubeinformers.SharedInformerFactory) *GenericHandler {

	this.clientset = clientset
	this.handlersByType = map[reflect.Type]SourceTypeHandler{}
	this.handlersByKind = map[string]SourceTypeHandler{}
	for rt, st := range types {
		h := st.NewHandler(clientset, informerfactory)
		logrus.Infof("adding source for %v(%s): %T", rt, st.GetKind(), h)
		this.handlersByType[rt] = h
		this.handlersByKind[st.GetKind()] = h
	}
	return this
}

func (this *GenericHandler) UpdateSource(src Source) error {
	return src.Update(this.clientset)
}

func (this *GenericHandler) GetSource(obj interface{}) (Source, error) {
	var h SourceTypeHandler
	key, ok := obj.(SourceId)
	if !ok {
		h = this.handlersByType[reflect.TypeOf(obj)]
		if h == nil {
			return nil, fmt.Errorf("unknown type %T", obj)
		}
	} else {
		h = this.handlersByKind[key.GetKind()]
		if h == nil {
			return nil, fmt.Errorf("unknown kind %s", key.GetKind())
		}
	}
	return h.GetSource(obj)
}

func (this *GenericHandler) AddEventHandler(stopCh <-chan struct{}, handlers cache.ResourceEventHandlerFuncs) error {
	for _, h := range this.handlersByKind {
		err := h.AddEventHandler(stopCh, handlers)
		if err != nil {
			return err
		}
	}
	return nil
}
