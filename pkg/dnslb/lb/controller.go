package lb

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/lib/pkg/resources"
	"github.com/mandelsoft/dns-controller-manager/pkg/dns/source"
)

var _MAIN_RESOURCE = resources.NewGroupKind(api.GroupName, "DNSLoadBalancer")

func init() {
	source.DNSSourceController(source.NewDNSSouceTypeForCreator("dnslb-dns", _MAIN_RESOURCE, NewDNSLBSource)).
		FinalizerDomain("mandelsoft.org").
		MustRegister("dnssources")
}

