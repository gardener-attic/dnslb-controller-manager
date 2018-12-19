package main

import (
	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/lb"

	"github.com/gardener/lib/pkg/controllermanager"
)

func main() {
	controllermanager.Start("dnslb-controller-manager", "dns load balancer controller manager", "nothing")
}
