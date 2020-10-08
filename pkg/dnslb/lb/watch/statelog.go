// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"fmt"
	"github.com/gardener/controller-manager-library/pkg/logger"
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
