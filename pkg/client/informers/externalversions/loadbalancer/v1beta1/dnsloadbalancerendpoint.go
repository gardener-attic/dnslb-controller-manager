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
// Code generated by informer-gen. DO NOT EDIT.

package v1beta1

import (
	time "time"

	loadbalancer_v1beta1 "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	versioned "github.com/gardener/dnslb-controller-manager/pkg/client/clientset/versioned"
	internalinterfaces "github.com/gardener/dnslb-controller-manager/pkg/client/informers/externalversions/internalinterfaces"
	v1beta1 "github.com/gardener/dnslb-controller-manager/pkg/client/listers/loadbalancer/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// DNSLoadBalancerEndpointInformer provides access to a shared informer and lister for
// DNSLoadBalancerEndpoints.
type DNSLoadBalancerEndpointInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.DNSLoadBalancerEndpointLister
}

type dNSLoadBalancerEndpointInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewDNSLoadBalancerEndpointInformer constructs a new informer for DNSLoadBalancerEndpoint type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewDNSLoadBalancerEndpointInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredDNSLoadBalancerEndpointInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredDNSLoadBalancerEndpointInformer constructs a new informer for DNSLoadBalancerEndpoint type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredDNSLoadBalancerEndpointInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LoadbalancerV1beta1().DNSLoadBalancerEndpoints(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LoadbalancerV1beta1().DNSLoadBalancerEndpoints(namespace).Watch(options)
			},
		},
		&loadbalancer_v1beta1.DNSLoadBalancerEndpoint{},
		resyncPeriod,
		indexers,
	)
}

func (f *dNSLoadBalancerEndpointInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredDNSLoadBalancerEndpointInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *dNSLoadBalancerEndpointInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&loadbalancer_v1beta1.DNSLoadBalancerEndpoint{}, f.defaultInformer)
}

func (f *dNSLoadBalancerEndpointInformer) Lister() v1beta1.DNSLoadBalancerEndpointLister {
	return v1beta1.NewDNSLoadBalancerEndpointLister(f.Informer().GetIndexer())
}
