package main

import (
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/ingress"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/service"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/lb"

	"github.com/gardener/controller-manager-library/pkg/controllermanager"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/cluster"
	"github.com/gardener/controller-manager-library/pkg/controllermanager/controller/mappings"
)

func main() {
	mappings.Configure().ForController("dnslb-loadbalancer").Map(cluster.DEFAULT, "target")
	controllermanager.Start("dnslb-controller-manager", "dns load balancer controller manager", "nothing")
}
