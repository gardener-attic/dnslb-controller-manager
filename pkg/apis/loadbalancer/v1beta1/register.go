// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"github.com/gardener/controller-manager-library/pkg/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer"
)

const (
	Version   = "v1beta1"
	GroupName = loadbalancer.GroupName

	LoadBalancerResourceKind   = "DNSLoadBalancer"
	LoadBalancerResourcePlural = "dnsloadbalancers"

	LoadBalancerEndpointResourceKind   = "DNSLoadBalancerEndpoint"
	LoadBalancerEndpointResourcePlural = "dnsloadbalancerendpoints"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme

	SchemeGroupVersion          = schema.GroupVersion{Group: loadbalancer.GroupName, Version: Version}
	LoadBalancerCRDName         = LoadBalancerResourcePlural + "." + loadbalancer.GroupName
	LoadBalancerEndpointCRDName = LoadBalancerEndpointResourcePlural + "." + loadbalancer.GroupName
)

var (
	LoadBalancerGroupKind         = schema.GroupKind{GroupName, LoadBalancerResourceKind}
	LoadBalancerEndpointGroupKind = schema.GroupKind{GroupName, LoadBalancerEndpointResourceKind}
)

// Resource gets an LoadBalancer GroupResource for a specified resource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&DNSLoadBalancer{},
		&DNSLoadBalancerList{},
		&DNSLoadBalancerEndpoint{},
		&DNSLoadBalancerEndpointList{},
	)
	metav1.AddToGroupVersion(s, SchemeGroupVersion)
	return nil
}

func init() {

	resources.Register(SchemeBuilder)
}
