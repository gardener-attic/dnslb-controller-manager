package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SCOPE_CLUSTER   = "Cluster"   // all name spaces
	SCOPE_NAMESPACE = "Namespace" // local namespace, only
	SCOPE_SELECTED  = "Selected"  // namespaces selected by namespaces field
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSProviderList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSProvider `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSProviderSpec   `json:"spec"`
	Status            DNSProviderStatus `json:"status"`
}

type DNSProviderSpec struct {
	Type      string                  `json:"type,omitempty"`
	Scope     *Scope                  `json:"scope,omitempty"`
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}

type DNSProviderStatus struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

type Scope struct {
	Type       string   `json:"type"`
	Namespaces []string `json:"namespaces,omitempty"`
}
