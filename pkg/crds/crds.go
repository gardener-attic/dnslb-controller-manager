// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crds

import (
	"github.com/gardener/controller-manager-library/pkg/resources/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
)

var DNSLBCRD = apiextensions.CreateCRDObject(api.GroupName, api.Version, api.LoadBalancerResourceKind, api.LoadBalancerResourcePlural, "dnlb", true,
	v1beta1.CustomResourceColumnDefinition{
		Name:        "DNSNAME",
		Description: "DNS Name of loadbalancer",
		Type:        "string",
		JSONPath:    ".spec.dnsname",
	},
	v1beta1.CustomResourceColumnDefinition{
		Name:        "TYPE",
		Description: "Type of loadbalancer",
		Type:        "string",
		JSONPath:    ".spec.type",
	},
	v1beta1.CustomResourceColumnDefinition{
		Name:        "STATUS",
		Description: "loadbalancer state",
		Type:        "string",
		JSONPath:    ".status.state",
	})

var DNSLBEPCRD = apiextensions.CreateCRDObject(api.GroupName, api.Version, api.LoadBalancerEndpointResourceKind, api.LoadBalancerEndpointResourcePlural, "dnslbep", true,
	v1beta1.CustomResourceColumnDefinition{
		Name:        "DNSLB",
		Description: "Loadbalancer",
		Type:        "string",
		JSONPath:    ".spec.loadbalancer",
	},
	v1beta1.CustomResourceColumnDefinition{
		Name:        "HEALTHY",
		Description: "Health status of endpoint",
		Type:        "boolean",
		JSONPath:    ".status.healthy",
	},
	v1beta1.CustomResourceColumnDefinition{
		Name:        "STATUS",
		Description: "Assigned to Loadbalancer",
		Type:        "string",
		JSONPath:    ".status.state",
	},
)
