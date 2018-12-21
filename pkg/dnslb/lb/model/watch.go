package model

import (
	"crypto/tls"
	"fmt"
	"github.com/mandelsoft/dns-controller-manager/pkg/dns/source"

	//api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"
	"github.com/gardener/dnslb-controller-manager/pkg/server/metrics"
	"github.com/gardener/lib/pkg/logger"
	"net/http"
)

////////////////////////////////////////////////////////////////////////////////
// Watch Request
////////////////////////////////////////////////////////////////////////////////

type Target struct {
	Name      string
	IPAddress string
	DNSEP     *lbutils.DNSLoadBalancerEndpointObject
}


func (t *Target) GetHostName() string {
	if t.Name != "" {
		return t.Name
	}
	return t.IPAddress
}

func (t *Target) GetRecordType() string {
	if t.Name != "" {
		return "CNAME"
	}
	return "A"
}

func (t *Target) GetKey() string {
	if t.DNSEP != nil {
		return t.DNSEP.ObjectName().String()
	}
	return t.GetHostName()
}

func (t *Target) IsValid() bool {
	return t.Name != "" || t.IPAddress != ""
}

func (t *Target) String() string {
	return fmt.Sprintf("target %s(%s)", t.GetRecordType(), t.GetHostName())
}

////////////////////////////////////////////////////////////////////////////////
// Watch Request
////////////////////////////////////////////////////////////////////////////////

type Watch struct {
	DNS        string
	HealthPath string
	StatusCode int
	Targets    []*Target
	Singleton  bool
	DNSLB      *lbutils.DNSLoadBalancerObject
}

func (w *Watch) String() string {
	if w.DNSLB == nil {
		return w.DNS
	}
	return fmt.Sprintf("%s [%s]", w.DNSLB.ObjectName(), w.DNS)
}

func (w *Watch) GetKey() string {
	if w.DNSLB == nil {
		return w.DNS
	}
	return w.DNSLB.ObjectName().String()
}

func (w *Watch) Handle(m *Model) source.DNSStatusUpdate {
	m.Debugf("handle %s", w.DNS)

	ctx:=InactiveContext(m)
	done := NewStatusUpdate(m, w)
	healthyTargets := []*Target{}
	msg := ""
	if len(w.Targets) == 0 {
		ctx.StateInfof(w.DNS, "no endpoints configured for %s", w)
		done.Error(true, fmt.Errorf("no endpoints configured"))
		return nil
	}


	if w.IsHealthy(w.DNS) {
		done.SetHealthy(true)
		ctx = ctx.StateInfof(w.DNS, "%s is healthy", w)
		metrics.ReportLB(w.GetKey(), w.DNS, true)
	} else {
		done.SetHealthy(false)
		ctx = ctx.StateInfof(w.DNS, "%s is NOT healthy", w)
		metrics.ReportLB(w.GetKey(), w.DNS, false)
	}

	if w.Singleton {
		for _, target := range w.Targets {
			mod := m.Check(target)
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
			done.AddActiveTarget(healthyTargets[0])
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
	mod:= m.Apply(healthyTargets...)
	if mod {
		done.SetMessage(fmt.Sprintf("replacing %s with %s", w.DNS, msg))
		logger.Info(done.message)
	} else {
		if !done.HasHealthy() {
			ctx.Infof("no healthy targets found")
			done.Failed(fmt.Errorf("no healthy targets found"))
		} else {
			done.Succeeded()
		}
	}

	return done
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
