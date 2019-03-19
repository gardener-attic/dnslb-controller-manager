package main

import (
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/ingress"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/service"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/lb"
	"github.com/gardener/external-dns-management/pkg/dns/source"

	"github.com/gardener/controller-manager-library/pkg/controllermanager"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/cluster"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/mappings"
)

func main() {
	cluster.Configure("dnstarget", "dnstarget", "target cluster for dns entries").
		Fallback(source.TARGET_CLUSTER).
		Register()
	mappings.Configure().ForController("dnslb-loadbalancer").
		Map(cluster.DEFAULT, source.TARGET_CLUSTER).
		Map(source.TARGET_CLUSTER, "dnstarget").Register()
	controllermanager.Start("dnslb-controller-manager", "dns load balancer controller manager", "nothing")
}
