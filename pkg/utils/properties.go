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

package utils

type Properties map[string]string

func (this Properties) HasKey(k string) bool {
	_, ok := this[k]
	return ok
}

func (this Properties) Equals(t map[string]string) bool {
	for k, v := range this {
		tv, ok := t[k]
		if !ok || tv != v {
			return false
		}
	}
	for k := range t {
		if !this.HasKey(k) {
			return false
		}
	}
	return true
}
