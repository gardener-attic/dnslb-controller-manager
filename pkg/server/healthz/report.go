// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthz

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

func Tick(key string) {
	setCheck(key)
}

func Start(key string) {
	setCheck(key)
}

func End(key string) {
	removeCheck(key)
}

var checks = map[string]time.Time{}
var lock sync.Mutex
var timeout = 2 * time.Minute

func setCheck(key string) {
	lock.Lock()
	defer lock.Unlock()

	checks[key] = time.Now()
}

func removeCheck(key string) {
	lock.Lock()
	defer lock.Unlock()

	delete(checks, key)
}

func isHealthy() bool {
	lock.Lock()
	defer lock.Unlock()

	limit := time.Now().Add(-timeout)

	for key, t := range checks {
		if t.Before(limit) {
			logrus.Warningf("outdated health check '%s': %s", key, limit.Sub(t))
			return false
		}
		logrus.Debugf("%s: %s", key, t)
	}
	return true

}

func healthInfo() (bool, string) {
	lock.Lock()
	defer lock.Unlock()

	limit := time.Now().Add(-timeout)
	info := ""
	for key, t := range checks {
		info = fmt.Sprintf("%s%s: %s\n", info, key, t)
		if t.Before(limit) {
			logrus.Warningf("outdated health check '%s': %s", key, limit.Sub(t))
			return false, info
		}
		logrus.Debugf("%s: %s", key, t)
	}
	return true, info

}

func SetTimeout(d time.Duration) {
	timeout = d
}
