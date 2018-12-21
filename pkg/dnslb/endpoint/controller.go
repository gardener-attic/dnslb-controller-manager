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
	"fmt"
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/resources"

	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/ingress"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/service"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

const AnnotationLoadbalancer = api.GroupName + "/dnsloadbalancer"

var endpointGK=resources.NewGroupKind(api.GroupName, api.LoadBalancerEndpointResourceKind)

func init() {
	controller.Configure("dnslb-endpoint").
		FinalizerDomain(api.GroupName).
		Reconciler(EndpointReconciler).
		Reconciler(SourceReconciler, "sources").
		WorkerPool("sources", 3, 0).ReconcilerWatch("sources", corev1.GroupName, "Service").
		ReconcilerWatch("sources", extensions.GroupName, "Ingress").
		MainResource(api.GroupName, api.LoadBalancerEndpointResourceKind).
		MustRegister("source")
}

func SlaveResources(c controller.Interface) []resources.Interface {
	res, err:= c.GetDefaultCluster().Resources().Get(endpointGK)
	if err!=nil {
		panic(fmt.Errorf("resources type %s not found: %s", endpointGK, err))
	}
	return []resources.Interface{res}
}