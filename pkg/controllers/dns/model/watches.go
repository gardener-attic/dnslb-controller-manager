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

package model

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"

	lbapi "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/k8s"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	"github.com/gardener/dnslb-controller-manager/pkg/server/metrics"

	corev1 "k8s.io/api/core/v1"
)

type WatchConfig struct {
	Watches []*Watch `yaml:"watches" json:"watches"`
}

func ReadConfig(path string) (*WatchConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := &WatchConfig{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

////////////////////////////////////////////////////////////////////////////////
// DNS Target
////////////////////////////////////////////////////////////////////////////////

type Target interface {
	GetHostName() string
	GetRecordType() string
	GetKey() string
}

type Target_ struct {
	Name      string                         `yaml:"naḿe,omitempty" json:"name,omitempt"`
	IPAddress string                         `yaml:"IP,omitempt" json:"IP,omitempt"`
	DNSEP     *lbapi.DNSLoadBalancerEndpoint `yaml:"-"`
}

type target struct {
	rtype string
	host  string
	dnsep *lbapi.DNSLoadBalancerEndpoint
}

func NewText(t string) Target {
	return NewTarget("TXT", t, nil)
}
func NewTarget(ty string, ta string, ep *lbapi.DNSLoadBalancerEndpoint) Target {
	return &target{rtype: ty, host: ta, dnsep: ep}
}
func (t *target) GetHostName() string   { return t.host }
func (t *target) GetRecordType() string { return t.rtype }
func (t *target) GetKey() string {
	if t.dnsep != nil {
		return k8s.Desc(t.dnsep)
	}
	return t.GetHostName()
}

func (t *Target_) GetHostName() string {
	if t.Name != "" {
		return t.Name
	}
	return t.IPAddress
}

func (t *Target_) GetRecordType() string {
	if t.Name != "" {
		return "CNAME"
	}
	return "A"
}

func (t *Target_) GetKey() string {
	if t.DNSEP != nil {
		return k8s.Desc(t.DNSEP)
	}
	return t.GetHostName()
}

func (t *Target_) IsValid() bool {
	return t.Name != "" || t.IPAddress != ""
}

func (t *Target_) String() string {
	return fmt.Sprintf("target %s(%s)", t.GetRecordType(), t.GetHostName())
}

////////////////////////////////////////////////////////////////////////////////
// Watch Request
////////////////////////////////////////////////////////////////////////////////

type Watch struct {
	DNS        string                 `yaml:"name" json:"name"`
	HealthPath string                 `yaml:"healthPath" json:"healthPath"`
	StatusCode int                    `yaml:"statusCode,omitempty" json:"statusCode,omitempty"`
	Targets    []*Target_             `yaml:"targets" json:"targets"`
	Singleton  bool                   `yaml:"singleton,omitempty" json:"singleton,omitempty"`
	DNSLB      *lbapi.DNSLoadBalancer `yaml:"-"`
}

func (w *Watch) String() string {
	if w.DNSLB == nil {
		return w.DNS
	}
	return fmt.Sprintf("%s/%s [%s]", w.DNSLB.GetNamespace(), w.DNSLB.GetName(), w.DNS)
}

func (w *Watch) GetKey() string {
	if w.DNSLB == nil {
		return w.DNS
	}
	return fmt.Sprintf("%s/%s", w.DNSLB.GetNamespace(), w.DNSLB.GetName())
}

func (w *Watch) Handle(m *Model) {
	m.Debugf("handle %s", w.DNS)

	done := NewStatusUpdate(m, w)
	healthyTargets := []Target{}
	msg := ""
	if len(w.Targets) == 0 {
		m.StateInfof(w.DNS, "no endpoints configured for %s", w)
		done.Error(true, fmt.Errorf("no endpoints configured"))
		return
	}

	var ctx log.LogCtx

	if w.IsHealthy(w.DNS) {
		done.SetHealthy(true)
		ctx = m.StateInfof(w.DNS, "%s is healthy", w)
		metrics.ReportLB(w.GetKey(), w.DNS, true)
	} else {
		done.SetHealthy(false)
		ctx = m.StateInfof(w.GetKey(), w.DNS, "%s is NOT healthy", w)
		metrics.ReportLB(w.GetKey(), w.DNS, false)
	}

	if w.Singleton {
		for _, target := range w.Targets {
			mod, err := m.Check(w.DNS, w.DNSLB, done, target)
			if err != nil {
				m.Errorf("error handling %s: %s", w.DNS, err)
			}
			if w.IsHealthy(target.GetHostName(), w.DNS) {
				metrics.ReportEndpoint(w.GetKey(), target.GetKey(), target.GetHostName(), true)
				if len(healthyTargets) == 0 {
					healthyTargets = append(healthyTargets, target)
				}
				done.AddHealthyTarget(target)
				if !mod {
					healthyTargets[0] = target
					ctx.StateInfof(target.GetHostName(), "healthy active target for %s is %s", w.DNS, target.GetHostName())
				} else {
					ctx.StateInfof(target.GetHostName(), "target %s is healthy", target.GetHostName())
				}
			} else {
				metrics.ReportEndpoint(w.GetKey(), target.GetKey(), target.GetHostName(), false)
				done.AddUnhealthyTarget(target)
				if !mod {
					ctx.StateInfof(target.GetHostName(), "active target %s is unhealthy", target.GetHostName())
				} else {
					ctx.StateInfof(target.GetHostName(), "target %s is unhealthy", target.GetHostName())
				}
			}
		}
		if len(healthyTargets) != 0 {
			done.AddActiveTarget(healthyTargets[0].(*Target_))
		}
	} else {

		for _, target := range w.Targets {
			if w.IsHealthy(target.GetHostName(), w.DNS) {
				metrics.ReportEndpoint(w.GetKey(), target.GetKey(), target.GetHostName(), true)
				ctx.StateInfof(target.GetHostName(), "target %s is healthy", target.GetHostName())
				done.AddActiveTarget(target)
				done.AddHealthyTarget(target)
				healthyTargets = append(healthyTargets, target)
				msg = fmt.Sprintf("%s %s", msg, target.GetHostName())
			} else {
				metrics.ReportEndpoint(w.GetKey(), target.GetKey(), target.GetHostName(), false)
				ctx.StateInfof(target.GetHostName(), "target %s in unhealthy", target.GetHostName())
				done.AddUnhealthyTarget(target)
			}
		}
	}
	mod, err := m.Apply(w.DNS, w.DNSLB, done, healthyTargets...)
	if err != nil {
		if done.IsInvalid() {
			done.Failed(err)
			return
		}
		m.recorder.Eventf(w.DNSLB, corev1.EventTypeWarning, "sync", "error handling %s: %s", w.DNS, err)
		logrus.Errorf("error handling %s: %s", w.DNS, err)
	}
	if mod {
		done.SetMessage(fmt.Sprintf("replacing %s with %s", w.DNS, msg))
		logrus.Info(done.message)
	} else {
		if !done.HasHealthy() {
			ctx.Infof("no healthy targets found")
			done.Failed(fmt.Errorf("no healthy targets found"))
		} else {
			done.Succeeded()
		}
	}
}

func (w *Watch) IsHealthy(name string, dns ...string) bool {
	var (
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client   = &http.Client{Transport: tr}
		hostname = fmt.Sprintf("https://%s%s", name, w.HealthPath)
	)
	statusCode := w.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	req, err := http.NewRequest("GET", hostname, nil)
	if err != nil {
		return false
	}
	if len(dns) > 0 {
		req.Header.Add("Host", dns[0])
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	resp.Body.Close()
	return resp.StatusCode == statusCode
}
