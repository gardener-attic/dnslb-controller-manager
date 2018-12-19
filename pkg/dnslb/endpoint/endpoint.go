package endpoint

import (
	"fmt"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources"
	"github.com/golang/protobuf/ptypes/duration"
	"k8s.io/apimachinery/pkg/runtime/schema"
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func (this *reconciler) newEndpoint(logger logger.LogContext, lb resources.Object, src sources.Source) *api.DNSLoadBalancerEndpoint {
	labels := map[string]string{
		"controller": this.controller.FinalizerName(),
		"source":     fmt.Sprintf("%s", src.Key()),
	}
	if lb.GetClusterName()!=src.GetClusterName() {
		labels["cluster"] = fmt.Sprintf("%s", src.GetCluster().GetId())
	}

	ip, cname := src.GetTargets(logger, lb)
	n := this.UpdateDeadline(logger, lb.Data().(*api.DNSLoadBalancer).Spec.EndpointValidityInterval, nil)
	r:=&api.DNSLoadBalancerEndpoint{}
	return &api.DNSLoadBalancerEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    src.GetName()+"-"+src.GroupKind().Kind+"-",
			Namespace:       lb.GetNamespace(),
		},
		Spec: api.DNSLoadBalancerEndpointSpec{
			IPAddress:    ip,
			CName:        cname,
			LoadBalancer: lb.GetName(),
		},
		Status: api.DNSLoadBalancerEndpointStatus{
			ValidUntil: n,
		},
	}
}


func (this *reconciler) UpdateDeadline(logger logger.LogContext, duration *metav1.Duration, deadline *metav1.Time) *metav1.Time {
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
