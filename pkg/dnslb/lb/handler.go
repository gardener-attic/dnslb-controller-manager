package lb

import (
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/dnslb-controller-manager/pkg/dnslb/lb/model"
	lbutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/utils"
	"github.com/mandelsoft/dns-controller-manager/pkg/dns/source"
)

var KEY_STATE = reflect.TypeOf((*State)(nil))

type DNSLBSource struct {
	source.DefaultDNSSource
	state *State
	started time.Time
}

func NewDNSLBSource(c controller.Interface) (source.DNSSource, error) {
	state := c.GetOrCreateSharedValue(KEY_STATE,
		func() interface{} {
			return NewState(c)
		}).(*State)
	return &DNSLBSource{state: state}, nil
}

func (this *DNSLBSource) Setup() {
	this.state.Setup()
}
func (this *DNSLBSource) Start() {
	this.started=time.Now()
}


func (this *DNSLBSource) GetDNSInfo(logger logger.LogContext, obj resources.Object, current *source.DNSCurrentState) (*source.DNSInfo, error) {
	targets, done:= this.GetTargets(logger, obj, current)
	info := &source.DNSInfo{Targets: targets, Update: done}
	logger.Infof("GET INFO for %s", obj.ObjectName())
	info.Names = utils.NewStringSet(obj.Data().(*api.DNSLoadBalancer).Spec.DNSName)
	return info, nil
}

func (this *DNSLBSource) GetTargets(logger logger.LogContext, obj resources.Object, current *source.DNSCurrentState) (utils.StringSet, source.DNSStatusUpdate) {
	now := metav1.Now()
	lb := lbutils.DNSLoadBalancer(obj)
	spec := lb.Spec()
	singleton, err:=this.IsSingleton(lb)
	if err!=nil {

	}
	w := &model.Watch{DNS: spec.DNSName,
		HealthPath: spec.HealthPath,
		Singleton:  singleton,
		StatusCode: spec.StatusCode,
		DNSLB:      lb.Copy(),
	}
	for _, o := range this.state.GetEndpoints(resources.NewObjectName(obj.GetNamespace(), obj.GetName())) {
		e := lbutils.DNSLoadBalancerEndpoint(o)
		ep:=e.DNSLoadBalancerEndpoint()
		t := &model.Target{IPAddress: ep.Spec.IPAddress, Name: ep.Spec.CName, DNSEP: e}
		if t.IsValid() {
			if now.Time.Before(this.started.Add(3*time.Minute)) || !this.handleCleanup(logger, e, w, &now) {
				w.Targets = append(w.Targets, t)
				logger.Debugf("found %s target '%s' for '%s'", t.GetRecordType(), t.GetHostName(), obj.ObjectName())
			}
		} else {
			logger.Warnf("invalid %s", t)
		}
	}

	m:=model.NewModel(logger, current)
	done:=w.Handle(m)
	return m.Get(), done
}

func (this *DNSLBSource) IsSingleton(lb *lbutils.DNSLoadBalancerObject) (bool, error) {
	singleton := false
	spec:=lb.Spec()
	if spec.Singleton != nil {
		singleton = *spec.Singleton
		if spec.Type != "" {
			newlb := lb.Copy()
			newlb.Status().State = "Error"
			newlb.Status().Message = "invalid load balancer type: singleton and type specicied"
			newlb.Update()
			return false, fmt.Errorf("invalid load balancer type: singleton and type specicied")
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
		if lb.Status().Message != msg || lb.Status().State != "Error" {
			newlb := lb.Copy()
			newlb.Status().State = "Error"
			newlb.Status().Message = msg
			newlb.Update()
		}
		return false, fmt.Errorf(msg)
	}
	return singleton, nil
}

func (this *DNSLBSource) handleCleanup(logger logger.LogContext, e *lbutils.DNSLoadBalancerEndpointObject, w *model.Watch, threshold *metav1.Time) bool {
	del := false
	ep:=e.DNSLoadBalancerEndpoint()
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
