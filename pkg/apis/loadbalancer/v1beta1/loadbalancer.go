// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	DNSName                  string           `json:"dnsname"`
	HealthPath               string           `json:"healthPath"`
	StatusCode               int              `json:"statusCode,omitempty"`
	Type                     string           `json:"type,omitempty"`
	TTL                      *int64           `json:"ttl,omitempty"`
	Singleton                *bool            `json:"singleton,omitempty"`
	EndpointValidityInterval *metav1.Duration `json:"endpointValidityInterval,omitempty"`
}

const (
	LBTYPE_BALANCED  = "Balanced"  // all active endpoints are selected
	LBTYPE_EXCLUSIVE = "Exclusive" // singleton dnsname entry (one active endpoint is selected)
)

type DNSLoadBalancerStatus struct {
	State   *string                 `json:"state,omitempty"`
	Message *string                 `json:"message,omitempty"`
	Active  []DNSLoadBalancerActive `json:"active,omitempty"`
}

type DNSLoadBalancerActive struct {
	Endpoint  string `json:"endpoint"`
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
	State      *string      `json:"state,omitempty"`
	Message    *string      `json:"message,omitempty"`
	Healthy    bool         `json:"healthy"`
	ValidUntil *metav1.Time `json:"validUntil,omitempty"`
}
