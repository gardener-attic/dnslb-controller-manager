// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package endpoint

import (
	"github.com/gardener/controller-manager-library/pkg/controllermanager/cluster"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/reconcile/reconcilers"
	"github.com/gardener/controller-manager-library/pkg/resources"
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"time"

	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

const AnnotationLoadbalancer = api.GroupName + "/dnsloadbalancer"

const TARGET_CLUSTER = "target"

const LBUSAGES = "loadbalancer"

const OPT_TARGETCHECKPERIOD = "target-check-period"

var serviceGK = resources.NewGroupKind(corev1.GroupName, "Service")
var ingressGK = resources.NewGroupKind(extensions.GroupName, "Ingress")

func init() {
	err := cluster.Register("target", "target", "target cluster for endpoints")
	if err != nil {
		panic(err)
	}

	controller.Configure("dnslb-endpoint").
		FinalizerDomain(api.GroupName).
		Cluster(cluster.DEFAULT). // used as main cluster
		DefaultedDurationOption(OPT_TARGETCHECKPERIOD, 60*time.Second, "period for checking targets").
		DefaultWorkerPool(3, 0).
		MainResource(corev1.GroupName, "Service").
		Watch(extensions.GroupName, "Ingress").
		Reconciler(SourceReconciler).
		Reconciler(reconcilers.UsageReconcilerTypeBySpec(nil, lbUsageAccessSpec), "usages").
		Cluster(TARGET_CLUSTER).
		WorkerPool("endpoints", 3, 0).
		Reconciler(reconcilers.SlaveReconcilerTypeBySpec(nil, lbSlaveAccessSpec), "endpoints").
		ReconcilerWatch("endpoints", api.GroupName, api.LoadBalancerEndpointResourceKind).
		ReconcilerWatch("usages", api.GroupName, api.LoadBalancerResourceKind).
		MustRegister("source")
}

var SlaveResources = reconcilers.ClusterResources(TARGET_CLUSTER, api.LoadBalancerEndpointGroupKind)
var MasterResources = reconcilers.ClusterResources(controller.CLUSTER_MAIN, serviceGK, ingressGK)

var lbSlaveAccessSpec = reconcilers.SlaveAccessSpec{Name: "endpoint", Slaves: SlaveResources, Masters: MasterResources}
var lbUsageAccessSpec = reconcilers.UsageAccessSpec{Name: LBUSAGES, ExtractorFactory: LBFunc, MasterResources: MasterResources}

func LBFunc(c controller.Interface) resources.UsedExtractor {
	clusterid := c.GetCluster(TARGET_CLUSTER).GetId()
	return func(obj resources.Object) resources.ClusterObjectKeySet {
		n, _ := LBForSource(obj)
		if n == nil {
			return nil
		}
		return resources.NewClusterObjectKeySet(n.ForGroupKind(api.LoadBalancerGroupKind).ForCluster(clusterid))
	}
}
