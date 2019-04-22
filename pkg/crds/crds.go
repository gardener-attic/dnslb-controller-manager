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

package crds

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"github.com/gardener/controller-manager-library/pkg/resources/apiextensions"

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

