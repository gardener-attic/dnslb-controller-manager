package utils

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"

	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/utils"

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

func (this *DNSLoadBalancerEndpointObject) GetLoadBalancerRef() resources.ClusterObjectKey {
	return resources.NewClusterKey(this.GetCluster().GetId(),schema.GroupKind{api.GroupName, api.LoadBalancerResourceKind}, this.GetNamespace(),this.DNSLoadBalancerEndpoint().Spec.LoadBalancer)
}

func (this *DNSLoadBalancerEndpointObject) UpdateState(state, msg string, healthy *bool) (bool, error) {
	 return this.Modify(func(data resources.ObjectData) (bool, error) {
		 ep := data.(*api.DNSLoadBalancerEndpoint)
		 mod := utils.ModificationState{}
		 mod.AssureStringPtrValue(&ep.Status.State, state)
		 if msg == "" {
			 mod.AssureStringPtrPtr(&ep.Status.Message, nil)

		 } else {
			 mod.AssureStringPtrPtr(&ep.Status.Message, &msg)
		 }
		 if healthy!=nil {
			 mod.AssureBoolValue(&ep.Status.Healthy, *healthy)
		 }
		return mod.Modified, nil
	})
}