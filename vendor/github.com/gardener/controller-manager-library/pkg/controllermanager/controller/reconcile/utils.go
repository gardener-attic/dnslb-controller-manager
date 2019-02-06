/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package reconcile

import (
	"fmt"

	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/resources"
	"k8s.io/apimachinery/pkg/api/errors"
)

func Succeeded(logger logger.LogContext, msg ...interface{}) Status {
	if len(msg) > 0 {
		logger.Info(msg...)
	}
	return Status{true, nil, -1}
}

func Repeat(logger logger.LogContext, err ...error) Status {
	for _, e := range err {
		logger.Error(e)
	}
	return Status{false, nil, -1}
}

func RepeatOnError(logger logger.LogContext, err error) Status {
	if err == nil {
		return Succeeded(logger)
	}
	return Repeat(logger, err)
}

func Delay(logger logger.LogContext, err error) Status {
	if err == nil {
		err = fmt.Errorf("reconcilation with problem")
	} else {
		logger.Warn(err)
	}
	return Status{true, err, -1}
}

func DelayOnError(logger logger.LogContext, err error) Status {
	if err == nil {
		return Succeeded(logger)
	}
	return Delay(logger, err)
}

func Failed(logger logger.LogContext, err error) Status {
	logger.Error(err)
	return Status{false, err, -1}
}

func FailedOnError(logger logger.LogContext, err error) Status {
	if err == nil {
		return Succeeded(logger)
	}
	return Failed(logger, err)
}

func FinalUpdate(logger logger.LogContext, modified bool, obj resources.Object) Status {
	if modified {
		return UpdateStatus(logger, obj.Update())
	}
	return Succeeded(logger)
}

func UpdateStatus(logger logger.LogContext, err error) Status {
	if err != nil {
		if errors.IsConflict(err) {
			return Repeat(logger, err)
		}
	}
	return DelayOnError(logger, err)
}

////////////////////////////////////////////////////////////////////////////////

func StringEqual(field *string, val string) bool {
	if field == nil {
		return val == ""
	}
	return val == *field
}

func StringValue(field *string) string {
	if field == nil {
		return ""
	}
	return *field
}

func StringSet(field **string, val string) {
	if val == "" {
		*field = nil
	}
	*field = &val
}

////////////////////////////////////////////////////////////////////////////////
