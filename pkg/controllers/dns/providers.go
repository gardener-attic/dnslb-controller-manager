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

package dns

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1/scope"
	"github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/model"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/provider"
	. "github.com/gardener/dnslb-controller-manager/pkg/controllers/dns/util"
	"github.com/gardener/dnslb-controller-manager/pkg/k8s"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"
	"github.com/gardener/dnslb-controller-manager/pkg/tools/workqueue"
)

func (this *Controller) handleAddObject(new interface{}) {
	pr := new.(*lbapi.DNSProvider)
	this.Debugf("-> new dns provider %s", k8s.Desc(pr))
	this.enqueue(pr, true)
}

func (this *Controller) handleDeleteObject(old interface{}) {
	pr := old.(*lbapi.DNSProvider)
	f := HasFinalizer(pr)
	if f {
		this.Debugf("-> delete dns provider %s", k8s.Desc(pr))
		this.enqueue(pr, true)
	}
}

func (this *Controller) handleUpdateObject(old, new interface{}) {
	newPr, _ := new.(*lbapi.DNSProvider)
	oldPr, _ := old.(*lbapi.DNSProvider)

	if newPr.GetResourceVersion() == oldPr.GetResourceVersion() {
		// Periodic resync will send update events for all known Resources.
		// Two different versions of the same Resource will always have different RVs.
		this.Debugf("-> reconcile dns provider %s", k8s.Desc(newPr))
		this.enqueue(newPr, false)
	} else {
		this.Debugf("-> update  dns provider %s", k8s.Desc(newPr))
		this.enqueue(newPr, true)
	}
}

// enqueue adds an object to the working queue.
// true: object has changed, ignore actual error state and rate limit
// false: add object rate limited, if last processing dediced to Forget
//        this just adds it to the queue
func (this *Controller) enqueue(pr *lbapi.DNSProvider, renew bool) {
	var key string
	var err error
	if key, err = k8s.ObjectKeyFunc(pr); err != nil {
		this.Error(err)
		return
	}
	if renew {
		this.workqueue.AddChanged(key)
	} else {
		this.workqueue.AddRateLimited(key)
	}
}

/////////////////////////////////////////////////////////////////////////////////

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
	w.ctx = c.Access
	if no >= 0 {
		w.ctx = c.Access.NewLogContext("worker", strconv.Itoa(no))
	}
	w.LogCtx = w.ctx
	w.workqueue = c.workqueue
	w.controller = c
	return w
}

func (this *Controller) runFor(prs []*lbapi.DNSProvider) {
	NewWorker(this, -1).RunFor(prs)
}

func (this *Worker) Run() {
	this.Infof("start")
	for this.processNextWorkItem() {
	}
	this.Infof("stop processing")
}
func (this *Worker) RunFor(prs []*lbapi.DNSProvider) {
	for _, pr := range prs {
		this.handleReconcile(pr)
	}
}

