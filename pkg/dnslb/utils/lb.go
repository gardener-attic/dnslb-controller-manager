package utils

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"

	"github.com/gardener/lib/pkg/controllermanager/controller/reconcile"
	"github.com/gardener/lib/pkg/resources"
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
	mod := reconcile.NewModificationState(this)
	status := this.Status()
	mod.AssureStringPtrValue(&status.State, api.STATE_ERROR)
	if msg == "" {
		mod.AssureStringPtrPtr(&status.Message, nil)

	} else {
		mod.AssureStringPtrPtr(&status.Message, &msg)
	}
	return mod.Modified, mod.Update()
}
