package lb

import (
	"fmt"
	"net"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/crds"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/lb/watch"
	lbutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"

	"github.com/gardener/external-dns-management/pkg/dns/source"

	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/resources"
	"github.com/gardener/controller-manager-library/pkg/utils"
)

var KEY_STATE = reflect.TypeOf((*State)(nil))

type DNSLBSource struct {
	source.DefaultDNSSource
	controller controller.Interface
	state      *State
	started    time.Time
	nxdomain   net.IP
}

var _ source.DNSSource = &DNSLBSource{}

func NewDNSLBSource(c controller.Interface) (source.DNSSource, error) {
	var clientsets = c.GetMainCluster().Clientsets()

	var ip net.IP
	val, _ := c.GetStringOption(OPT_BOGUS_NXDOMAIN)
	if val != "" {
		ip = net.ParseIP(val)
		if ip != nil {
			c.Warnf("invalid ip address %q configured for bogus nxdomain", val)
		} else {
			c.Infof("using bogus nxdomain address %s", ip)
		}
	}
	err := crds.RegisterCrds(clientsets)
	if err != nil {
		return nil, err
	}
	state := c.GetOrCreateSharedValue(KEY_STATE,
		func() interface{} {
			return NewState(c)
		}).(*State)
	return &DNSLBSource{controller: c, state: state, nxdomain: ip}, nil
}

func (this *DNSLBSource) Setup() {
	this.state.Setup()
}
func (this *DNSLBSource) Start() {
	this.started = time.Now()
}

func (this *DNSLBSource) GetDNSInfo(logger logger.LogContext, obj resources.Object, current *source.DNSCurrentState) (*source.DNSInfo, error) {
	lb := lbutils.DNSLoadBalancer(obj)
	if lb.Spec().DNSName == "" {
		lb.Copy().UpdateState(api.STATE_ERROR, "no dns name specified")
		return nil, fmt.Errorf("no dns name specified")
	}
	targets, done, err := this.GetTargets(logger, obj, current)
	if err != nil {
		return nil, err
	}
	info := &source.DNSInfo{Targets: targets, Feedback: done}
	spec := &obj.Data().(*api.DNSLoadBalancer).Spec
	info.Names = utils.NewStringSet(spec.DNSName)
	info.TTL = spec.TTL
	return info, nil
}

func (this *DNSLBSource) Deleted(logger logger.LogContext, key resources.ClusterObjectKey) {
	logger.Infof("loadbalancer is deleting -> reschedule all endpoint objects")
	for _, o := range this.state.GetEndpointsFor(key) {
		logger.Infof("reschedule endpoint %q", o.ObjectName())
		this.controller.Enqueue(o)
	}
	this.state.RemoveLoadBalancer(key)
	this.DefaultDNSSource.Deleted(logger, key)
}

func (this *DNSLBSource) GetTargets(logger logger.LogContext, obj resources.Object, current *source.DNSCurrentState) (utils.StringSet, source.DNSFeedback, error) {
	now := metav1.Now()
	lb := lbutils.DNSLoadBalancer(obj)

	w, err := watch.NewWatch(logger, lb, current, this.nxdomain)
	if err != nil {
		return nil, nil, err
	}
	for _, o := range this.state.GetEndpointsFor(obj.ClusterKey()) {
		e := lbutils.DNSLoadBalancerEndpoint(o)
		ep := e.DNSLoadBalancerEndpoint()
		t := &watch.Target{IPAddress: ep.Spec.IPAddress, Name: ep.Spec.CName, DNSEP: e}
		if t.IsValid() {
			if now.Time.Before(this.started.Add(3*time.Minute)) || !this.handleCleanup(logger, e, w, &now) {
				w.Targets = append(w.Targets, t)
				logger.Debugf("found %s target '%s' for '%s'", t.GetRecordType(), t.GetHostName(), obj.ObjectName())
			}
		} else {
			logger.Warnf("invalid %s", t)
		}
	}

	set, done := w.Handle()
	return set, done, nil
}

func (this *DNSLBSource) handleCleanup(logger logger.LogContext, e *lbutils.DNSLoadBalancerEndpointObject, w *watch.Watch, threshold *metav1.Time) bool {
	del := false
	ep := e.DNSLoadBalancerEndpoint()
	if ep.Status.ValidUntil != nil {
		if ep.Status.ValidUntil.Before(threshold) {
			del = true
		}
	} else {
		if w == nil {
			del = true
		}
	}
	if del {
		e.Delete()
		if w != nil {
			w.DNSLB.Eventf(corev1.EventTypeNormal, "sync", "dns load balancer endpoint %s/%s deleted", ep.GetNamespace(), ep.GetName())
		}
		logger.Infof("outdated dns load balancer endpoint %s deleted", e.ObjectName())
	}
	return del
}
