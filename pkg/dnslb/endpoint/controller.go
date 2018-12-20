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
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/lib/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/service"
)

const AnnotationLoadbalancer = api.GroupName + "/dnsloadbalancer"

func init() {
	controller.Configure("dnslb-endpoint").
		FinalizerDomain(api.GroupName).
		Reconciler(EndpointReconciler).
		Reconciler(SourceReconciler,"sources").
		Pool("sources").ReconcilerWatch("sources", corev1.GroupName, "Service").
		MainResource(api.GroupName,api.LoadBalancerEndpointResourceKind).
		MustRegister("source")
}

type baseReconciler struct {
	reconcile.DefaultReconciler
	controller  controller.Interface
	ep_resource resources.Interface
	slaves      *resources.SlaveCache
}

func (h *baseReconciler) Setup() {
	h.slaves = h.controller.GetOrCreateSharedValue("slaves", h.SetupSlaveCache).(*resources.SlaveCache)
}

func (h *baseReconciler) SetupSlaveCache() interface{} {
	cache:=resources.NewSlaveCache()

	h.controller.Infof("setup endpoint owner cache")
	list, _ := h.ep_resource.ListCached(labels.Everything())
	cache.Setup(list)
	h.controller.Infof("setup done")
	return cache
}

func (this *baseReconciler) lookupEndpoint(obj resources.ClusterObjectKey) resources.Object {
	return this.slaves.GetSlave(obj)
}