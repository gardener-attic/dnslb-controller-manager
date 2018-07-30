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

package scope

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/k8s"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"
)

type AccessControl interface {
	ValidFor(bj metav1.Object) bool
}

type accessControl struct {
	namespaces StringSet
}

func (this *accessControl) ValidFor(obj metav1.Object) bool {
	return this.namespaces == nil || this.namespaces.Contains(obj.GetNamespace())
}

func Eval(obj metav1.Object, scoperef **Scope, log log.LogCtx) (AccessControl, bool, error) {
	var err error
	mod := false
	access := &accessControl{}

	if *scoperef == nil {
		*scoperef = &Scope{Type: SCOPE_CLUSTER}
		mod = true
	} else {
		switch (*scoperef).Type {
		case "":
			log.Infof("adapt scope for %s", k8s.Desc(obj))
			(*scoperef).Type = SCOPE_CLUSTER
			(*scoperef).Namespaces = nil
			mod = true
		case SCOPE_CLUSTER, SCOPE_NAMESPACE:
			if (*scoperef).Namespaces != nil && len((*scoperef).Namespaces) > 0 {
				(*scoperef).Namespaces = nil
				mod = true
			}
			if (*scoperef).Type == SCOPE_NAMESPACE {
				access.namespaces = NewStringSet(obj.GetNamespace())
			}
		case SCOPE_SELECTED:
			access.namespaces = NewStringSetByArray((*scoperef).Namespaces)
		default:
			err = fmt.Errorf("invalid provider scope '%s'", (*scoperef).Type)
		}
	}
	return access, mod, err
}
