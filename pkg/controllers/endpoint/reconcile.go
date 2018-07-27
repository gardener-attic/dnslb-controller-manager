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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/endpoint/source"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/endpoint/util"
)

// Operation Contract:
// true,  nil: valid source, everything ok, just do reconcile
// true,  err: valid source, but required state not reached, redo rate limited
// false, err: invalid source by configuration
// false, nil: operation temporarily failed, just redo

func (this *Worker) handleReconcile(src source.Source) (bool, error) {
	ref, _ := GetLoadBalancerRef(src)
	this.Debugf("HANDLE reconcile %s for %s", source.Desc(src), ref)
	if ref != nil {
		exist := HasFinalizer(src)
		lb, normal, err := this.validate(ref, src)
		if err != nil {
			if exist {
				this.deleteOldEndpoints(src)
			}
			return normal, err
		}
		if !exist {
			this.Infof("set finalizer for %s", source.Desc(src))
			SetFinalizer(src)
			err = this.controller.UpdateSource(src)
			if err != nil {
				return true, err
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

func (this *Worker) cleanupFinalizer(src source.Source) (bool, error) {
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

func (this *Worker) validate(ref ObjectRef, src source.Source) (*lbapi.DNSLoadBalancer, bool, error) {
	lb, err := this.controller.lbLister.DNSLoadBalancers(ref.GetNamespace()).Get(ref.GetName())
	if lb == nil || err != nil {
		if errors.IsNotFound(err) {
			this.deleteOldEndpoints(src)
			return nil, true, fmt.Errorf("dns loadbalancer '%s' does not exist", ref)
		} else {
			return nil, false, fmt.Errorf("cannot get dns loadbalancer '%s': %s", ref, err)
		}
	}
	if normal, err := src.Validate(lb); err != nil {
		return nil, normal, err
	}
	return lb, true, nil
}

func (this *Worker) handleDelete(src source.Source) (bool, error) {
	ref, _ := GetLoadBalancerRef(src)
	this.Debugf("HANDLE delete service  %s/%s for %s", src.GetNamespace(), src.GetName(), ref)
	err := this.deleteOldEndpoints(src)
	if err != nil {
		return true, err
	}
	return this.cleanupFinalizer(src)
}

func (this *Worker) deleteOldEndpoints(src source.Source, exclude ...*lbapi.DNSLoadBalancerEndpoint) error {
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

func (this *Worker) deleteEndpoint(src source.Source, namespace, name string) error {
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
