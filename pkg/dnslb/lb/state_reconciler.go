package lb

import (
	"fmt"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/resources"
	"k8s.io/apimachinery/pkg/api/errors"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"
)

func StateReconciler(c controller.Interface) (reconcile.Interface, error) {
	state := c.GetOrCreateSharedValue(KEY_STATE,
		func() interface{} {
			return NewState(c)
		}).(*State)

	return &stateReconciler{
		controller: c,
		state:      state,
	}, nil
}

type stateReconciler struct {
	reconcile.DefaultReconciler
	controller controller.Interface
	state      *State
}

func (this *stateReconciler) Reconcile(logger logger.LogContext, obj resources.Object) reconcile.Status {
	logger.Infof("reconcile endpoint %q", obj.ClusterKey())

	this.state.UpdateEndpoint(logger, obj)
	ep:=lbutils.DNSLoadBalancerEndpoint(obj)
	lbref:=ep.GetLoadBalancerRef()
	_,err:=obj.GetCluster().Resources().GetCachedObject(lbref)
	if errors.IsNotFound(err) {
		ep.UpdateState(api.STATE_ERROR,fmt.Sprintf("loadbalancer %q not found", lbref), nil)
	}
	return reconcile.Succeeded(logger)

}

func (this *stateReconciler) Deleted(logger logger.LogContext, key resources.ClusterObjectKey) reconcile.Status {
	logger.Infof("deleting endpoint %q", key)
	this.state.RemoveEndpoint(logger, key)
	return reconcile.Succeeded(logger)
}
