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
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/dnslb-controller-manager/pkg/log"
)

func AssureString(s *string, n string) bool {
	if *s != n {
		*s = n
		return false
	}
	return true
}

func GetLabel(obj metav1.Object, name string) string {
	labels := obj.GetLabels()
	return labels[name]
}

func AssureLabel(obj metav1.Object, name, value string) bool {
	labels := obj.GetLabels()
	if labels[name] != value {
		labels[name] = value
		return false
	}
	return true
}

type ObjectRef interface {
	GetName() string
	GetNamespace() string
}

type objectRef struct {
	Namespace string
	Name      string
}

func NewObjectRef(ns, name string) ObjectRef {
	return &objectRef{ns, name}
}
func (this *objectRef) GetName() string {
	return this.Name
}
func (this *objectRef) GetNamespace() string {
	return this.Namespace
}
func (this *objectRef) String() string {
	return this.Namespace + "/" + this.Name
}

func Ref(obj metav1.Object) ObjectRef {
	return NewObjectRef(obj.GetNamespace(), obj.GetName())
}

func RefEquals(r1, r2 ObjectRef) bool {
	if r1 == r2 {
		return true
	}
	if r1 == nil || r2 == nil {
		return false
	}
	return r1.GetNamespace() == r2.GetNamespace() && r1.GetName() == r2.GetName()
}

func GetLoadBalancerRef(obj metav1.Object) (ObjectRef, error) {
	for n, v := range obj.GetAnnotations() {
		if n == AnnotationLoadbalancer {
			parts := strings.Split(v, "/")
			switch len(parts) {
			case 1:
				return NewObjectRef(obj.GetNamespace(), parts[0]), nil
			case 2:
				return NewObjectRef(parts[0], parts[1]), nil
			default:
				return nil, fmt.Errorf("invalid dns loadbalancer ref '%s'", v)
			}
		}
	}
	return nil, nil
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

func UpdateDeadline(duration *metav1.Duration, deadline *metav1.Time, log log.LogCtx) *metav1.Time {
	if duration != nil && duration.Duration != 0 {
		if log != nil {
			log.Debugf("validity interval found: %s", duration.Duration)
		}
		now := time.Now()
		if deadline != nil {
			sub := deadline.Time.Sub(now)
			if sub > 120*time.Second {
				if log != nil {
					log.Debugf("time diff: %s", sub)
				}
				return deadline
			}
		}
		next := metav1.NewTime(now.Add(duration.Duration))
		if log != nil {
			log.Debugf("update deadline (%s+%s): %s", now, duration.Duration, next.Time)
		}
		return &next
	}
	return nil
}
