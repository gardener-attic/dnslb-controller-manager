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

package v1beta1

import (
	v1beta1 "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	scheme "github.com/gardener/dnslb-controller-manager/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// DNSProvidersGetter has a method to return a DNSProviderInterface.
// A group's client should implement this interface.
type DNSProvidersGetter interface {
	DNSProviders(namespace string) DNSProviderInterface
}

// DNSProviderInterface has methods to work with DNSProvider resources.
type DNSProviderInterface interface {
	Create(*v1beta1.DNSProvider) (*v1beta1.DNSProvider, error)
	Update(*v1beta1.DNSProvider) (*v1beta1.DNSProvider, error)
	UpdateStatus(*v1beta1.DNSProvider) (*v1beta1.DNSProvider, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.DNSProvider, error)
	List(opts v1.ListOptions) (*v1beta1.DNSProviderList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.DNSProvider, err error)
	DNSProviderExpansion
}

// dNSProviders implements DNSProviderInterface
type dNSProviders struct {
	client rest.Interface
	ns     string
}

// newDNSProviders returns a DNSProviders
func newDNSProviders(c *LoadbalancerV1beta1Client, namespace string) *dNSProviders {
	return &dNSProviders{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the dNSProvider, and returns the corresponding dNSProvider object, and an error if there is any.
func (c *dNSProviders) Get(name string, options v1.GetOptions) (result *v1beta1.DNSProvider, err error) {
	result = &v1beta1.DNSProvider{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("dnsproviders").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DNSProviders that match those selectors.
func (c *dNSProviders) List(opts v1.ListOptions) (result *v1beta1.DNSProviderList, err error) {
	result = &v1beta1.DNSProviderList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("dnsproviders").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested dNSProviders.
func (c *dNSProviders) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("dnsproviders").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a dNSProvider and creates it.  Returns the server's representation of the dNSProvider, and an error, if there is any.
func (c *dNSProviders) Create(dNSProvider *v1beta1.DNSProvider) (result *v1beta1.DNSProvider, err error) {
	result = &v1beta1.DNSProvider{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("dnsproviders").
		Body(dNSProvider).
		Do().
		Into(result)
	return
}

// Update takes the representation of a dNSProvider and updates it. Returns the server's representation of the dNSProvider, and an error, if there is any.
func (c *dNSProviders) Update(dNSProvider *v1beta1.DNSProvider) (result *v1beta1.DNSProvider, err error) {
	result = &v1beta1.DNSProvider{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("dnsproviders").
		Name(dNSProvider.Name).
		Body(dNSProvider).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *dNSProviders) UpdateStatus(dNSProvider *v1beta1.DNSProvider) (result *v1beta1.DNSProvider, err error) {
	result = &v1beta1.DNSProvider{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("dnsproviders").
		Name(dNSProvider.Name).
		SubResource("status").
		Body(dNSProvider).
		Do().
		Into(result)
	return
}

// Delete takes name of the dNSProvider and deletes it. Returns an error if one occurs.
func (c *dNSProviders) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("dnsproviders").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *dNSProviders) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("dnsproviders").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched dNSProvider.
func (c *dNSProviders) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.DNSProvider, err error) {
	result = &v1beta1.DNSProvider{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("dnsproviders").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
