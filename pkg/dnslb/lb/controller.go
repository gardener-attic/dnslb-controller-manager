package lb

import (
	api "github.com/gardener/dnslb-controller-manager/pkg/apis/loadbalancer/v1beta1"
	"github.com/gardener/external-dns-management/pkg/dns/source"
)

var OPT_BOGUS_NXDOMAIN = "bogus-nxdomain"

func init() {
	source.DNSSourceController(source.NewDNSSouceTypeForCreator("dnslb-loadbalancer", api.LoadBalancerGroupKind, NewDNSLBSource), nil).
		RequireLease().
		FinalizerDomain(api.GroupName).
		StringOption(OPT_BOGUS_NXDOMAIN, "ip address returned by DNS for unknown domain").
		Reconciler(StateReconciler, "state").ReconcilerWatch("state", api.GroupName, api.LoadBalancerEndpointResourceKind).
		MustRegister("loadbalancer")
}
