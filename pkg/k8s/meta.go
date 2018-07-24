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

package k8s

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	//	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Object interface {
	metav1.Object
	runtime.Object
}

type ObjectIdentity interface {
	GetKind() string
	GetName() string
	GetNamespace() string
}

func ObjectKind(obj Object) string {
	return obj.GetObjectKind().GroupVersionKind().Kind
}

func ObjectKeyFunc(obj interface{}) (string, error) {
	if key, ok := obj.(string); ok {
		return string(key), nil
	}
	id, ok := obj.(ObjectIdentity)
	if ok {
		return id.GetKind() + "/" + id.GetNamespace() + "/" + id.GetName(), nil
	}
	meta, ok := obj.(Object)
	if !ok {
		return "", fmt.Errorf("object has no meta")
	}
	return ObjectKind(meta) + "/" + meta.GetNamespace() + "/" + meta.GetName(), nil
}

func SplitObjectKey(key string) (kind, namespace, name string, err error) {
	parts := strings.Split(key, "/")
	switch len(parts) {
	case 1:
		// name only, no namespace
		return "", "", parts[0], nil
	case 2:
		// kind, name
		return parts[0], "", parts[1], nil
	case 3:
		// kind, namespace and name
		return parts[0], parts[1], parts[2], nil
	}

	return "", "", "", fmt.Errorf("unexpected key format: %q", key)
}

func Desc(obj Object) string {
	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
}
