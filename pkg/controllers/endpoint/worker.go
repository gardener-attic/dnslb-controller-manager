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

package endpoint

import (
	"fmt"
	"strconv"

	"github.com/gardener/dnslb-controller-manager/pkg/controllers/endpoint/source"
	"github.com/gardener/dnslb-controller-manager/pkg/k8s"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"
	"github.com/gardener/dnslb-controller-manager/pkg/tools/workqueue"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// Worker describe a single threaded worker entity synchronously working
// on requests provided by the controller workqueue
// It is basically a single go routine with a state for subsequenet methods
// called from this go routine
type Worker struct {
	log.LogCtx
	ctx        log.LogCtx
	controller *Controller
	workqueue  workqueue.RateLimitingInterface
}

func NewWorker(c *Controller, no int) *Worker {
	w := &Worker{}

	w.ctx = c.NewLogContext("worker", strconv.Itoa(no))
	w.LogCtx = w.ctx
	w.controller = c
	w.workqueue = c.workqueue
	return w
}

func (this *Worker) Run() {
	this.Infof("start")
	for this.processNextWorkItem() {
	}
	this.Infof("stop")
}

func (this *Worker) internalErr(obj interface{}, err error) bool {
	this.workqueue.Forget(obj)
	this.ctx.Error(err)
	return true
}

func (this *Worker) setResource(key string) func() {
	this.LogCtx = this.ctx.NewLogContext("resource", key)
	return func() { this.LogCtx = this.ctx }
}

func (this *Worker) processNextWorkItem() bool {
	obj, shutdown := this.workqueue.Get()

	if shutdown {
		return false
	}

	defer this.workqueue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		return this.internalErr(obj, fmt.Errorf("expected string in workqueue but got %#v", obj))
	}

	if this.handleTask(key) {
		return true
	}

	healthz.Tick("endpoint")

	kind, namespace, name, err := k8s.SplitObjectKey(key)
	if err != nil {
		return this.internalErr(obj, fmt.Errorf("error syncing '%s': %s", key, err))
	}
	defer this.setResource(key)()
	id := source.NewSourceId(kind, namespace, name)

	s, err := this.controller.GetSource(id)
	if err != nil {
		// The Service resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			return this.internalErr(obj, fmt.Errorf("%s in work queue no longer exists", key))
		} else {
			this.Errorf("error syncing '%s': %s", key, err)
			this.workqueue.AddRateLimited(key)
			return true
		}
	} else {

		s = s.DeepCopy()
		if s.GetDeletionTimestamp() == nil {
			ok, err = this.handleReconcile(s)
		} else {
			ok, err = this.handleDelete(s)

		}
		if err != nil {
			this.controller.Eventf(s, corev1.EventTypeWarning, "sync", err.Error())
			//runtime.HandleError(fmt.Errorf("error syncing '%s': %s", key, err))
			if ok {
				// some problem reported, but valid state -> rate limit
				this.Errorf("problem syncing '%s': %s", key, err)
				this.workqueue.AddRateLimited(key)
			} else {
				// object config error -> wait for new change of object
				this.Errorf("wait for new change '%s': %s", key, err)
				this.workqueue.WaitForChange(obj)
			}
		} else {
			if ok {
				// no error, and everything valid -> just reset rate limter
				this.workqueue.Forget(obj)
			} else {
				// operation temporarily failed (no error) -> just redo operation
				this.Infof("redo reconcile for '%s':", key)
				this.workqueue.Add(obj)
			}
		}
	}
	this.Debugf("done with %s", key)
	return true
}
