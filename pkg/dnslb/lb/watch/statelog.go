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

package watch

import (
	"fmt"
	"github.com/gardener/lib/pkg/logger"
	"github.com/sirupsen/logrus"
)

type LogContext interface {
	logger.LogContext
	StateInfof(key, msgfmt string, args ...interface{}) LogContext
}

type someEntry struct {
	logger.LogContext
}

func (this *someEntry) NewContext(key, value string) logger.LogContext {
	return this.LogContext.NewContext(key, value)
}

func (this *someEntry) StateInfof(key, msgfmt string, args ...interface{}) LogContext {
	msg := fmt.Sprintf(msgfmt, args...)
	state[key] = msg
	this.LogContext.Infof("%s", msg)
	return this
}

type nullEntry struct {
	*someEntry
}

func (this *nullEntry) StateInfof(key, msgfmt string, args ...interface{}) LogContext {
	msg := fmt.Sprintf(msgfmt, args...)
	if state[key] != msg || logrus.GetLevel() > logrus.InfoLevel {
		state[key] = msg
		this.LogContext.Infof("%s", msg)
		return this.someEntry
	}
	return this
}

func (this *nullEntry) Infof(msgfmt string, args ...interface{}) {
	if logrus.GetLevel() > logrus.InfoLevel {
		this.someEntry.Infof(msgfmt, args...)
	}
}
func (this *nullEntry) Info(args ...interface{}) {
	if logrus.GetLevel() > logrus.InfoLevel {
		this.someEntry.Info(args...)
	}
}

/////////////////////////////////////////////////////////////////////////////////

var state = map[string]string{}

func InactiveContext(ctx logger.LogContext) LogContext {
	return &nullEntry{&someEntry{ctx}}
}
