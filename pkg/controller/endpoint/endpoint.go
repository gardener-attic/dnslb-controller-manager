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
	"strings"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/endpoint/source"
	. "github.com/gardener/dnslb-controller-manager/pkg/controller/endpoint/util"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func EndpointRef(lb ObjectRef, src source.Source, srcid string) ObjectRef {
	kind := strings.ToLower(src.GetKind())
	ns := ""
	//src.GetTypeMeta()
	if lb.GetNamespace() != src.GetNamespace() {
		ns = fmt.Sprintf("%s-", src.GetNamespace())
	}
	if srcid != "" {
		return NewObjectRef(lb.GetNamespace(), fmt.Sprintf("%s-%s%s-%s", srcid, ns, src.GetName(), kind))
	}
	return NewObjectRef(lb.GetNamespace(), fmt.Sprintf("%s%s-%s", ns, src.GetName(), kind))
}

func updateEndpoint(ep, newep *lbapi.DNSLoadBalancerEndpoint, lb *lbapi.DNSLoadBalancer, log log.LogCtx) (*lbapi.DNSLoadBalancerEndpoint, bool) {
	ok := true
	ep = ep.DeepCopy()
	ok = ok && AssureLabel(ep, "controller", GetLabel(newep, "controller"))
	ok = ok && AssureLabel(ep, "source", GetLabel(newep, "source"))

	ok = ok && AssureString(&ep.Spec.IPAddress, newep.Spec.IPAddress)
	ok = ok && AssureString(&ep.Spec.CName, newep.Spec.CName)
	ok = ok && AssureString(&ep.Spec.LoadBalancer, newep.Spec.LoadBalancer)

	n := UpdateDeadline(lb.Spec.EndpointValidityInterval, ep.Status.ValidUntil, log)
	if n != ep.Status.ValidUntil {
		ep.Status.ValidUntil = n
		ok = false
	}
	return ep, !ok
}

func newEndpoint(lb *lbapi.DNSLoadBalancer, src source.Source, srcid string) *lbapi.DNSLoadBalancerEndpoint {
	labels := map[string]string{
		"controller": Finalizer,
		"source":     fmt.Sprintf("%s:%s", src.GetKind(), Ref(src)),
	}
	if srcid != "" {
		labels["cluster"] = fmt.Sprintf("%s", srcid)
	}

	owners := []metav1.OwnerReference{}
	if srcid != "" {
		labels["cluster"] = srcid
	} else {
		owners = []metav1.OwnerReference{
			*metav1.NewControllerRef(src, schema.GroupVersionKind{
				Group:   corev1.SchemeGroupVersion.Group,
				Version: corev1.SchemeGroupVersion.Version,
				Kind:    src.GetKind(),
			}),
		}
	}
	r := EndpointRef(lb, src, srcid)
	ip, cname := src.GetEndpoint(lb)
	n := UpdateDeadline(lb.Spec.EndpointValidityInterval, nil, nil)
	return &lbapi.DNSLoadBalancerEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:            r.GetName(),
			Namespace:       r.GetNamespace(),
			OwnerReferences: owners,
		},
		Spec: lbapi.DNSLoadBalancerEndpointSpec{
			IPAddress:    ip,
			CName:        cname,
			LoadBalancer: lb.GetName(),
		},
		Status: lbapi.DNSLoadBalancerEndpointStatus{
			ValidUntil: n,
		},
	}
}
