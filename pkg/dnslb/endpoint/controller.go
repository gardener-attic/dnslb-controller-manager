// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package endpoint

import (
	"fmt"
	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/controllermanager/cluster"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/endpoint/source"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/endpoint/util"
)

const AnnotationLoadbalancer = api.GroupName + "/dnsloadbalancer"

func init() {
	controller.Configure("dnslb-endpoint").
		MainResource("core","Service").
		FinalizerDomain(api.GroupName).
		Reconciler(Reconciler).MustRegister("source")
}

func Reconciler(c controller.Interface) (reconcile.Interface, error) {
	lb, err:=c.GetDefaultCluster().GetResource(resources.NewGroupKind(api.GroupName, api.LoadBalancerResourceKind))
	if err != nil {
	    return nil,err
	}
	ep, err:=c.GetDefaultCluster().GetResource(resources.NewGroupKind(api.GroupName, api.LoadBalancerEndpointResourceKind))
	if err != nil {
		return nil,err
	}
	return &reconciler{
		controller: c,
		lb_resource: lb,
		ep_resource: ep,
	}, nil
}

type reconciler struct {
	reconcile.DefaultReconciler
	controller controller.Interface
	lb_resource resources.Interface
	ep_resource resources.Interface
}


func (this *reconciler) Reconcile(logger logger.LogContext, obj resources.Object) reconcile.Status {
	ref:=this.IsValid()
	if ref != nil {
		logger.Debugf("HANDLE reconcile %s for %s", obj.ObjectName(), ref)
		exist := this.controller.HasFinalizer(obj)
		lb, result := this.validate(ref, src)
		if !result.IsSucceeded() {
			if exist {
				this.deleteOldEndpoints(src)
			}
			return result
		}
		if !exist {
			err:=this.controller.SetFinalizer(obj)
			if err != nil {
				return reconcile.Delay(logger,err)
			}
		}
		newep := newEndpoint(lb, src, this.controller.clusterid)
		ep, err := this.controller.epLister.DNSLoadBalancerEndpoints(newep.GetNamespace()).Get(newep.GetName())
		if err != nil {
			if errors.IsNotFound(err) {
				this.deleteOldEndpoints(src)
				this.Infof("endpoint not found -> create it")
				ep, err = this.controller.CreateEndpoint(newep)
				if err != nil {
					return true, fmt.Errorf("error creating load balancer endpoint '%s': %s", Ref(newep), err)
				}
				this.controller.Eventf(src, corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s created", Ref(ep))
				return true, nil
			} else {
				return true, fmt.Errorf("error getting load balancer endpoint '%s': %s", Ref(newep), err)
			}
		}
		ep, update := updateEndpoint(ep, newep, lb, this.LogCtx)
		if update {
			this.deleteOldEndpoints(src, ep)
			this.Infof("endpoint found, but requires update")
			ep, err = this.controller.UpdateEndpoint(ep)
			if err != nil {
				if errors.IsConflict(err) {
					this.Warnf("conflict updating load balancer endpoint '%s': %s", Ref(ep), err)
					return false, nil
				}
				return true, fmt.Errorf("error updating load balancer endpoint '%s': %s", Ref(ep), err)
			}
			this.controller.Eventf(src, corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s updated", Ref(ep))
		} else {
			this.Debugf("endpoint up to date")
		}
	} else {
		if HasFinalizer(src) {
			this.controller.Eventf(src, corev1.EventTypeNormal, "sync", "dns load balancer annotation removed")
			this.Infof("load balancer annotation removed -> cleanup")
		}
		err := this.deleteOldEndpoints(src)
		if err != nil {
			return true, err
		}
		return this.cleanupFinalizer(src)
	}
	return true, nil
}

func (this *reconciler) IsValid(obj resources.Object) resources.ObjectName {
	for n, v := range obj.GetAnnotations() {
		if n == AnnotationLoadbalancer {
			parts := strings.Split(v, "/")
			switch len(parts) {
			case 1:
				return resources.NewObjectName(obj.GetNamespace(), parts[0])
			case 2:
				return resources.NewObjectName(parts[0], parts[1])
			default:
				return nil
			}
		}
	}
	return nil
}

func (this *reconciler) validate(logger logger.LogContext, ref resources.ObjectName, src resources.Object) (resources.Object, reconcile.Status) {
	lb, err := this.lb_resource.GetCached(ref)
	if lb == nil || err != nil {
		if errors.IsNotFound(err) {
			this.deleteOldEndpoints(src)
			return nil, reconcile.Failed(logger,fmt.Errorf("dns loadbalancer '%s' does not exist", ref))
		} else {
			return nil, reconcile.Delay(logger, fmt.Errorf("cannot get dns loadbalancer '%s': %s", ref, err))
		}
	}
	if normal, err := src.Validate(lb); err != nil {
		if normal {
			return nil, reconcile.Delay(logger,err)
		}
		return nil, reconcile.Failed(logger, err)
	}
	return lb, reconcile.Succeeded(logger)
}

func (this *reconciler) cleanupFinalizer(src source.Source) (bool, error) {
	if HasFinalizer(src) {
		this.Infof("removing finalizer for %s", source.Desc(src))
		RemoveFinalizer(src)
		err := this.controller.UpdateSource(src)
		if err != nil {
			if errors.IsConflict(err) {
				this.Warnf("conflict updating %s: %s", source.Desc(src), err)
				return false, nil
			}
			return true, err
		}
	}
	return true, nil
}

func (this *reconciler) handleDelete(src source.Source) (bool, error) {
	ref, _ := GetLoadBalancerRef(src)
	this.Debugf("HANDLE delete service  %s/%s for %s", src.GetNamespace(), src.GetName(), ref)
	err := this.deleteOldEndpoints(src)
	if err != nil {
		return true, err
	}
	return this.cleanupFinalizer(src)
}

func (this *reconciler) deleteOldEndpoints(src resources.Object, exclude ...*api.DNSLoadBalancerEndpoint) error {
	eplist, err := this.controller.epLister.DNSLoadBalancerEndpoints("").List(labels.Everything())
	if err != nil {
		return err
	}
	found := false
main:
	for _, ep := range eplist {
		ref := NewObjectRef(ep.GetNamespace(), ep.Spec.LoadBalancer)
		tgt := EndpointRef(ref, src, this.controller.clusterid)
		if RefEquals(ep, tgt) {
			for _, ex := range exclude {
				if RefEquals(ep, ex) {
					continue main
				}
			}
			if !found {
				found = true
				this.Infof("deleting current/old dns load balancer endpoints for %s", Ref(src))
			}
			suberr := this.deleteEndpoint(src, ep.GetNamespace(), ep.GetName())
			if suberr != nil {
				err = suberr
			}
		}
	}

	if err != nil {
		return err
	}
	return err
}

func (this *reconciler) deleteEndpoint(src source.Source, namespace, name string) error {
	err := this.controller.DeleteEndpoint(namespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error deleting load balancer endpoint '%s/%s': %s", namespace, name, err)
		}
	}
	this.controller.Eventf(src, corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s/%s deleted", namespace, name)
	this.Infof("dns load balancer endpoint %s/%s deleted", namespace, name)

	return nil
}
