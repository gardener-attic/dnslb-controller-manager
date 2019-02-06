package utils

import (
	"fmt"

	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/gardener/controller-manager-library/pkg/resources"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var DNSLoadBalancerEndpointType = (*api.DNSLoadBalancerEndpoint)(nil)

type DNSLoadBalancerEndpointObject struct {
	resources.Object
}

func (this *DNSLoadBalancerEndpointObject) DNSLoadBalancerEndpoint() *api.DNSLoadBalancerEndpoint {
	return this.Data().(*api.DNSLoadBalancerEndpoint)
}

func DNSLoadBalancerEndpoint(o resources.Object) *DNSLoadBalancerEndpointObject {

	if o.IsA(DNSLoadBalancerEndpointType) {
		return &DNSLoadBalancerEndpointObject{o}
	}
	return nil
}

func (this *DNSLoadBalancerEndpointObject) Copy() *DNSLoadBalancerEndpointObject {
	return &DNSLoadBalancerEndpointObject{this.Object.DeepCopy()}
}

func (this *DNSLoadBalancerEndpointObject) Spec() *api.DNSLoadBalancerEndpointSpec {
	return &this.DNSLoadBalancerEndpoint().Spec
}
func (this *DNSLoadBalancerEndpointObject) Status() *api.DNSLoadBalancerEndpointStatus {
	return &this.DNSLoadBalancerEndpoint().Status
}

func (this *DNSLoadBalancerEndpointObject) GetCName() string {
	return this.DNSLoadBalancerEndpoint().Spec.CName
}
func (this *DNSLoadBalancerEndpointObject) GetIPAddress() string {
	return this.DNSLoadBalancerEndpoint().Spec.IPAddress
}

func (this *DNSLoadBalancerEndpointObject) GetLoadBalancerRef() *resources.ClusterObjectKey {
	name := this.DNSLoadBalancerEndpoint().Spec.LoadBalancer
	if name == "" {
		return nil
	}
	key := resources.NewClusterKey(this.GetCluster().GetId(), schema.GroupKind{api.GroupName, api.LoadBalancerResourceKind}, this.GetNamespace(), name)
	return &key
}

func (this *DNSLoadBalancerEndpointObject) Validate() error {
	lbref := this.GetLoadBalancerRef()
	if lbref == nil {
		return fmt.Errorf("no load balancer specified")
	}
	o, err := this.GetCluster().Resources().GetCachedObject(lbref)
	if errors.IsNotFound(err) || (err == nil && o.IsDeleting()) {
		return fmt.Errorf("loadbalancer %q not found", lbref.ObjectName())
	}
	return nil
}

func (this *DNSLoadBalancerEndpointObject) UpdateState(state, msg string, healthy *bool) (bool, error) {
	mod := resources.NewModificationState(this.Object)
	status := this.Status()
	mod.AssureStringPtrValue(&status.State, state)
	if msg == "" {
		mod.AssureStringPtrPtr(&status.Message, nil)

	} else {
		mod.AssureStringPtrPtr(&status.Message, &msg)
	}
	if healthy != nil {
		mod.AssureBoolValue(&status.Healthy, *healthy)
	}
	return mod.Modified, mod.Update()

}
