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

package workqueue

import (
	"sync"

	"k8s.io/client-go/util/workqueue"
)

func DefaultControllerRateLimiter() workqueue.RateLimiter {
	return workqueue.DefaultControllerRateLimiter()
}

type RateLimitingInterface interface {
	workqueue.RateLimitingInterface

	WaitForChange(item interface{})
	HasError(item interface{}) bool
	AddChanged(item interface{})
}

type _workqueue struct {
	workqueue.RateLimitingInterface
	error map[interface{}]struct{}
	lock  sync.Mutex
}

func NewNamedRateLimitingQueue(rateLimiter workqueue.RateLimiter, name string) RateLimitingInterface {
	return &_workqueue{
		RateLimitingInterface: workqueue.NewNamedRateLimitingQueue(rateLimiter, name),
		error: map[interface{}]struct{}{},
	}
}

func (this *_workqueue) WaitForChange(item interface{}) {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.error[item] = struct{}{}
}

func (this *_workqueue) AddChanged(item interface{}) {
	this.lock.Lock()
	defer this.lock.Unlock()
	if _, ok := this.error[item]; ok {
		delete(this.error, item)
	}
	this.Forget(item)
	this.Add(item)
}

func (this *_workqueue) HasError(item interface{}) bool {
	this.lock.Lock()
	defer this.lock.Unlock()
	_, ok := this.error[item]
	return ok
}
