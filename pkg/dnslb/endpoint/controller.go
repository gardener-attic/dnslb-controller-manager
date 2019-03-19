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
	"github.com/gardener/controller-manager-library/pkg/controllermanager/cluster"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/reconcile/reconcilers"
	"github.com/gardener/controller-manager-library/pkg/resources"
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"

	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

const AnnotationLoadbalancer = api.GroupName + "/dnsloadbalancer"

const TARGET_CLUSTER = "target"
const KEY_USAGES = "source-usages"

var serviceGK = resources.NewGroupKind(corev1.GroupName, "Service")
var ingressGK = resources.NewGroupKind(extensions.GroupName, "Ingress")

func init() {
	cluster.Register("target", "target", "target cluster for endpoints")

	controller.Configure("dnslb-endpoint").
		FinalizerDomain(api.GroupName).
		Cluster(cluster.DEFAULT). // used as main cluster
		DefaultWorkerPool(3, 0).
		MainResource(corev1.GroupName, "Service").
		Watch(extensions.GroupName, "Ingress").
		Reconciler(SourceReconciler).
		Cluster(TARGET_CLUSTER).
		WorkerPool("endpoints", 3, 0).
		Reconciler(reconcilers.SlaveReconcilerType("endpoint", SlaveResources, nil, MasterResources), "endpoints").
		ReconcilerWatch("endpoints", api.GroupName, api.LoadBalancerEndpointResourceKind).
		Reconciler(AnnotationLoadBalancerReconciler, "annotationLoadbalancers").
		ReconcilerWatch("annotationLoadbalancers", api.GroupName, api.LoadBalancerResourceKind).
		MustRegister("source")
}

var SlaveResources = reconcilers.ClusterResources(TARGET_CLUSTER, api.LoadBalancerEndpointGroupKind)
var MasterResources = reconcilers.ClusterResources(controller.CLUSTER_MAIN, serviceGK, ingressGK)
