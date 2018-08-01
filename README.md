# DNS Loadbalancer Controller Manager

The *DNS Load Balancer Controller Manager* hosts kubernetes controllers managing
DNS entries acting as kind of load balancer. Depending on health checks on
explicitly maintained endpoints the endpoints are added or removed from an DNS
entry.

It is primarily designed to support multi-cluster loadbalancing (see below)

It defines 3 new resource kinds using the api group `loadbalancer.gardener.cloud`
and version `v1beta1`.
- `DNSProvider`: A resource describing a dedicated DNS access
- `DNSLoadBalancer`: a resource describing a dedicated load balancer defining the DNS name and the health check
- `DNSLoadBalancerEndpoint`: a resource describing a dedicated load balancer target endpoint

The following DNS Provider types are supported so far:
- AWS Route53


## Controllers

The controller manager hosts two different controllers:

### DNS Controller

The DNS Controller uses the resources decribed above to main DNS entries.
Is uses the DNSProvider resources to get access to various DNS accounts with
different hosted zones. The matching provider for the DNS name of an `DNSLoadBalancer` 
is determined automatically. The `DNSLoadBalancerEndpoint`resources are used
as potential targets for the maintained DNS names.

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

The dns controller acts on the second cluster to look for DNS providers,
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
config file (legacy mode). In combination with using the static DNS providers
this mode can be used to work standalone.

If the `--providers` option is used to select a static provider, every
supported DNS provider type may provide a default provider according to
the environment settings. If the value `dynamic` (default) is specified, only
the `DNSProvider` resources found in the target cluster are used.

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
      --cluster string       Cluster identity
      --controllers string   Comma separated list of controllers to start (<name>,source,target,all) (default "all")
      --dry-run              Dry run for DNS controller
      --duration int         Runtime before stop (in seconds)
  -h, --help                 help for dnslb-controller-manager
      --identity string      DNS record identifer (default "GardenRing")
      --interval int         DNS check/update interval in seconds (default 30)
      --kubeconfig string    path to the kubeconfig file
  -D, --log-level string     log level (default "info")
      --once                 only one update instread of loop
      --plugin-dir string    directory containing go plugins for DNS provider types
      --port int             http server endpoint port for health-check (default: 0=no server)
      --providers string     Selection mode for DNS providers (static,dynamic,all,<type name>) (default "dynamic")
      --targetkube string    path to the kubeconfig file for shared virtual cluster
      --ttl int              DNS record ttl in seconds (default 60)
      --watches string       config file for watches

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
 
### DNS Provider

```
apiVersion: loadbalancer.gardener.cloud/v1beta1
kind: DNSProvider
metadata:
  name: aws
  namespace: acme
spec:
  type: aws
  scope: 
    type: Selected  # or Cluster/Namespace
    namespaces:
	  - acme
  secretRef:
    name: route53
```

A provider may only be used for a load balancer resource if it is in the scope
of the provider. The following scopes are supported:

- `Cluster`: (default) valid for all namespaces in kubernetes cluster
- `Namespace`: only valid for the namespace of the provider resource
- `Selected`: valid for the explicitly managed namespace list in property `namespaces`


## Supported DNS Provider Types

For every provider type multiple provider (with different credentials)
may be configured by deploying the appropriate `DNSProvider` resources.

Additional provider types can be added by go plugins (see below).
Plugins are enabled by specifying the `--plugin-dir` option.

### AWS Route53 

The AWS Route53 provider type is selected by using the type name `aws`.

The secret must have the fields:

|Name|Meaning|
|--|--|
|AWS_ACCESS_KEY_ID|The aws access key id|
|AWS_SECRET_ACCESS_KEY|The aws secret access key|

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

## Plugins

Go plugins can be used to add new independently developed DNS provider types.
The plugins must be placed in a dedicated folder, which is specified 
by the `--plugin-dir`  option

A DNS provider type must implement the 
[`DNSProviderType`](pkg/controller/dns/provider/type.go#L27) interface found in package 
`github.com/gardener/dnslb-controller-manager/pkg/controller/dns/provider`.

The main package must provide a variable called `Name` of type `string` containing
the name of the plugin. To register a provider type it has to implement an
`init` function registering the provided provider types. For example:

		func init() {
			provider.RegisterProviderType("aws", &AWSProviderType{})
		}
		
The specified name can then be used in the `DNSProvider` kubernetes resources
to add a dedicated set of hosted zones handled by this provider type.