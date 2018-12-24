package lb

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	lbutils "github.com/gardener/dnslb-controller-manager/pkg/dnslb/utils"
	"github.com/gardener/lib/pkg/controllermanager/controller"
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/resources"
	"k8s.io/apimachinery/pkg/labels"
	"sync"
)

type LoadBalancers map[resources.ObjectName]resources.ObjectNameSet
type Endpoints map[resources.ObjectName]resources.Object
type Owners map[resources.ObjectName]resources.ObjectName

type State struct {
	lock          sync.RWMutex
	controller    controller.Interface
	endpoints     Endpoints
	owners        Owners
	loadbalancers LoadBalancers
}

func NewState(c controller.Interface) *State {
	return &State{
		controller:    c,
		owners:        Owners{},
		endpoints:     Endpoints{},
		loadbalancers: LoadBalancers{},
	}
}

func (this *State) Setup() {
	this.lock.Lock()
	defer this.lock.Unlock()

	this.controller.Infof("setup endpoint cache")

	rscs := this.controller.GetDefaultCluster().Resources()
	res, _ := rscs.GetByExample(&api.DNSLoadBalancerEndpoint{})

	list, _ := res.ListCached(labels.Everything())
	for _, e := range list {
		name := e.ObjectName()
		endp := lbutils.DNSLoadBalancerEndpoint(e)
		lb := resources.NewObjectName(name.Namespace(), endp.Spec().LoadBalancer)
		this.addEndpoint(e, lb)
	}
	this.controller.Infof("found %d endpoints for %d load balancers", len(this.endpoints), len(this.loadbalancers))
}

func (this *State) UpdateEndpoint(logger logger.LogContext, e resources.Object) {
	name := e.ObjectName()
	endp := lbutils.DNSLoadBalancerEndpoint(e)
	lb := resources.NewObjectName(name.Namespace(), endp.Spec().LoadBalancer)

	this.lock.Lock()
	defer this.lock.Unlock()
	old := this.owners[name]
	if old != lb {
		logger.Infof("update loadbalancer for %q : %q -> %q", name, old, lb)
		this.removeEndpoint(name)
		this.addEndpoint(e, lb)
	} else {
		this.endpoints[name] = e
	}
}

func (this *State) RemoveEndpoint(logger logger.LogContext, k resources.ClusterObjectKey) {

	this.lock.Lock()
	defer this.lock.Unlock()
	this.removeEndpoint(k.ObjectName())
}

func (this *State) removeEndpoint(name resources.ObjectName) {
	old := this.owners[name]
	if old != nil {
		set := this.loadbalancers[old]
		set.Remove(name)
		if len(set) == 0 {
			delete(this.loadbalancers, old)
		}
	}
	delete(this.owners, name)
	delete(this.endpoints, name)
}

func (this *State) addEndpoint(obj resources.Object, lb resources.ObjectName) {
	name := obj.ObjectName()
	this.owners[name] = lb
	this.endpoints[name] = obj
	set := this.loadbalancers[lb]
	if set == nil {
		set = resources.ObjectNameSet{}
		this.loadbalancers[lb] = set
	}
	set.Add(name)
}

func (this *State) GetEndpoints(name resources.ObjectName) []resources.Object {
	this.lock.RLock()
	defer this.lock.RUnlock()
	r := []resources.Object{}
	for n := range this.loadbalancers[name] {
		r = append(r, this.endpoints[n])
	}
	return r
}
