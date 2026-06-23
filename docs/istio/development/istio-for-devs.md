# Istio for Developers

## Istio Ambient Mesh Overview

We use Istio only in Ambient mode, see https://istio.io/latest/docs/ambient

Especially pay attention to https://istio.io/latest/docs/ambient/architecture/traffic-redirection/ document that describes traffic redirection. 

## Istio Distribution

https://github.com/Netcracker/qubership-istio/tree/main

Qubership Istio distribution is just an umbrella helm chart that includes vanilla Istio helm charts as helm dependencies and applies minimal customizations in build time: docker registry URL override, resource profiles, RBAC resources, monitoring dashboards and some default helm values. 

It is safe to install Istio distro on cluster with any running apps on it, because Istio will affect connectivity in specific namespace only after this namespace is enrolled into mesh, see https://github.com/Netcracker/qubership-istio/blob/main/docs/public/namespace-enrollment.md

## Cloud Core with Istio Ambient Mesh

### High Level Architecture

We target to maintain fully backward-compatible Service Mesh behavior on Istio without using legacy Core Mesh in the end. This means that Cloud Core OOB gateways will become Istio entities but they will keep their dns names, header modification behavior, etc. 

But removing legacy service mesh requires all applications to migrate their route configurations on Istio. Until then, for BWC period we "wrap" our existing service mesh solution with Istio. 

![istio-current-to-be](./images/istio-current-to-be.drawio.svg)

All contract domain names (e.g. public gateway dns name) are resolved to Istio gateways and we provide fallback routes in Istio gateways forwarding traffic to our legacy gateways if no migrated route matched. So migrated to Istio APIs will be served by Istio gateways while not yet migrated APIs will be served by legacy gateways. 

