// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lb

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"

	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/resources"
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
	ep := lbutils.DNSLoadBalancerEndpoint(obj)
	err := ep.Validate()
	mod := false
	if err != nil {
		mod, err = ep.UpdateState(api.STATE_INVALID, err.Error(), nil)
	} else {
		if reconcile.StringEqual(ep.Status().State, api.STATE_INVALID) {
			mod, err = ep.UpdateState(api.STATE_PENDING, "", nil)
		}
	}
	if mod {
		logger.Warnf("changing state to %s(%s)",
			reconcile.StringValue(ep.Status().State),
			reconcile.StringValue(ep.Status().Message))
	}
	return reconcile.UpdateStatus(logger, err)

}

func (this *stateReconciler) Deleted(logger logger.LogContext, key resources.ClusterObjectKey) reconcile.Status {
	logger.Infof("deleting endpoint %q", key)
	this.state.RemoveEndpoint(logger, key)
	return reconcile.Succeeded(logger)
}
