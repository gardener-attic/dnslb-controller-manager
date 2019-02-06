package lb

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/resources"
	"k8s.io/apimachinery/pkg/labels"
)

type LoadBalancers map[resources.ObjectName]resources.ObjectNameSet
type Endpoints map[resources.ObjectName]resources.Object
type Owners map[resources.ObjectName]resources.ObjectName

type State struct {
	controller    controller.Interface
	endpoints     resources.SubObjectCache
}

func LoadBalancer(o resources.Object) resources.ClusterObjectKeySet {
	set:=resources.ClusterObjectKeySet{}
	name:= o.Data().(*api.DNSLoadBalancerEndpoint).Spec.LoadBalancer
	if name=="" {
		return set
	}
	key:=resources.NewClusterKey(o.GetCluster().GetId(), api.LoadBalancerGroupKind, o.GetNamespace(), name)
	return set.Add(key)
}

func NewState(c controller.Interface) *State {
	return &State{
		controller:    c,
		endpoints:     *resources.NewSubObjectCache(LoadBalancer),
	}
}

func (this *State) Setup() {
	this.controller.Infof("setup endpoint cache")

	rscs := this.controller.GetMainCluster().Resources()
	res, _ := rscs.GetByExample(&api.DNSLoadBalancerEndpoint{})

	list, _ := res.ListCached(labels.Everything())
	this.endpoints.Setup(list)
	this.controller.Infof("found %d endpoints for %d load balancers", this.endpoints.SubObjectCount(), this.endpoints.Size())
}

func (this *State) UpdateEndpoint(logger logger.LogContext, e resources.Object) {
	if this.endpoints.RenewSubObject(e) {
		key:=e.ClusterKey()
		logger.Infof("update loadbalancer for %q: %v", key.ObjectName(), this.endpoints.GetOwners(key))
	}
}

func (this *State) RemoveEndpoint(logger logger.LogContext, key resources.ClusterObjectKey) {
	this.endpoints.DeleteSubObject(key)
}

func (this *State) GetEndpointsFor(key resources.ClusterObjectKey) []resources.Object {
	return this.endpoints.GetByKey(key)
}
