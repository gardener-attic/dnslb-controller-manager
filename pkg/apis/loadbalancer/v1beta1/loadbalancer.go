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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSLoadBalancer `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSLoadBalancerSpec   `json:"spec"`
	Status            DNSLoadBalancerStatus `json:"status"`
}

type DNSLoadBalancerSpec struct {
	DNSName                  string           `json:"DNSName"`
	HealthPath               string           `json:"healthPath"`
	StatusCode               int              `json:"statusCode,omitempty"`
	Type                     string           `json:"type,omitempty"`
	Singleton                *bool            `json:"singleton,omitempty"`
	EndpointValidityInterval *metav1.Duration `json:"endpointValidityInterval,omitempty"`
}

const (
	LBTYPE_BALANCED  = "Balanced"  // all active endpoints are selected
	LBTYPE_EXCLUSIVE = "Exclusive" // singleton DNS entry (one active endpoint is selected)
)

type DNSLoadBalancerStatus struct {
	State   string                  `json:"state"`
	Message string                  `json:"message,omitempty"`
	Active  []DNSLoadBalancerActive `json:"active,omitempty"`
}

type DNSLoadBalancerActive struct {
	Name      string `json:"name"`
	IPAddress string `json:"ipaddress,omitempty"`
	CName     string `json:"cname,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSLoadBalancerEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSLoadBalancerEndpoint `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSLoadBalancerEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSLoadBalancerEndpointSpec   `json:"spec"`
	Status            DNSLoadBalancerEndpointStatus `json:"status"`
}

type DNSLoadBalancerEndpointSpec struct {
	LoadBalancer string `json:"loadbalancer"`
	IPAddress    string `json:"ipaddress,omitempty"`
	CName        string `json:"cname,omitempty"`
}

type DNSLoadBalancerEndpointStatus struct {
	Active     bool         `json:"active"`
	Healthy    bool         `json:"healthy"`
	ValidUntil *metav1.Time `json:"validUntil,omitempty"`
}
