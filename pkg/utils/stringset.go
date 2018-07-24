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

import (
	"strings"
)

type StringSet map[string]struct{}

func (this StringSet) Contains(n string) bool {
	_, ok := this[n]
	return ok
}
func (this StringSet) Add(n string) {
	this[n] = struct{}{}
}
func (this StringSet) Remove(n string) {
	delete(this, n)
}
func (this StringSet) AddAll(n string) {
	for _, p := range strings.Split(n, ",") {
		this.Add(strings.ToLower(strings.TrimSpace(p)))
	}
}
