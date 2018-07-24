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

package log

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

type LogCtx interface {
	NewLogContext(key, value string) LogCtx

	Errorf(msgfmt string, args ...interface{})
	Error(err error)
	Warnf(msgfmt string, args ...interface{})
	Warn(err error)
	Infof(msgfmt string, args ...interface{})
	Debugf(msgfmt string, args ...interface{})

	StateInfof(key, msgfmt string, args ...interface{}) LogCtx
}

//////////////////////////////////////////////////////////////////////////////////

type _LogCtx struct {
	key   string
	entry *logrus.Entry
	lock  sync.Mutex
	null  LogCtx
}

var _ LogCtx = &_LogCtx{}

var defaultLogContext = New().(*_LogCtx)

func NewLogContext(key, value string) LogCtx {
	return &_LogCtx{key: fmt.Sprintf("%s: ", value), entry: logrus.StandardLogger().WithField(key, value)}
}

func New() LogCtx {
	return &_LogCtx{key: "", entry: logrus.NewEntry(logrus.StandardLogger())}
}

func (this *_LogCtx) NewLogContext(key, value string) LogCtx {
	return &_LogCtx{key: fmt.Sprintf("%s%s: ", this.key, value), entry: this.entry.WithField(key, value)}
}

func (this *_LogCtx) getNull() LogCtx {
	if this.null == nil {
		this.lock.Lock()
		if this.null == nil {
			some := &someEntry{this}
			this.null = &nullEntry{some}
		}
		this.lock.Unlock()
	}
	return this.null
}

func (this *_LogCtx) StateInfof(key, msgfmt string, args ...interface{}) LogCtx {
	return this.getNull().StateInfof(key, msgfmt, args...)
}

func (this *_LogCtx) Debugf(msgfmt string, args ...interface{}) {
	this.entry.Debugf(this.key+msgfmt, args...)
}

func (this *_LogCtx) Infof(msgfmt string, args ...interface{}) {
	this.entry.Infof(this.key+msgfmt, args...)
}

func (this *_LogCtx) Warnf(msgfmt string, args ...interface{}) {
	this.entry.Warnf(this.key+msgfmt, args...)
}
func (this *_LogCtx) Warn(err error) {
	this.entry.Warnf("%s%s", this.key, err)
}

func (this *_LogCtx) Errorf(msgfmt string, args ...interface{}) {
	this.entry.Errorf(this.key+msgfmt, args...)
}
func (this *_LogCtx) Error(err error) {
	this.entry.Errorf("%s%s", this.key, err)
}

/////////////////////////////////////////////////////////////////////////////////

func Debugf(msgfmt string, args ...interface{}) {
	defaultLogContext.Debugf(msgfmt, args...)
}

func Infof(msgfmt string, args ...interface{}) {
	defaultLogContext.Infof(msgfmt, args...)
}

func Warnf(msgfmt string, args ...interface{}) {
	defaultLogContext.Warnf(msgfmt, args...)
}
func Warn(err error) {
	defaultLogContext.Warn(err)
}

func Errorf(msgfmt string, args ...interface{}) {
	defaultLogContext.Errorf(msgfmt, args...)
}
func Error(err error) {
	defaultLogContext.Error(err)
}
