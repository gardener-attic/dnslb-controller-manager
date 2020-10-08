// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package endpoint

import (
	"fmt"
	"strings"
	"time"

	dnsutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"
	"k8s.io/apimachinery/pkg/api/errors"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources"

	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (this *source_reconciler) newEndpoint(logger logger.LogContext, lb resources.Object, src sources.Source) *dnsutils.DNSLoadBalancerEndpointObject {
	labels := map[string]string{
		"controller": this.FinalizerHandler().FinalizerName(lb),
		"source":     fmt.Sprintf("%s", src.Key()),
	}
	if lb.GetClusterName() != src.GetClusterName() {
		labels["cluster"] = fmt.Sprintf("%s", src.GetCluster().GetId())
	}

	ip, cname := src.GetTargets(lb)
	n := this.UpdateDeadline(logger, lb.Data().(*api.DNSLoadBalancer).Spec.EndpointValidityInterval, nil)
	r, _ := this.ep_resource.Wrap(&api.DNSLoadBalancerEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: src.GetName() + "-" + strings.ToLower(src.GroupKind().Kind) + "-",
			Namespace:    lb.GetNamespace(),
		},
		Spec: api.DNSLoadBalancerEndpointSpec{
			IPAddress:    ip,
			CName:        cname,
			LoadBalancer: lb.GetName(),
		},
		Status: api.DNSLoadBalancerEndpointStatus{
			ValidUntil: n,
		},
	})
	r.AddOwner(src)
	return dnsutils.DNSLoadBalancerEndpoint(r)
}

func (this *source_reconciler) updateEndpoint(logger logger.LogContext, oldep, newep resources.Object, lb resources.Object, src sources.Source) *resources.ModificationState {
	n := dnsutils.DNSLoadBalancerEndpoint(newep).DNSLoadBalancerEndpoint()
	o := dnsutils.DNSLoadBalancerEndpoint(oldep).DNSLoadBalancerEndpoint()
	mod := resources.NewModificationState(oldep)
	mod.AssureLabel("controller", newep.GetLabel("controller"))
	mod.AssureLabel("source", newep.GetLabel("source"))
	mod.AssureLabel("cluster", newep.GetLabel("cluster"))
	mod.AddOwners(src)

	mod.AssureStringValue(&o.Spec.IPAddress, n.Spec.IPAddress)
	mod.AssureStringValue(&o.Spec.CName, n.Spec.CName)
	mod.AssureStringValue(&o.Spec.LoadBalancer, n.Spec.LoadBalancer)

	lbspec := dnsutils.DNSLoadBalancer(lb).Spec()
	t := this.UpdateDeadline(logger, lbspec.EndpointValidityInterval, o.Status.ValidUntil)
	if t != o.Status.ValidUntil {
		o.Status.ValidUntil = t
		mod.Modify(true)
	}
	return mod
}

func (this *source_reconciler) UpdateDeadline(logger logger.LogContext, duration *metav1.Duration, deadline *metav1.Time) *metav1.Time {
	if duration != nil && duration.Duration != 0 {
		logger.Debugf("validity interval found: %s", duration.Duration)
		now := time.Now()
		if deadline != nil {
			sub := deadline.Time.Sub(now)
			if sub > 120*time.Second {
				logger.Debugf("time diff: %s", sub)
				return deadline
			}
		}
		next := metav1.NewTime(now.Add(duration.Duration))
		logger.Debugf("update deadline (%s+%s): %s", now, duration.Duration, next.Time)
		return &next
	}
	return nil
}

func (this *source_reconciler) deleteEndpoint(logger logger.LogContext, src resources.Object, ep resources.Object) error {
	if ep != nil {
		err := ep.Delete()
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("error deleting load balancer endpoint '%s': %s", ep.ObjectName(), err)
			}
		}
		src.Eventf(corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s deleted", ep.ObjectName())
		logger.Infof("dns load balancer endpoint %s deleted", ep.ObjectName())
	}
	return nil
}
