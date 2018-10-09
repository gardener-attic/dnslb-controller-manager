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

package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/gardener/dnslb-controller-manager/pkg/log"

	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"
)

type SchemeAdder func(*runtime.Scheme) error

/////////////////////////////////////////////////////////////////////////////////

type ObjectRef struct {
	Namespace string
	Name      string
}

func NewObjectRef(ns, name string) ObjectRef {
	return ObjectRef{ns, name}
}

func (this *ObjectRef) GetName() string {
	return this.Name
}
func (this *ObjectRef) GetNamespace() string {
	return this.Namespace
}
func (this *ObjectRef) String() string {
	return this.Namespace + "/" + this.Name
}

func Ref(obj metav1.Object) ObjectRef {
	return NewObjectRef(obj.GetNamespace(), obj.GetName())
}

/////////////////////////////////////////////////////////////////////////////////

type Event struct {
	eventtype string
	reason    string
	message   string
}

func NewEvent(eventtype, reason, message string) *Event {
	return &Event{eventtype, reason, message}
}

func (this *Event) Equals(e *Event) bool {
	return this.eventtype == e.eventtype && this.reason == e.reason && this.message == e.message
}

func (this *Event) String() string {
	return this.eventtype + "/" + this.reason + "/" + this.message
}

/////////////////////////////////////////////////////////////////////////////////

type EventRecorder interface {
	Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{})
	Event(object runtime.Object, eventtype, reason, message string)
}

type _EventRecorder struct {
	log.LogCtx
	record.EventRecorder
	sent map[ObjectRef]*Event
}

var _ EventRecorder = &_EventRecorder{}

func NewEventRecorder(logctx log.LogCtx, agentName string, clientset clientset.Interface, adder ...SchemeAdder) record.EventRecorder {
	for _, a := range adder {
		a(scheme.Scheme)
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logctx.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	return &_EventRecorder{
		LogCtx:        logctx,
		EventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: agentName}),
		sent:          map[ObjectRef]*Event{},
	}
}

func (this *_EventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	this.Event(object, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func (this *_EventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	o, ok := object.(metav1.Object)
	if ok {
		e := NewEvent(eventtype, reason, message)
		ref := Ref(o)
		last := this.sent[ref]
		if last != nil {
			if last.Equals(e) {
				this.Debugf("skip event for %s: %s", &ref, e)
				return
			}
		}
		this.sent[ref] = e
		this.Infof("send event for %s: %s", &ref, e)
	}
	this.EventRecorder.Event(object, eventtype, reason, message)
}
