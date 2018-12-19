package utils

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/lib/pkg/resources"
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