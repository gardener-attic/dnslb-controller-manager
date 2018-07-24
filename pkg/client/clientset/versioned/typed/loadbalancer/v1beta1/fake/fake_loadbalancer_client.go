/*
Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1beta1 "github.com/gardener/dnslb-controller-manager/pkg/client/clientset/versioned/typed/loadbalancer/v1beta1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeLoadbalancerV1beta1 struct {
	*testing.Fake
}

func (c *FakeLoadbalancerV1beta1) DNSLoadBalancers(namespace string) v1beta1.DNSLoadBalancerInterface {
	return &FakeDNSLoadBalancers{c, namespace}
}

func (c *FakeLoadbalancerV1beta1) DNSLoadBalancerEndpoints(namespace string) v1beta1.DNSLoadBalancerEndpointInterface {
	return &FakeDNSLoadBalancerEndpoints{c, namespace}
}

func (c *FakeLoadbalancerV1beta1) DNSProviders(namespace string) v1beta1.DNSProviderInterface {
	return &FakeDNSProviders{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeLoadbalancerV1beta1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
