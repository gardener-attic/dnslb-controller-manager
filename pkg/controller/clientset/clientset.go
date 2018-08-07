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

package clientset

import (
	loadbalancerv1beta1 "github.com/gardener/dnslb-controller-manager/pkg/client/clientset/versioned/typed/loadbalancer/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	kubernetes "k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	flowcontrol "k8s.io/client-go/util/flowcontrol"
)

type Interface interface {
	GetConfig() *rest.Config

	kubernetes.Interface

	ApiextensionsV1beta1() apiextensionsv1beta1.ApiextensionsV1beta1Interface
	// Deprecated: please explicitly pick a version if possible.
	Apiextensions() apiextensionsv1beta1.ApiextensionsV1beta1Interface

	LoadbalancerV1beta1() loadbalancerv1beta1.LoadbalancerV1beta1Interface
	// Deprecated: please explicitly pick a version if possible.
	Loadbalancer() loadbalancerv1beta1.LoadbalancerV1beta1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	kubernetes.Interface
	apiextensionsV1beta1 *apiextensionsv1beta1.ApiextensionsV1beta1Client
	loadbalancerV1beta1  *loadbalancerv1beta1.LoadbalancerV1beta1Client

	config *rest.Config
}

// GetConfig return the config object used fto create this clientset prior to
// specialization. THis is inteded to create other kinds of clientsets based
// on this clientset.
func (c *Clientset) GetConfig() *rest.Config {
	return c.config
}

// ApiextensionsV1beta1 retrieves the ApiextensionsV1beta1Client
func (c *Clientset) ApiextensionsV1beta1() apiextensionsv1beta1.ApiextensionsV1beta1Interface {
	return c.apiextensionsV1beta1
}

// Deprecated: Apiextensions retrieves the default version of ApiextensionsClient.
// Please explicitly pick a version.

func (c *Clientset) Apiextensions() apiextensionsv1beta1.ApiextensionsV1beta1Interface {
	return c.apiextensionsV1beta1
}

// LoadbalancerV1beta1 retrieves the LoadbalancerV1beta1Client
func (c *Clientset) LoadbalancerV1beta1() loadbalancerv1beta1.LoadbalancerV1beta1Interface {
	return c.loadbalancerV1beta1
}

// Deprecated: Loadbalancer retrieves the default version of LoadbalancerClient.
// Please explicitly pick a version.

func (c *Clientset) Loadbalancer() loadbalancerv1beta1.LoadbalancerV1beta1Interface {
	return c.loadbalancerV1beta1
}

// NewForConfig creates a new Clientset for the given config.
func NewForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var cs Clientset
	var err error

	cs.config = c

	cs.Interface, err = kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	cs.apiextensionsV1beta1, err = apiextensionsv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	cs.loadbalancerV1beta1, err = loadbalancerv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	return &cs, nil
}
