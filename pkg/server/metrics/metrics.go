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

package metrics

import (
	//"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prometheus.MustRegister(EndpointHealth)
	prometheus.MustRegister(EndpointHosts)
	prometheus.MustRegister(EndpointActive)
	prometheus.MustRegister(LoadBalancers)
	prometheus.MustRegister(LoadBalancerDNS)
}

var (
	EndpointHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "endpoint_health",
			Help: "Health status of possible endpoints for DNS Loadbalancers",
		},
		[]string{"loadbalancer", "endpoint"},
	)
	EndpointHosts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "endpoint_hosts",
			Help: "Hostnames for endpoints with health status",
		},
		[]string{"loadbalancer", "endpoint", "host"},
	)
)

func ReportEndpoint(lb, key, host string, active bool) {
	setActive(EndpointHealth.WithLabelValues(lb, key), active)
	setActive(EndpointHosts.WithLabelValues(lb, key, host), active)
}

/////////////////////////////////////////////////////////////////////////////////

var (
	EndpointActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "endpoint_active",
			Help: "Health status of possible endpoints for DNS Loadbalancers",
		},
		[]string{"loadbalancer", "endpoint"},
	)
)

func ReportActiveEndpoint(lb, key string, active bool) {
	setActive(EndpointActive.WithLabelValues(lb, key), active)
}

/////////////////////////////////////////////////////////////////////////////////
var (
	LoadBalancers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "loadbalancer_health",
			Help: "Health status of DNS Loadbalancers",
		},
		[]string{"loadbalancer"},
	)
	LoadBalancerDNS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "loadbalancer_dnsnames",
			Help: "DNS names for load balancers with health status",
		},
		[]string{"loadbalancer", "dnsname"},
	)
)

func ReportLB(key, dns string, active bool) {
	setActive(LoadBalancers.WithLabelValues(key), active)
	setActive(LoadBalancerDNS.WithLabelValues(key, dns), active)
}

/////////////////////////////////////////////////////////////////////////////////

func setActive(g prometheus.Gauge, active bool) {
	if active {
		g.Set(1)
	} else {
		g.Set(0)
	}
}
