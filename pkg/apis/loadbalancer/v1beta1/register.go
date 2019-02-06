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
