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

package config

import (
	"fmt"
	"time"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func RegisterCrds(clientset clientset.Interface) error {
	err := CreateCRD(clientset, api.LoadBalancerCRDName, api.LoadBalancerResourceKind, api.LoadBalancerResourcePlural, "dnslb")
	if err != nil {
		return fmt.Errorf("failed to create CRD: %v", err)
	}
	err = CreateCRD(clientset, api.LoadBalancerEndpointCRDName, api.LoadBalancerEndpointResourceKind, api.LoadBalancerEndpointResourcePlural, "dnslbep")
	if err != nil {
		return fmt.Errorf("failed to create CRD: %v", err)
	}
	err = CreateCRD(clientset, api.DNSProviderCRDName, api.DNSProviderResourceKind, api.DNSProviderResourcePlural, "dnsprov")
	if err != nil {
		return fmt.Errorf("failed to create CRD: %v", err)
	}
	err = WaitCRDReady(clientset, api.LoadBalancerCRDName)
	if err != nil {
		return fmt.Errorf("failed to wait for CRD: %v", err)
	}
	err = WaitCRDReady(clientset, api.LoadBalancerEndpointCRDName)
	if err != nil {
		return fmt.Errorf("failed to wait for CRD: %v", err)
	}
	err = WaitCRDReady(clientset, api.DNSProviderCRDName)
	if err != nil {
		return fmt.Errorf("failed to wait for CRD: %v", err)
	}
	return nil
}

func CreateCRD(clientset clientset.Interface, crdName, rkind, rplural, shortName string) error {
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crdName,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   api.SchemeGroupVersion.Group,
			Version: api.SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: rplural,
				Kind:   rkind,
			},
		},
	}
	if len(shortName) != 0 {
		crd.Spec.Names.ShortNames = []string{shortName}
	}
	_, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func WaitCRDReady(clientset clientset.Interface, crdName string) error {
	err := wait.PollImmediate(5*time.Second, 60*time.Second, func() (bool, error) {
		crd, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					return true, nil
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					return false, fmt.Errorf("Name conflict: %v", cond.Reason)
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("wait CRD created failed: %v", err)
	}
	return nil
}
