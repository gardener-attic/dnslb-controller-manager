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

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gardener/dnslb-controller-manager/pkg/server/healthz"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Serve starts a HTTP server.
func Serve(ctx context.Context, bindAddress string, port int) {
	logrus.Info("adding health and metrics endpoint")
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", healthz.Healthz)

	listenAddress := fmt.Sprintf("%s:%d", bindAddress, port)
	server := &http.Server{Addr: listenAddress, Handler: nil}
	go func() {
		<-ctx.Done()
		logrus.Infof("shutting down server with timeout")
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		server.Shutdown(ctx)
	}()
	go func() {
		logrus.Infof("DNS loadbalancer controller manager HTTP server started (serving on %s)", listenAddress)
		server.ListenAndServe()
		logrus.Infof("server stopped")
	}()

}
