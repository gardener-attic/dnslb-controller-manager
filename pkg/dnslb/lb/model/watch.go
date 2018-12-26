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
	DNSName    string
	HealthPath string
	StatusCode int
	Targets    []*Target
	Singleton  bool
	DNSLB      *lbutils.DNSLoadBalancerObject
}

func (this *Watch) String() string {
	if this.DNSLB == nil {
		return this.DNSName
	}
	return fmt.Sprintf("%s [%s]", this.DNSLB.ObjectName(), this.DNSName)
}

func (this *Watch) GetKey() string {
	if this.DNSLB == nil {
		return this.DNSName
	}
	return this.DNSLB.ObjectName().String()
}

func (this *Watch) Handle(m *Model) source.DNSFeedback {
	m.Debugf("handle %s", this.DNSName)

	ctx:=InactiveContext(m)
	done := NewStatusUpdate(m, this)
	healthyTargets := []*Target{}
	msg := ""
	if len(this.Targets) == 0 {
		ctx.StateInfof(this.DNSName, "no endpoints configured for %s", this)
		done.Error(true, fmt.Errorf("no endpoints configured"))
		return nil
	}

	if this.IsHealthy(this.DNSName) {
		done.SetHealthy(true)
		ctx = ctx.StateInfof(this.DNSName, "%s is healthy", this)
		metrics.ReportLB(this.GetKey(), this.DNSName, true)
	} else {
		done.SetHealthy(false)
		ctx = ctx.StateInfof(this.DNSName, "%s is NOT healthy", this)
		metrics.ReportLB(this.GetKey(), this.DNSName, false)
	}

	if this.Singleton {
		for _, target := range this.Targets {
			mod := m.Check(target)
			if this.IsHealthy(target.GetHostName(), this.DNSName) {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), true)
				if len(healthyTargets) == 0 {
					healthyTargets = append(healthyTargets, target)
				}
				done.AddHealthyTarget(target)
				if !mod {
					healthyTargets[0] = target
					ctx.StateInfof(target.GetHostName(), "healthy active target for %s is %s", this.DNSName, target.GetHostName())
				} else {
					ctx.StateInfof(target.GetHostName(), "target %s is healthy", target.GetHostName())
				}
			} else {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), false)
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

		for _, target := range this.Targets {
			if this.IsHealthy(target.GetHostName(), this.DNSName) {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), true)
				ctx.StateInfof(target.GetHostName(), "target %s is healthy", target.GetHostName())
				done.AddActiveTarget(target)
				done.AddHealthyTarget(target)
				healthyTargets = append(healthyTargets, target)
				msg = fmt.Sprintf("%s %s", msg, target.GetHostName())
			} else {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), false)
				ctx.StateInfof(target.GetHostName(), "target %s in unhealthy", target.GetHostName())
				done.AddUnhealthyTarget(target)
			}
		}
	}


	mod:= m.Apply(healthyTargets...)
	if mod {
		done.SetMessage(fmt.Sprintf("replacing %s with %s", this.DNSName, msg))
		logger.Info(done.message)
	} else {
		if !done.HasHealthy() {
			ctx.Infof("no healthy targets found")
			done.Failed(this.DNSName, fmt.Errorf("no healthy targets found"))
		} else {
			done.Succeeded()
		}
	}

	return done
}

func (this *Watch) IsHealthy(name string, dns ...string) bool {
	var (
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client   = &http.Client{Transport: tr}
		hostname = fmt.Sprintf("https://%s%s", name, this.HealthPath)
	)
	statusCode := this.StatusCode
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
