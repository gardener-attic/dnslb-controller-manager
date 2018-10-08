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
	"time"

	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"

	"k8s.io/apimachinery/pkg/labels"
)

type Task interface {
	GetName() string
	GetInterval() time.Duration
	Execute(w *Worker)
}

type ScheduledTask struct {
	name     string
	interval time.Duration
	task     func(w *Worker)
}

func (this *ScheduledTask) GetName() string {
	return this.name
}
func (this *ScheduledTask) GetInterval() time.Duration {
	return this.interval
}
func (this *ScheduledTask) Execute(w *Worker) {
	this.task(w)
}

var tasks = map[string]Task{}

func init() {
	t := &ScheduledTask{"unusedcheck", healthz.CycleTime, lifeCheck}
	tasks[t.GetName()] = t
}

/////////////////////////////////////////////////////////////////////////////////

func (this *Controller) scheduleTasks() {
	for _, t := range tasks {
		this.scheduleTask(t)
	}
}

func (this *Controller) scheduleTask(t Task) {
	this.workqueue.AddAfter(t.GetName(), t.GetInterval())
}

func (this *Worker) handleTask(name string) bool {
	t, ok := tasks[name]
	if ok {
		t.Execute(this)
		this.workqueue.Forget(name)
		this.controller.scheduleTask(t)

	}
	return ok
}

/////////////////////////////////////////////////////////////////////////////////

func lifeCheck(w *Worker) {
	list, err := w.controller.lbLister.DNSLoadBalancers("").List(labels.Everything())
	if err == nil && len(list) == 0 {
		w.Debugf("no lbs found -> health tick")
		healthz.Tick("endpoint")
	}
}
