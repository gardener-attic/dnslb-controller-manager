package utils

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"

	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/utils"
)

var DNSLoadBalancerType = (*api.DNSLoadBalancer)(nil)

type DNSLoadBalancerObject struct {
	resources.Object
}

func (this *DNSLoadBalancerObject) DNSLoadBalancer() *api.DNSLoadBalancer {
	return this.Data().(*api.DNSLoadBalancer)
}

func DNSLoadBalancer(o resources.Object) *DNSLoadBalancerObject {

	if o.IsA(DNSLoadBalancerType) {
		return &DNSLoadBalancerObject{o}
	}
	return nil
}
func (this *DNSLoadBalancerObject) Copy() *DNSLoadBalancerObject {
	return &DNSLoadBalancerObject{this.Object.DeepCopy()}
}

func (this *DNSLoadBalancerObject) Spec() *api.DNSLoadBalancerSpec {
	return &this.DNSLoadBalancer().Spec
}
func (this *DNSLoadBalancerObject) Status() *api.DNSLoadBalancerStatus {
	return &this.DNSLoadBalancer().Status
}

func (this *DNSLoadBalancerObject) GetDNSName() string {
	return this.DNSLoadBalancer().Spec.DNSName
}

func (this *DNSLoadBalancerObject) UpdateState(state, msg string) (bool, error) {
	return this.Modify(func(data resources.ObjectData) (bool, error) {
		lb := data.(*api.DNSLoadBalancer)
		mod := utils.ModificationState{}
		mod.AssureStringPtrValue(&lb.Status.State, api.STATE_ERROR)
		if msg == "" {
			mod.AssureStringPtrPtr(&lb.Status.Message, nil)

		} else {
			mod.AssureStringPtrPtr(&lb.Status.Message, &msg)
		}
		return mod.Modified, nil
	})
}

