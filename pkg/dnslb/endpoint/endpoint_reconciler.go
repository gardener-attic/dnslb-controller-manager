package endpoint

import (
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile/reconcilers"
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/resources"
)

type endpoint_reconciler struct {
	*reconcilers.SlaveReconciler
}


func EndpointReconciler(c controller.Interface) (reconcile.Interface, error) {
	return &endpoint_reconciler{
		reconcilers.NewSlaveReconciler(c,"endpoint", SlaveResources, nil),
	}, nil
}


func (this *endpoint_reconciler) Reconcile(logger logger.LogContext, obj resources.Object) reconcile.Status {
	return this.SlaveReconciler.Reconcile(logger, obj)
}