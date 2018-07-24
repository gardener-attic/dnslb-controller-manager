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

	"github.com/sirupsen/logrus"
)

type someEntry struct {
	ctx LogCtx
}

func (this *someEntry) NewLogContext(key, value string) LogCtx {
	return this.ctx.NewLogContext(key, value)
}

func (this *someEntry) StateInfof(key, msgfmt string, args ...interface{}) LogCtx {
	msg := fmt.Sprintf(msgfmt, args...)
	state[key] = msg
	this.ctx.Infof("%s", msg)
	return this
}
func (this *someEntry) Warnf(msgfmt string, args ...interface{}) {
	this.ctx.Warnf(msgfmt, args...)
}
func (this *someEntry) Warn(err error) {
	this.ctx.Warn(err)
}
func (this *someEntry) Errorf(msgfmt string, args ...interface{}) {
	this.ctx.Errorf(msgfmt, args...)
}
func (this *someEntry) Error(err error) {
	this.ctx.Warn(err)
}
func (this *someEntry) Infof(msgfmt string, args ...interface{}) {
	this.ctx.Infof(msgfmt, args...)
}
func (this *someEntry) Debugf(msgfmt string, args ...interface{}) {
	this.ctx.Debugf(msgfmt, args...)
}

type nullEntry struct {
	*someEntry
}

func (this *nullEntry) StateInfof(key, msgfmt string, args ...interface{}) LogCtx {
	msg := fmt.Sprintf(msgfmt, args...)
	if state[key] != msg || logrus.GetLevel() > logrus.InfoLevel {
		state[key] = msg
		this.ctx.Infof("%s", msg)
		return this.someEntry
	}
	return this
}
func (this *nullEntry) Infof(msgfmt string, args ...interface{}) {
	if logrus.GetLevel() > logrus.InfoLevel {
		this.someEntry.Infof(msgfmt, args...)
	}
}

/////////////////////////////////////////////////////////////////////////////////

var state = map[string]string{}
var _nullEntry = defaultLogContext.getNull()

func StateInfof(key, msgfmt string, args ...interface{}) LogCtx {
	return _nullEntry.StateInfof(key, msgfmt, args...)
}
