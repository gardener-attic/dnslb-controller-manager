package endpoint

import (

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"


	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
)

type endpoint_reconciler struct {
	baseReconciler
}


func EndpointReconciler(c controller.Interface) (reconcile.Interface, error) {
	ep, err:=c.GetDefaultCluster().GetResource(resources.NewGroupKind(api.GroupName, api.LoadBalancerEndpointResourceKind))
	if err != nil {
		return nil,err
	}
	return &endpoint_reconciler{
		baseReconciler: baseReconciler{controller: c, ep_resource:ep},
	}, nil
}


func (this *endpoint_reconciler) Reconcile(logger logger.LogContext, obj resources.Object) reconcile.Status {
	this.slaves.UpdateSlave(obj)
	return reconcile.Succeeded(logger)
}

func (this *endpoint_reconciler) Deleted(logger logger.LogContext, key resources.ClusterObjectKey) reconcile.Status {
	this.slaves.DeleteSlave(key)
	return reconcile.Succeeded(logger)
}