func (this *Worker) internalErr(obj interface{}, err error) bool {
	this.workqueue.Forget(obj)
	this.Error(err)
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
	healthz.Tick("provider")
	defer this.workqueue.Done(obj)
	key, ok := obj.(string)
	if !ok {
		return this.internalErr(obj, fmt.Errorf("expected string in workqueue but got %#v", obj))
	}
	_, namespace, name, err := k8s.SplitObjectKey(key)
	if err != nil {
		return this.internalErr(obj, fmt.Errorf("error syncing '%s': %s", key, err))
	}

	defer this.setResource(key)()

	s, err := this.controller.GetProvider(namespace, name)
	if err != nil {
		// The Provider resource may no longer exist, in which case we stop
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

func (this *Worker) handleReconcile(pr *lbapi.DNSProvider) (bool, error) {
	access, mod, err := scope.Eval(pr, &pr.Spec.Scope, this.LogCtx)

	if err != nil {
		pr.Status.State = "Error"
		if pr.Status.Message != err.Error() {
			this.Errorf("provider %s failed: %s", k8s.Desc(pr), err)
			pr.Status.Message = err.Error()
			this.controller.UpdateProvider(pr)
			mod = true
		}
	} else {
		if !HasFinalizer(pr) {
			this.Infof("set finalizer for %s", k8s.Desc(pr))
			SetFinalizer(pr)
			mod = true
		}
	}
	if mod {
		this.Infof("update provider %s (%s)", k8s.Desc(pr), pr.Spec.Scope)
		_, err := this.controller.UpdateProvider(pr)
		if err != nil {
			return true, err
		}
	}
	if err != nil {
		return true, err
	}

	name := fmt.Sprintf("%s/%s", pr.GetNamespace(), pr.GetName())
	old := GetRegistration(name)
	if old != nil {
		oldtype := old.GetType()
		newReg := GetTypeRegistration(pr.Spec.Type)
		if newReg == nil || oldtype != newReg.GetProviderType() {
			// delete and recreate provider instance with different type
			this.Infof("replace provider '%s' for new type '%s'->'%s'", name, oldtype, newReg.GetProviderType())
			old = UnregisterProvider(name)
			if old != nil {
				err := this.deleteRegistration(old)
				if err != nil {
					return true, fmt.Errorf("cleanup of dns provider failed: %s", err)
				}
			}
		} else {
			newCfg, err := this.getConfig(pr)
			if err != nil {
				return true, err
			}
			// update by unregister and recreate provider instance
			oldCfg := old.GetConfig()
			if oldCfg.Equals(newCfg) {
				old.SetAccessControl(access)
				this.setActive(pr)
				return true, nil
			}
			this.Infof("detecting config change for provider '%s':", name)
			this.Infof("old:  %+v", oldCfg)
			this.Infof("new:  %+v", newCfg)
			UnregisterProvider(name)
		}
	}
	prov, err := this.newProvider(pr)
	if err == nil {
		err = this.validate(prov)
	}
	if err != nil {
		pr.Status.State = "Error"
		if pr.Status.Message != err.Error() {
			this.Errorf("provider %s failed: %s", k8s.Desc(pr), err)
			pr.Status.Message = err.Error()
			this.controller.UpdateProvider(pr)
		}
		return false, err
	}
	this.setActive(pr)
	this.Infof("register new %s provider '%s'", pr.Spec.Type, name)
	RegisterProvider(name, prov, access)
	return true, nil
}

func (this *Worker) setActive(pr *lbapi.DNSProvider) {
	if pr.Status.State != "Active" {
		this.Infof("set provider '%s' to active", k8s.Desc(pr))
		pr.Status.State = "Active"
		pr.Status.Message = ""
		this.controller.UpdateProvider(pr)
	}
}

func (this *Worker) handleDelete(pr *lbapi.DNSProvider) (bool, error) {
	name := fmt.Sprintf("%s/%s", pr.GetNamespace(), pr.GetName())
	if HasFinalizer(pr) {
		this.Infof("deleteing dns provider %s", name)
		this.controller.lock.Lock()
		old := UnregisterProvider(name)
		this.controller.lock.Unlock()
		if old == nil {
			this.Infof("dns provider %s not yet registered -> fake registration entry", name)
			prov, err := this.newProvider(pr)
			if err != nil {
				//this.controller.Eventf(pr,corev1.EventTypeNormal, "delete", "%s", err)
				return true, fmt.Errorf("%s", err)
			}
			old = NewRegistration(name, prov)
		}
		err := this.deleteRegistration(old)
		if err != nil {
			return true, fmt.Errorf("cleanup of dns provider failed: %s", err)
		}

		RemoveFinalizer(pr)
		_, err = this.controller.UpdateProvider(pr)
		if err != nil {
			return true, err
		}

		secret, err := this.getSecret(pr)
		if secret != nil {
			if HasFinalizer(secret) {
				secret = secret.DeepCopy()
				RemoveFinalizer(secret)
				secret, err = this.controller.UpdateSecret(secret)
				if err != nil {
					return true, err
				}
			}
		}
	}
	return true, nil
}

func (this *Worker) validate(prov DNSProvider) error {
	domains := prov.GetDomains()
	this.Debugf("found: %+v", domains)
	return ForRegistrations(func(reg *Registration) error {
		have := reg.GetDomains()
		this.Debugf("checking %s: %+v", reg.GetName(), have)
		for d := range domains {
			if have.Contains(d) {
				return fmt.Errorf("duplicate domain '%s' with '%s'", d, reg.GetName())
			}
		}
		return nil
	})
}

func (this *Worker) getSecret(pr *lbapi.DNSProvider) (*corev1.Secret, error) {
	ref := pr.Spec.SecretRef
	if ref != nil {
		if ref.Namespace == "" {
			ref = ref.DeepCopy()
			ref.Namespace = pr.GetNamespace()
		}
		secret, err := this.controller.GetSecret(ref)
		if err != nil {
			return nil, fmt.Errorf("cannot get secret %s/%s for provider %s: %s",
				ref.Namespace, ref.Name, k8s.Desc(pr), err)
		}
		return secret, nil
	}
	return nil, nil
}

func (this *Worker) getConfig(pr *lbapi.DNSProvider) (map[string]string, error) {
	var config = map[string]string{}

	secret, err := this.getSecret(pr)
	if err != nil {
		return nil, err
	}
	if secret != nil {
		if !HasFinalizer(secret) {
			secret = secret.DeepCopy()
			SetFinalizer(secret)
			secret, err = this.controller.UpdateSecret(secret)
			if err != nil {
				return nil, err
			}
		}
		for k, v := range secret.Data {
			config[k] = string(v)
		}
	}
	return config, nil
}

func (this *Worker) newProvider(pr *lbapi.DNSProvider) (DNSProvider, error) {
	name := fmt.Sprintf("%s/%s", pr.GetNamespace(), pr.GetName())
	if pr.Spec.Type == "" {
		return nil, fmt.Errorf("type field missing for provider %s", k8s.Desc(pr))
	}
	config, err := this.getConfig(pr)
	if err != nil {
		return nil, err
	}
	ptype := GetTypeRegistration(pr.Spec.Type)
	if ptype == nil {
		return nil, fmt.Errorf("unknown provider type '%s' for  %s", pr.Spec.Type, k8s.Desc(pr))
	}
	return ptype.NewProvider(name, this.controller.cli_config, config, this.controller.NewLogContext("type", pr.Spec.Type).NewLogContext("provider", name))
}

func (this *Worker) deleteRegistration(reg *Registration) error {
	m := model.NewModel(this.controller.cli_config, this.controller.Access, this.controller.clientset, this.LogCtx)
	m.ForRegistrations = func(f func(*Registration) error) error {
		return f(reg)
	}
	m.Reset()
	return m.Update()
}
