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

package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ObjectRef struct {
	Namespace string
	Name      string
}

func GetObjectRef(obj metav1.Object) ObjectRef {
	return ObjectRef{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func HasFinalizer(obj metav1.Object) bool {
	for _, n := range obj.GetFinalizers() {
		if n == Finalizer {
			return true
		}
	}
	return false
}

func SetFinalizer(obj metav1.Object) {
	if !HasFinalizer(obj) {
		obj.SetFinalizers(append(obj.GetFinalizers(), Finalizer))
	}
}

func RemoveFinalizer(obj metav1.Object) {
	list := obj.GetFinalizers()
	for i, n := range list {
		if n == Finalizer {
			obj.SetFinalizers(append(list[:i], list[i+1:]...))
		}
	}
}
