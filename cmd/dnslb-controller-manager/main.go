package main

import (
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/lb"
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint"

	"github.com/gardener/lib/pkg/controllermanager"
)

func main() {
	controllermanager.Start("dnslb-controller-manager", "dns load balancer controller manager", "nothing")
}
