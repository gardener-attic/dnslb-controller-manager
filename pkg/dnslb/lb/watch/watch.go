package watch

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/utils"

	"github.com/gardener/external-dns-management/pkg/dns/source"

	//api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"
	"github.com/gardener/dnslb-controller-manager/pkg/server/metrics"
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
	logger.LogContext
	nxdomain net.IP

	dnsname    string
	HealthPath string
	StatusCode int
	Targets    []*Target
	Singleton  bool
	DNSLB      *lbutils.DNSLoadBalancerObject

	current *source.DNSCurrentState
	updated utils.StringSet
}

func NewWatch(logger logger.LogContext, lb *lbutils.DNSLoadBalancerObject, current *source.DNSCurrentState, nxdomain net.IP) (*Watch, error) {
	singleton, err := IsSingleton(logger, lb)
	if err != nil {
		return nil, err
	}
	spec := lb.Spec()
	return &Watch{
		LogContext: logger,

		dnsname:    spec.DNSName,
		HealthPath: spec.HealthPath,
		Singleton:  singleton,
		StatusCode: spec.StatusCode,
		DNSLB:      lb.Copy(),

		current:  current,
		nxdomain: nxdomain,
	}, nil
}

func (this *Watch) String() string {
	if this.DNSLB == nil {
		return this.dnsname
	}
	return fmt.Sprintf("%s [%s]", this.DNSLB.ObjectName(), this.dnsname)
}

func (this *Watch) GetKey() string {
	if this.DNSLB == nil {
		return this.dnsname
	}
	return this.DNSLB.ObjectName().String()
}

func (this *Watch) check(targets ...*Target) bool {
	set := utils.StringSet{}
	for _, t := range targets {
		set.Add(t.GetHostName())
	}
	return set.Equals(this.current.Targets)
}

func (this *Watch) apply(targets ...*Target) bool {
	set := utils.StringSet{}
	for _, t := range targets {
		set.Add(t.GetHostName())
	}
	this.updated = set
	return !set.Equals(this.current.Targets)
}

func (this *Watch) Get() utils.StringSet {
	return this.updated
}

func (this *Watch) GetDNSState(dnsname string) *source.DNSState {
	return this.current.Names[dnsname]
}

func (this *Watch) Handle() (utils.StringSet, source.DNSFeedback) {
	this.Debugf("handle %s", this.dnsname)

	ctx := InactiveContext(this)
	done := NewStatusUpdate(this)
	healthyTargets := []*Target{}
	if len(this.Targets) == 0 {
		ctx.StateInfof(this.dnsname, "no endpoints configured for %s", this)
		done.Error(true, fmt.Errorf("no endpoints configured"))
		return nil, nil
	}

	ips, err := net.LookupIP(this.dnsname)
	if err != nil || bytes.Equal(ips[0], this.nxdomain) {
		ctx = ctx.StateInfof(this.dnsname, "%s not yet resolvable", this)
		done.SetHealthy(false)
		metrics.ReportLB(this.GetKey(), this.dnsname, false)
	} else {
		if this.IsHealthy(this.dnsname) {
			done.SetHealthy(true)
			ctx = ctx.StateInfof(this.dnsname, "%s is healthy", this)
			metrics.ReportLB(this.GetKey(), this.dnsname, true)
		} else {
			done.SetHealthy(false)
			ctx = ctx.StateInfof(this.dnsname, "%s is NOT healthy", this)
			metrics.ReportLB(this.GetKey(), this.dnsname, false)
		}
	}

	if this.Singleton {
		for _, target := range this.Targets {
			active := this.check(target)
			if this.IsHealthy(target.GetHostName(), this.dnsname) {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), true)
				if len(healthyTargets) == 0 {
					healthyTargets = append(healthyTargets, target)
				}
				done.AddHealthyTarget(target)
				if active {
					healthyTargets[0] = target
					ctx.StateInfof(target.GetHostName(), "healthy active target for %s is %s", this.dnsname, target.GetHostName())
				} else {
					ctx.StateInfof(target.GetHostName(), "target %s is healthy", target.GetHostName())
				}
			} else {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), false)
				done.AddUnhealthyTarget(target)
				if active {
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
			if this.IsHealthy(target.GetHostName(), this.dnsname) {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), true)
				ctx.StateInfof(target.GetHostName(), "target %s is healthy", target.GetHostName())
				done.AddActiveTarget(target)
				done.AddHealthyTarget(target)
				healthyTargets = append(healthyTargets, target)
			} else {
				metrics.ReportEndpoint(this.GetKey(), target.GetKey(), target.GetHostName(), false)
				ctx.StateInfof(target.GetHostName(), "target %s in unhealthy", target.GetHostName())
				done.AddUnhealthyTarget(target)
			}
		}
	}

	mod := this.apply(healthyTargets...)
	if mod {
		done.SetMessage(fmt.Sprintf("replacing targets for %s: %s -> %s", this.dnsname, this.current.Targets, this.updated))
		this.Info(done.message)
	} else {
		if !done.HasHealthy() {
			ctx.Infof("no healthy targets found")
			done.Failed(this.dnsname, fmt.Errorf("no healthy targets found"))
		}
	}

	return this.updated, done
}

func (this *Watch) IsHealthy(hostname string, dns ...string) bool {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	url := fmt.Sprintf("https://%s%s", hostname, this.HealthPath)

	statusCode := this.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	if len(dns) > 0 {
		req.Host = dns[0]
	} else {
		dns = []string{hostname}
	}

	this.Debugf("health check for %q(%q)%s", hostname, dns[0], this.HealthPath)
	resp, err := client.Do(req)
	if err != nil {
		this.Debugf("request failed")
		return false
	}

	resp.Body.Close()
	this.Debugf("found status %d", resp.StatusCode)
	return resp.StatusCode == statusCode
}

func IsSingleton(logger logger.LogContext, lb *lbutils.DNSLoadBalancerObject) (bool, error) {
	singleton := false
	spec := lb.Spec()
	if spec.Singleton != nil {
		singleton = *spec.Singleton
		if spec.Type != "" {
			lb.Copy().UpdateState(api.STATE_ERROR, "invalid load balancer type: singleton and type specified")
			return false, fmt.Errorf("invalid load balancer type: singleton and type specified")
		}
	}
	switch spec.Type {
	case api.LBTYPE_EXCLUSIVE:
		singleton = true
	case api.LBTYPE_BALANCED:
		singleton = false
	case "": // fill-in default
		newlb := lb.Copy()
		if singleton {
			newlb.Spec().Type = api.LBTYPE_EXCLUSIVE
		} else {
			newlb.Spec().Type = api.LBTYPE_BALANCED
		}
		newlb.Spec().Singleton = nil
		logger.Infof("adapt lb type for %s/%s", newlb.GetNamespace(), newlb.GetName())
		newlb.Update()
	default:
		msg := "invalid load balancer type"
		lb.Copy().UpdateState(api.STATE_ERROR, msg)
		return false, fmt.Errorf(msg)
	}
	return singleton, nil
}
