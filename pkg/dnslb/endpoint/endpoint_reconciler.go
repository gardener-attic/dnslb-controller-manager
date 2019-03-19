package endpoint

import (
	"fmt"
	"strings"
	"time"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"

	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/reconcile/reconcilers"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/resources"

	dnsutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type source_reconciler struct {
	*reconcilers.SlaveAccess
	usages       *reconcilers.UsageAccess
	lb_resource  resources.Interface
	ep_resource  resources.Interface
	sourceUsages *utils.SharedUsages
}

func SourceReconciler(c controller.Interface) (reconcile.Interface, error) {
	target := c.GetCluster(TARGET_CLUSTER)

	lb, err := target.GetResource(resources.NewGroupKind(api.GroupName, api.LoadBalancerResourceKind))
	if err != nil {
		return nil, err
	}
	ep, err := target.GetResource(resources.NewGroupKind(api.GroupName, api.LoadBalancerEndpointResourceKind))
	if err != nil {
		return nil, err
	}

	return &source_reconciler{
		SlaveAccess: reconcilers.NewSlaveAccess(c, "endpoint", SlaveResources, MasterResources),
		usages:      reconcilers.NewUsageAccess(c, LBUSAGES, MasterResources, LBFunc(c)),
		lb_resource: lb,
		ep_resource: ep,
	}, nil
}

func (this *source_reconciler) Setup() {
	this.SlaveAccess.Setup()
	this.usages.Setup()
}

func (this *source_reconciler) Reconcile(logger logger.LogContext, obj resources.Object) reconcile.Status {
	ep := this.AssertSingleSlave(logger, obj.ClusterKey(), this.LookupSlaves(obj.ClusterKey()), nil)
	this.usages.RenewOwner(obj)
	ref, src := this.IsValid(obj)
	if ref != nil {
		logger.Debugf("HANDLE reconcile %s for %s", obj.ObjectName(), ref)
		lb, result := this.validate(logger, ref, src)
		if !result.IsSucceeded() {
			this.deleteEndpoint(logger, src, ep)
			return result
		}
		err := this.SetFinalizer(obj)
		if err != nil {
			return reconcile.Delay(logger, err)
		}
		newep := this.newEndpoint(logger, lb, src)
		if ep == nil {
			logger.Infof("endpoint for loadbalancer %s not found -> create it", ref)
			err := this.CreateSlave(src, newep)
			if err != nil {
				return reconcile.Delay(logger, fmt.Errorf("error creating load balancer endpoint: %s", err))
			}
			logger.Infof("dns load balancer endpoint %s created for %s", newep.ObjectName(), ref)
			src.Eventf(corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s created", newep.ObjectName())
			return reconcile.Succeeded(logger).RescheduleAfter(60 * time.Second)
		}
		mod := this.updateEndpoint(logger, ep, newep, lb, src)
		if mod.Modified {
			logger.Infof("endpoint found, but requires update")
			err := this.UpdateSlave(mod.Object())
			if err != nil {
				if errors.IsConflict(err) {
					return reconcile.Repeat(logger, fmt.Errorf("conflict updating load balancer endpoint '%s': %s", ep.ObjectName(), err))
				}
				return reconcile.Delay(logger, fmt.Errorf("error updating load balancer endpoint '%s': %s", ep.ObjectName(), err))
			}
			src.Eventf(corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s updated", ep.ObjectName())
		} else {
			logger.Debugf("endpoint up to date")
		}
		return reconcile.Succeeded(logger).RescheduleAfter(60 * time.Second)
	} else {
		err := this.deleteEndpoint(logger, obj, ep)
		if err != nil {
			return reconcile.Delay(logger, err)
		}
		return reconcile.DelayOnError(logger, this.RemoveFinalizer(obj))
	}
	return reconcile.Succeeded(logger)
}

func LBForSource(obj resources.Object) (resources.ObjectName, bool) {
	for n, v := range obj.GetAnnotations() {
		if n == AnnotationLoadbalancer {
			parts := strings.Split(v, "/")
			switch len(parts) {
			case 1:
				return resources.NewObjectName(obj.GetNamespace(), parts[0]), true
			case 2:
				return resources.NewObjectName(parts[0], parts[1]), true
			default:
				return nil, true
			}
		}
	}
	return nil, false
}

func (this *source_reconciler) IsValid(obj resources.Object) (resources.ObjectName, sources.Source) {
	t := sources.SourceTypes[obj.GroupKind()]
	if t == nil {
		return nil, nil
	}
	src, _ := t.Get(obj)
	n, found := LBForSource(obj)
	if n == nil {
		if found && this.HasFinalizer(obj) {
			return nil, src
		}
		return nil, nil
	}
	return n, src
}

func (this *source_reconciler) validate(logger logger.LogContext, ref resources.ObjectName, src sources.Source) (*dnsutils.DNSLoadBalancerObject, reconcile.Status) {
	lb, err := this.lb_resource.GetCached(ref)
	if lb == nil || err != nil {
		if errors.IsNotFound(err) {
			src.Eventf(corev1.EventTypeNormal, AnnotationLoadbalancer, fmt.Sprintf("dns loadbalancer '%s' does not exist", ref))
			return nil, reconcile.Failed(logger, fmt.Errorf("dns loadbalancer '%s' does not exist", ref)).RescheduleAfter(10 * time.Minute)
		} else {
			src.Eventf(corev1.EventTypeNormal, AnnotationLoadbalancer, fmt.Sprintf("cannot get dns loadbalancer '%s': %s", ref, err))
			return nil, reconcile.Delay(logger, fmt.Errorf("cannot get dns loadbalancer '%s': %s", ref, err))
		}
	}
	if normal, err := src.Validate(lb); err != nil {
		if normal {
			return nil, reconcile.Delay(logger, err)
		}
		return nil, reconcile.Failed(logger, err)
	}
	return dnsutils.DNSLoadBalancer(lb), reconcile.Succeeded(logger)
}

func (this *source_reconciler) Delete(logger logger.LogContext, obj resources.Object) reconcile.Status {
	this.sourceUsages.RemoveValue(obj.ClusterKey())
	ref, src := this.IsValid(obj)
	failed := false
	if src != nil {
		logger.Debugf("HANDLE delete source  %s for %s", src.ObjectName(), ref)
		for _, ep := range this.LookupSlaves(obj.ClusterKey()) {
			err := this.deleteEndpoint(logger, src, ep)
			if err != nil {
				logger.Warn(err)
				failed = true
			}
		}
		if failed {
			return reconcile.Delay(logger, fmt.Errorf("some endpoint deletion failed"))
		}
		return reconcile.DelayOnError(logger, this.RemoveFinalizer(obj))
	}
	this.usages.DeleteOwner(obj.ClusterKey())
	return reconcile.Succeeded(logger)
}

func (this *source_reconciler) Deleted(logger logger.LogContext, key resources.ClusterObjectKey) reconcile.Status {
	this.usages.DeleteOwner(key)
	return reconcile.Succeeded(logger)
}