This approach is not applied to composite/facade gateways, since such gateway always belongs to single application that can be fully migrated to Istio in one deploy session. See [Detailed Deployment Diagram](#detailed-deployment-diagram) for more details.

### Gateways Mapping and Traffic Flow

We requrie to enroll all namespaces of composite deployment into mesh at the same time. So all interactions inside the environment got intercepted by waypoint by default. 

Ingress (public and private) and egress gateways become gateways deployed via k8s Gateway API CRs with `kind: Gateway`. These istio gateways excluded from mesh by Istio design (their deployments have label `istio.io/dataplane-mode=none`), which means that outgoing traffic from them is not redirected to waypoint and goes strictly to target micro service.

All east-west routes (which in legacy mesh was served by compoiste/facade gateways and internal gateway) served by istio waypoint proxy. `internal-gateway-service` dns name is still available but internal gateway is now emulated by waypoint (having fallback route to legacy internal gateway for BWC period).

While it is possible to deploy as many different waypoints as you want and map which services served by which waypoint, we recommend to use single waypoint for namespace - it is scalable deployment with HPA so deploying more waypoints is not necessary but will cause significant resource consumption boost.

![istio-traffic-flow](./images/istio-traffic-flow.drawio.svg)

### Detailed Deployment Diagram

![istio-gws-dpl-diag](./images/istio-deployment-diagram.drawio.svg)

### Feature Flag and Rollback

Deployment parameter `SERVICE_MESH_TYPE` introduced to switch between legacy core mesh and Istio. 
Possible values:
* `Core` - default value; legacy Service Mesh mode with no Istio resources being deployed;
* `Istio` - enalbes Istio but keeps legacy OOB Cloud-Core gateways and creates fallback routes from Istio to legacy Core gateways.

### Zero-Down-Time Cloud-Core Migration

ZDT for Cloud-Core OOB gateways migration on Istio is reached via helm hook weights and phases (that get translated to sync-waves by ArgoCD).

Most of the Cloud-Core k8s resource templates for Istio located in https://github.com/Netcracker/qubership-core-mesh-config
But some of the resources are in the following repos for historical reasons:
* https://github.com/Netcracker/qubership-core-ingress-gateway
* https://github.com/Netcracker/qubership-core-facade-operator

Rollback to previous state (without Istio) can be achieved by redeploying **all** applications with feature flag `SERVICE_MESH_TYPE` set to `Core`. 

### Egress Gateway

We require Egress gateway to be managed by Cloud-Core before migrating to Istio. [facade-operator](https://github.com/Netcracker/qubership-core-facade-operator) with Istio support includes feature to migrate legacy Cloud-Core egress on Istio gateway. To avoid collisions of CRs describing legacy egress causing unwanted deletion of legacy egress during migration, new legacy CR for egress was introduced: TBD: link to CR in ingress-gateway repo

This relates only to egress gateway deployment, but routes for egress should be migrated by route configurations owners. 

Migration of Core to Istio should come before migration of other apps - otherwise facade-operator might delete migrated services since it is not yet aware of which resource to delete during migration.

## Application Migration on Istio

In order to help with applications migration on Istio we provide:
* migration guide;
* AI skills;
* httproutes-generator maven plugin;
* `SERVICE_MESH_TYPE` feature flag support in all route registration libraries that disables routes registration in legacy control-plane when Istio enabled.

For more details see [Core Mesh to Istio Migration Gudie](../migration-guide/core-mesh-to-istio-migration-guide.md).

## Writing Configurations

TBD: routes ordering, EnvoyFilter, forbidden routes

## Troubleshooting
### Is My Request Served by Istio? Is it mTLS encrypted?

If it is request coming to public or private gateway, then it will be served by istio ingress gateway and gets encrypted in ztunnel. In any other case outgoing pod decides: if outgoing pod is participant of mesh, then yes; otherwise request will ommit ztunnel and waypoint - no routing rules and no encryption applied.

### Is My Pod Participant of Mesh?

Steps to check:
1. Check labels on namespace:
  * `istio.io/dataplane-mode: ambient` - enables interception by ztunnel for workloads in namespace;
  * `istio.io/use-waypoint: waypoint` - makes ztunnel forward traffic to waypoint for workloads in namespace;
2. Check labels on deployment - they may override namespace-level labels for this specific workload;
3. Go to pod terminal and check ztunnel socket ports 15008, 15006, 15001 via `cat /proc/net/tcp6` or `cat /proc/net/tcp` depending on protocol version:
  ```bash
  $ cat /proc/net/tcp6 | grep 3A
   0: 00000000000000000000000001000000:3ACD 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 889094167 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:3A99 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 889094171 1 0000000000000000 100 0 0 10 0
   2: 00000000000000000000000000000000:3A9E 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 889094170 1 0000000000000000 100 0 0 10 0
   3: 00000000000000000000000000000000:3AA0 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 889094169 1 0000000000000000 100 0 0 10 0
   5: 0000000000000000FFFF00000100007F:3A99 0000000000000000FFFF0000CF99830A:DF6A 01 00000000:00000000 02:00003D4E 00000000     0        0 889111322 2 0000000000000000 21 4 23 10 -1
   7: 0000000000000000FFFF0000CF99830A:3A9E 0000000000000000FFFF0000CE90810A:8B9C 01 00000000:00000000 02:00003F77 00000000     0        0 890852069 2 0000000000000000 20 4 31 50 -1
   9: 0000000000000000FFFF00000100007F:3A99 0000000000000000FFFF0000CF99830A:E010 01 00000000:00000000 02:00004136 00000000     0        0 889119529 2 0000000000000000 22 4 29 10 -1
  18: 0000000000000000FFFF00000100007F:3A99 0000000000000000FFFF0000CF99830A:D724 01 00000000:00000000 02:000013C4 00000000     0        0 890899654 2 0000000000000000 20 4 31 10 -1
  20: 0000000000000000FFFF00000100007F:3A99 0000000000000000FFFF0000CF99830A:99FC 01 00000000:00000000 02:000033BC 00000000     0        0 889110101 2 0000000000000000 20 4 24 10 -1
  22: 0000000000000000FFFF00000100007F:3A99 0000000000000000FFFF0000CF99830A:9A9C 01 00000000:00000000 02:0000236D 00000000     0        0 891024785 2 0000000000000000 20 4 31 10 -1
  27: 0000000000000000FFFF00000100007F:3A99 0000000000000000FFFF0000CF99830A:D38C 01 00000000:00000000 02:000002B5 00000000     0        0 890837365 2 0000000000000000 22 4 29 10 -1
  ```

### How to Trace Request Flow?

All gateways (istio, waypoint, legacy core mesh) have access logs. Ztunnel logs is better to check in logging aggregator (e.g. graylod) in case cluster has many worker nodes since ztunnel is daemon set. 

### Request/Watch Stuck After Namespace Enrollment into Mesh

As mentioned in https://github.com/Netcracker/qubership-istio/blob/main/docs/public/namespace-enrollment.md it is required to restart all workloads in namespace when enrolling namespace into mesh, otherwise long-living sessions will be dropped silently and not all network clients can detect that and re-establish the connection.

### Monitoring

Dashboard provided with Istio distribution display metrics on Istio components and gateways HW resources consumption and traffic statistics. 