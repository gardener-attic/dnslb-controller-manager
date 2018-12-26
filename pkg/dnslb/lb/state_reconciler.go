package lb

import (
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
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
	logger.Infof("reconcile endpoint %q", obj.ObjectName())
	this.state.UpdateEndpoint(logger, obj)
	return reconcile.Succeeded(logger)

}

func (this *stateReconciler) Deleted(logger logger.LogContext, key resources.ClusterObjectKey) reconcile.Status {
	logger.Infof("deleting endpoint %q", key)
	this.state.RemoveEndpoint(logger, key)
	return reconcile.Succeeded(logger)
}
