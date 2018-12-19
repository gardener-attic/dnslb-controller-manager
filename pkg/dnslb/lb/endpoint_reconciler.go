package lb

import (
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
)

func EndpointReconciler(c controller.Interface) (reconcile.Interface, error) {
	state := c.GetOrCreateSharedValue(KEY_STATE,
		func() interface{} {
			return NewState(c)
		}).(*State)

	return &endpointReconciler{
		controller: c,
		state:      state,
	}, nil
}

type endpointReconciler struct {
	reconcile.DefaultReconciler
	controller controller.Interface
	state      *State
}

func (this *endpointReconciler) Reconcile(logger logger.LogContext, obj resources.Object) reconcile.Status {
	logger.Infof("reconcile entry %q", obj.ObjectName())
	this.state.UpdateEndpoint(logger, obj)
	return reconcile.Succeeded(logger)

}

func (this *endpointReconciler) Deleted(logger logger.LogContext, key resources.ClusterObjectKey) reconcile.Status {
	logger.Infof("deleting endpoint %q", key)
	this.state.RemoveEndpoint(logger, key)
	return reconcile.Succeeded(logger)
}
