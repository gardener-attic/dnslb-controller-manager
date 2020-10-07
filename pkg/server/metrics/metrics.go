// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"time"

	//"github.com/sirupsen/logrus"

	"github.com/gardener/controller-manager-library/pkg/resources"
	"github.com/gardener/controller-manager-library/pkg/server"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prometheus.MustRegister(EndpointHealth)
	prometheus.MustRegister(EndpointHosts)
	prometheus.MustRegister(EndpointActive)
	prometheus.MustRegister(LoadBalancers)
	prometheus.MustRegister(LoadBalancerDNS)
	prometheus.MustRegister(DNSReconciler)
	prometheus.MustRegister(DNSReconcileTime)

	server.RegisterHandler("/metrics", promhttp.Handler())

}

var (
	EndpointHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "endpoint_health",
			Help: "Health status of possible endpoints for dnsname Loadbalancers",
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
			Help: "Health status of possible endpoints for dnsname Loadbalancers",
		},
		[]string{"loadbalancer", "endpoint"},
	)
)

func ReportActiveEndpoint(lb, key resources.ObjectName, active bool) {
	setActive(EndpointActive.WithLabelValues(lb.String(), key.String()), active)
}

/////////////////////////////////////////////////////////////////////////////////
var (
	LoadBalancers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "loadbalancer_health",
			Help: "Health status of dnsname Loadbalancers",
		},
		[]string{"loadbalancer"},
	)
	LoadBalancerDNS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "loadbalancer_dnsnames",
			Help: "dnsname names for load balancers with health status",
		},
		[]string{"loadbalancer", "dnsname"},
	)
)

func ReportLB(key, dns string, active bool) {
	setActive(LoadBalancers.WithLabelValues(key), active)
	setActive(LoadBalancerDNS.WithLabelValues(key, dns), active)
}

/////////////////////////////////////////////////////////////////////////////////
var (
	lastReconcile time.Time
	DNSReconciler = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "dns_reconcile_interval",
			Help: "duration between two dnsname reconcilations",
		},
	)
	DNSReconcileTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "dns_reconcile_duration",
			Help: "duration of a dnsname reconcilation run",
		},
	)
)

func ReportStartDNSReconcile() {
	now := time.Now()
	if !lastReconcile.IsZero() {
		DNSReconciler.Set(float64(now.Sub(lastReconcile) / time.Second))
	}
	lastReconcile = now
}

func ReportDoneDNSReconcile() int {
	d := time.Now().Sub(lastReconcile) / time.Second
	DNSReconcileTime.Set(float64(d))
	return int(d)
}

/////////////////////////////////////////////////////////////////////////////////

func setActive(g prometheus.Gauge, active bool) {
	if active {
		g.Set(1)
	} else {
		g.Set(0)
	}
}
