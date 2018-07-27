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

package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gardener/dnslb-controller-manager/pkg/controller/clientset"

	"github.com/gardener/dnslb-controller-manager/pkg/tools/leaderelection"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	//"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

type StartFunc func(clientset clientset.Interface, ctx context.Context)

func StartWithLease(msg string, clientset clientset.Interface, ctx context.Context, f StartFunc) error {
	logrus.Infof("requesting lease for %s", msg)
	recorder := createRecorder(clientset)
	leaderElectionConfig, err := makeLeaderElectionConfig(clientset, recorder)
	if err != nil {
		return err
	}

	leaderElectionConfig.Callbacks = leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			logrus.Infof("Acquired leadership, starting controllers for %s.", msg)
			f(clientset, ctx)
		},
		OnStoppedLeading: func() {
			logrus.Infof("Lost leadership, cleaning up %s.", msg)
		},
	}
	leaderElector, err := leaderelection.NewLeaderElector(*leaderElectionConfig)
	if err != nil {
		return fmt.Errorf("couldn't create leader elector: %v", err)
	}

	go leaderElector.Run(ctx)

	return nil
}

func createRecorder(kubeClient clientset.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Debugf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "dnslb-controller-manager"})
}

func makeLeaderElectionConfig(clientset clientset.Interface, recorder record.EventRecorder) (*leaderelection.LeaderElectionConfig, error) {
	hostname, err := os.Hostname()
	hostname = fmt.Sprintf("%s/%d", hostname, os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	lock, err := resourcelock.New(
		"configmaps",
		"garden",
		"dnslb-controller-manager-lease",
		clientset.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      hostname,
			EventRecorder: recorder,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create resource lock: %v", err)
	}

	return &leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
	}, nil
}
