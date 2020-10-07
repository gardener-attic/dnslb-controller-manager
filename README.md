# DNS Loadbalancer Controller Manager

[![reuse compliant](https://reuse.software/badge/reuse-compliant.svg)](https://reuse.software/)

The *DNS Load Balancer Controller Manager* hosts kubernetes controllers managing
DNS entries acting as kind of load balancer. Depending on health checks on
explicitly maintained endpoints the endpoints are added or removed from an DNS
entry. In order words it acts as a DNS source controller, and the DNS entries are
provisioned to an external DNS server with the help of a separately running DNS provisioning
controller. See project [external-dns-management](https://github.com/gardener/external-dns-management) for more details.

It is primarily designed to support multi-cluster loadbalancing (see below)

It defines 2 new resource kinds using the api group `loadbalancer.gardener.cloud`
and version `v1beta1`.
- `DNSLoadBalancer`: a resource describing a dedicated load balancer defining the DNS name and the health check
- `DNSLoadBalancerEndpoint`: a resource describing a dedicated load balancer target endpoint


## Controllers

The controller manager hosts two different controllers:

### DNS Controller

The DNS Controller uses the resources described above to main DNS entries.
The `DNSLoadBalancerEndpoint`resources are used as potential targets for the 
maintained DNS names.

### DNS Endpoint Controller

The endpoint controller scans a cluster for annotated service and ingress resources
looking for the annotation

			loadbalancer.gardener.cloud/dnsloadbalancer
			
expecting the name of the load balancer resource as value.
For those matching resources it maintains endpoint resources (mentioned above).

## Multi Cluster Mode

Basically both controllers can work on the same cluster. This would be a single
cluster scenario. But for such a scenario the introduction of explicitly maintained
loadbalancer and endpoint resources would be superfluous.

The intended scenario is a multi-cluster scenario, where the various endpoints
reside in different clusters. Therefore the two controllers may use different 
target clusters for scanning.

If two kubeconfigs are configured for the controller manager, the endpoint
controller scans the default (source) cluster for service and ingress resources
and expects the load balancer and endpoint resources to be maintained in the
second cluster.

The dns controller acts on the second cluster to look for
loadbalancers and endpoints to maintain the desired DNS entries.

This second cluster should be shared among the various source clusters to
maintain a central loadbalancing datasource. 

For every source cluster the complete controller manager is deployed varying 
the first cluster access for the local cluster and using the second cluster
access for the shared one.

## Leases

The controllers request leases in the different clusters, therefore they can be
run multiple times across the involved clusters.

## Run Modes

The controller manager can be started for a single kind of controller or for
both controllers at once. Nevertheless the DNS controller always requests
its lease from the shared cluster. Therefore it effectivly runs only once
in the complete landscape, even if started with each controller manager instance.

If the `--watches` option is used, the DNS controller doesn't use the custom
resources for the load balancer but reads the definitions from the given
config file (legacy mode).

## Command Line Interface

```
This manager manages DNS LB endpoint resources for DNS Loadbalancer
resources based on annotations in services and ingresses. Based on
those endpoints a second controller manages DNS entries. The endpoint
sources may reside in different kubernetes clusters than the one
hosting the DNS loadbalancer and endpoint resources.

Usage:
  dnslb-controller-manager [flags]

Flags:
      --bogus-nxdomain string                            default for all controller "bogus-nxdomain" options
  -c, --controllers string                               comma separated list of controllers to start (<name>,source,target,all) (default "all")
      --dnslb-endpoint.endpoints.pool.size int           worker pool size for pool endpoints of controller dnslb-endpoint
      --dnslb-loadbalancer.bogus-nxdomain string         ip address returned by DNS for unknown domain
      --dnslb-loadbalancer.default.pool.size int         worker pool size for pool default of controller dnslb-loadbalancer
      --dnslb-loadbalancer.exclude-domains stringArray   excluded domains
      --dnslb-loadbalancer.key string                    selecting key for annotation
      --dnslb-loadbalancer.target-name-prefix string     name prefix in target namespace for cross cluster generation
      --dnslb-loadbalancer.target-namespace string       target namespace for cross cluster generation
      --dnslb-loadbalancer.targets.pool.size int         worker pool size for pool targets of controller dnslb-loadbalancer
      --exclude-domains stringArray                      default for all controller "exclude-domains" options
  -h, --help                                             help for dnslb-controller-manager
      --key string                                       default for all controller "key" options
      --kubeconfig string                                default cluster access
      --kubeconfig.id string                             id for cluster default
  -D, --log-level string                                 logrus log level
  -n, --namespace-local-access-only                      enable access restriction for namespace local access only
      --plugin-dir string                                directory containing go plugins
      --pool.size int                                    default for all controller "pool.size" options
      --server-port-http int                             directory containing go plugins
      --target string                                    target cluster for dns requests
      --target-name-prefix string                        default for all controller "target-name-prefix" options
      --target-namespace string                          default for all controller "target-namespace" options
      --target.id string                                 id for cluster target
```

## Custom Resource Definitions

### DNS Load Balancer

```
apiVersion: loadbalancer.gardener.cloud/v1beta1
kind: DNSLoadBalancer
metadata:
  name: test
  namespace: acme
spec:
  DNSName: test.acme.com
  type: Balanced # or Exclusive
  healthPath:  /healthz
  statusCode: 200 # default
  endpointValidityInterval: 5m # Optional
status:
  active:
    - ipaddress: "172.18.117.33"
      name: "a-test-service"
  state: healthy
  message:
```

If the optional endpoint validity interval is specified, the endpoint
controller generates endpoints with a limited lifetime, and updates
it accordingly as long as it is running. The dns controller automatically
discards outdated endpoint resources.

### DNS Load Balancer Endpoint

```
apiVersion: loadbalancer.gardener.cloud/v1beta1
kind: DNSLoadBalancerEndpoint
metadata:
  name: a-test-service
  namespace: acme
spec:
  ipaddress: 172.18.117.33 # or cname
  loadbalancer: test
status:
  active: true
  healthy: true
  validUntil: 2018-07-24T11:34:44Z
```

The `validUtil` status property is managed by the
endpoint controller, if the loadbalancer resource requests it
by specifying a validity interval for endpoints.
 
## HTTP Endpoints

If the controller manager is called with the `--port` option using a value larger
than zero an https server is started serving two endpoints:

### Health Endpoint

A health endpoint with path `/healthz` is provided at the given port.
It reports status code 200 if everything looks fine. The timestamps of the
internal check keys are reported as content.

### Metrics Endpoint

A metrics endpoint (for prometheus) is provided with the path `/metrics` .
It supports five metrics:

|Metric|Label|Meaning|
|------|-----|-------|
|`endpoint_health`| | Health status of an endpoint (0/1) |
| |`loadbalancer`| Load balancer name |
| |`endpoint`| Endpoint name |
|`endpoint_active`| | Active status of an endpoint (assigned to DNS entry) |
| |`loadbalancer`| Load balancer name |
| |`endpoint`| Endpoint name |
|`endpoint_hosts`| | Hostname for an endpoint resource with health status |
| |`endpoint`| Endpoint name |
| |`host`| Hostname |
|`loadbalancer_health`| | Health status of a load balancer (0/1) |
| |`loadbalancer`| Load balancer name |
|`loadbalancer_dnsnames`| | DNS names of a load balancer with health status |
| |`loadbalancer`| Load balancer name |dns_reconcile_duration
| |`dnsname`| DNS name of the load balancer |
| `dns_reconcile_duration` | | Duration of a DNS reconcilation run |
| `dns_reconcile_interval` | | Duration between two DNS reconcilations |
