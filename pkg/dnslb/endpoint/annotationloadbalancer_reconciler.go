package endpoint

import (
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"

	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/resources"
)

// annotationlb_reconciler ensures that annotated ingresses and services are enqueued if DNS load balancer spec has changed.
type annotationlb_reconciler struct {
	controller.Interface
	reconcile.DefaultReconciler
	sourceUsages *utils.SharedUsages
}

func AnnotationLoadBalancerReconcilerType(usages *utils.SharedUsages) controller.ReconcilerType {
	return func(c controller.Interface) (reconcile.Interface, error) {
		return AnnotationLoadBalancerReconciler(c, usages)
	}
}

func AnnotationLoadBalancerReconciler(c controller.Interface, usages *utils.SharedUsages) (reconcile.Interface, error) {
	return &annotationlb_reconciler{
		Interface:    c,
		sourceUsages: usages,
	}, nil
}

func (this *annotationlb_reconciler) Reconcile(logger logger.LogContext, obj resources.Object) reconcile.Status {
	set := this.sourceUsages.Get(obj.ClusterKey())
	logger.Infof("AnnotationLoadBalancerReconciler: reconcile load balancer %s for annotated source objects %v", obj.ObjectName(), set)
	for sourceKey := range set {
		this.Interface.EnqueueKey(sourceKey)
	}
	return reconcile.Succeeded(logger)
}
