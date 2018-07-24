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

// DNSProviderInformer provides access to a shared informer and lister for
// DNSProviders.
type DNSProviderInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.DNSProviderLister
}

type dNSProviderInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewDNSProviderInformer constructs a new informer for DNSProvider type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewDNSProviderInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredDNSProviderInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredDNSProviderInformer constructs a new informer for DNSProvider type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredDNSProviderInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LoadbalancerV1beta1().DNSProviders(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LoadbalancerV1beta1().DNSProviders(namespace).Watch(options)
			},
		},
		&loadbalancer_v1beta1.DNSProvider{},
		resyncPeriod,
		indexers,
	)
}

func (f *dNSProviderInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredDNSProviderInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *dNSProviderInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&loadbalancer_v1beta1.DNSProvider{}, f.defaultInformer)
}

func (f *dNSProviderInformer) Lister() v1beta1.DNSProviderLister {
	return v1beta1.NewDNSProviderLister(f.Informer().GetIndexer())
}
