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

package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gardener/dnslb-controller-manager/cmd/dnslb-controller-manager/app"
	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/sirupsen/logrus"
)

func main() {
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	ctx := config.CancelContext(config.MainContext())
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-c
		logrus.Infof("process is being terminated")
		config.Cancel(ctx)
		<-c
		logrus.Infof("process is aborted immediately")
		os.Exit(0)
	}()

	command := app.NewCommandStartGardenRingControllerManager(os.Stdout, os.Stderr, ctx)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}

	logrus.Infof("waiting for all controllers to shutdown (max 120sec)")

	config.SyncPointWait(ctx, 120*time.Second)

	logrus.Infof("main exits")
}
