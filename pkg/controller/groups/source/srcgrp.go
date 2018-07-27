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

package source

import (
	"context"
	"time"

	informers "k8s.io/client-go/informers"

	"github.com/gardener/dnslb-controller-manager/pkg/controller/groups"
)

var Name = "source"

func init() {
	groups.GetType(Name).SetActivator(new)
}

func new(g *groups.Group, ctx context.Context) (groups.GroupData, context.Context, error) {
	d := &GroupData{}
	d.informerFactory = informers.NewSharedInformerFactory(g.GetClientset(), 30*time.Second)
	ctx = context.WithValue(ctx, g.GetName()+"InformerFactory", d.informerFactory)
	return d, ctx, nil
}

type GroupData struct {
	informerFactory informers.SharedInformerFactory
}

var _ groups.GroupData = &GroupData{}
