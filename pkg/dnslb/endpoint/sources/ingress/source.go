// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ingress

import (
	"fmt"
	"github.com/gardener/controller-manager-library/pkg/resources"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"
	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Source struct {
	*resources.IngressObject
}

type SourceType struct {
	schema.GroupKind
}

var _ sources.Source = &Source{}

func init() {
	sources.Register(&SourceType{resources.NewGroupKind(api.GroupName, "Ingress")})
}

func (this *SourceType) GetGroupKind() schema.GroupKind {
	return this.GroupKind
}

func (this *SourceType) Get(obj resources.Object) (sources.Source, error) {
	if obj.GroupKind() != this.GroupKind {
		return nil, fmt.Errorf("invalid object type %q", obj.GroupKind())
	}
	return &Source{resources.Ingress(obj)}, nil
}

func (this *Source) GetTargets(lb resources.Object) (ip, cname string) {
	data := this.Ingress()
	target := utils.DNSLoadBalancer(lb).DNSLoadBalancer()
	for _, l := range data.Status.LoadBalancer.Ingress {
		if l.IP != "" {
			ip = l.IP
		}
		if l.Hostname != "" {
			cname = l.Hostname
		}
	}
	if cname == "" && ip == "" {
		for _, i := range data.Spec.Rules {
			if i.Host != "" && i.Host != target.Spec.DNSName {
				cname = i.Host
				return
			}
		}
	}
	return
}

func (this *Source) Validate(lb resources.Object) (bool, error) {
	data := this.Ingress()
	target := utils.DNSLoadBalancer(lb).DNSLoadBalancer()
	dns := false
	for _, i := range data.Spec.Rules {
		if i.Host != "" {
			if i.Host == target.Spec.DNSName {
				dns = true
			}
		}
	}
	if !dns {
		return false, fmt.Errorf("load balancer host '%s' not configured as host rule for '%s'", target.Spec.DNSName, this.ObjectName())
	}
	cname, ip := this.GetTargets(lb)
	if cname == "" && ip == "" {
		return false, fmt.Errorf("no host rule or loadbalancer status defined for '%s'", this.ObjectName())
	}
	return true, nil
}
