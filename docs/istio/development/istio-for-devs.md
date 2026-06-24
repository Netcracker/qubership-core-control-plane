# Istio for Developers

## Table of Contents

- [Istio Ambient Mesh Overview](#istio-ambient-mesh-overview)
- [Istio Distribution](#istio-distribution)
- [Cloud Core with Istio Ambient Mesh](#cloud-core-with-istio-ambient-mesh)
  - [High Level Architecture](#high-level-architecture)
  - [Gateways Mapping and Traffic Flow](#gateways-mapping-and-traffic-flow)
  - [Detailed Deployment Diagram](#detailed-deployment-diagram)
  - [Feature Flag and Rollback](#feature-flag-and-rollback)
  - [Zero-Down-Time Cloud-Core Migration](#zero-down-time-cloud-core-migration)
  - [Egress Gateway](#egress-gateway)
- [Application Migration on Istio](#application-migration-on-istio)
- [Writing Configurations](#writing-configurations)
- [Troubleshooting](#troubleshooting)
  - [Is My Request Served by Istio? Is it mTLS encrypted?](#is-my-request-served-by-istio-is-it-mtls-encrypted)
  - [Is My Pod Participant of Mesh?](#is-my-pod-participant-of-mesh)
  - [How to Trace Request Flow?](#how-to-trace-request-flow)
  - [Request/Watch Stuck After Namespace Enrollment into Mesh](#requestwatch-stuck-after-namespace-enrollment-into-mesh)
  - [Monitoring](#monitoring)

## Istio Ambient Mesh Overview

We use Istio only in Ambient mode, see https://istio.io/latest/docs/ambient

Especially pay attention to https://istio.io/latest/docs/ambient/architecture/traffic-redirection/ document that describes traffic redirection.

## Istio Distribution

https://github.com/Netcracker/qubership-istio

Qubership Istio distribution is just an umbrella helm chart that includes vanilla Istio helm charts as helm dependencies and applies minimal customizations in build time: docker registry URL override, resource profiles, RBAC resources, monitoring dashboards and some default helm values.

It is safe to install Istio distro on cluster with any running apps on it, because Istio will affect connectivity in specific namespace only after this namespace is enrolled into mesh, see https://github.com/Netcracker/qubership-istio/blob/main/docs/public/namespace-enrollment.md

## Cloud Core with Istio Ambient Mesh

### High Level Architecture

We aim to maintain fully backward-compatible Service Mesh behavior on Istio without using the legacy Core Mesh in the end. This means that Cloud Core OOB gateways will become Istio entities, but they will keep their DNS names, header modification behavior, etc.

![istio-current-to-be](./images/istio-current-to-be.drawio.svg)

But removing the legacy service mesh requires all applications to migrate their route configurations to Istio. Until then, for the BWC period we "wrap" our existing service mesh solution with Istio.

![istio-mesh-wrap](./images/istio-mesh-wrap.drawio.svg)

All contract domain names (e.g. the public gateway DNS name) are resolved to Istio gateways, and we provide fallback routes in Istio gateways forwarding traffic to our legacy gateways if no migrated route matched. So APIs migrated to Istio will be served by Istio gateways, while not-yet-migrated APIs will be served by legacy gateways.

This approach is not applied to composite/facade gateways, since such a gateway always belongs to a single application that can be fully migrated to Istio in one deploy session. See [Detailed Deployment Diagram](#detailed-deployment-diagram) for more details.

### Gateways Mapping and Traffic Flow

We require enrolling all namespaces of a composite deployment into the mesh at the same time. So all interactions inside the environment get intercepted by the waypoint by default.

Ingress (public and private) and egress gateways become gateways deployed via k8s Gateway API CRs with `kind: Gateway`. These Istio gateways are excluded from the mesh by Istio design (their deployments have the label `istio.io/dataplane-mode=none`), which means that outgoing traffic from them is not redirected to the waypoint and goes strictly to the target microservice.

All east-west routes (which in the legacy mesh were served by composite/facade gateways and the internal gateway) are served by the Istio waypoint proxy. The `internal-gateway-service` DNS name is still available, but the internal gateway is now emulated by the waypoint (having a fallback route to the legacy internal gateway for the BWC period).

While it is possible to deploy as many different waypoints as you want and map which services are served by which waypoint, we recommend using a single waypoint per namespace - it is a scalable deployment with HPA, so deploying more waypoints is not necessary but will cause a significant resource consumption boost.

![istio-traffic-flow](./images/istio-traffic-flow.drawio.svg)

### Detailed Deployment Diagram

![istio-gws-dpl-diag](./images/istio-deployment-diagram.drawio.svg)

### Feature Flag and Rollback

The deployment parameter `SERVICE_MESH_TYPE` is introduced to switch between the legacy core mesh and Istio.
Possible values:
* `Core` - default value; legacy Service Mesh mode with no Istio resources being deployed;
* `Istio` - enables Istio but keeps legacy OOB Cloud-Core gateways and creates fallback routes from Istio to legacy Core gateways.

### Zero-Down-Time Cloud-Core Migration

ZDT for Cloud-Core OOB gateways migration on Istio is reached via helm hook weights and phases (that get translated to sync-waves by ArgoCD).

Most of the Cloud-Core k8s resource templates for Istio are located in https://github.com/Netcracker/qubership-core-mesh-config
But some of the resources are in the following repos for historical reasons:
* https://github.com/Netcracker/qubership-core-ingress-gateway
* https://github.com/Netcracker/qubership-core-facade-operator

Rollback to the previous state (without Istio) can be achieved by redeploying **all** applications with the feature flag `SERVICE_MESH_TYPE` set to `Core`.

### Egress Gateway

We require the Egress gateway to be managed by Cloud-Core before migrating to Istio. The [facade-operator](https://github.com/Netcracker/qubership-core-facade-operator) with Istio support includes a feature to migrate legacy Cloud-Core egress to an Istio gateway. To avoid collisions of CRs describing legacy egress causing unwanted deletion of legacy egress during migration, a new legacy CR for egress was introduced: TBD: link to CR in ingress-gateway repo

This relates only to the egress gateway deployment, but routes for egress should be migrated by the route configuration owners.

Migration of Core to Istio should come before migration of other apps - otherwise the facade-operator might delete migrated services since it is not yet aware of which resource to delete during migration.

## Application Migration on Istio

In order to help with application migration to Istio we provide:
* migration guide;
* AI skills;
* httproutes-generator maven plugin;
* `SERVICE_MESH_TYPE` feature flag support in all route registration libraries that disables routes registration in legacy control-plane when Istio enabled.

For more details see [Core Mesh to Istio Migration Guide](../migration-guide/core-mesh-to-istio-migration-guide.md).

## Writing Configurations

TBD: routes ordering, EnvoyFilter, forbidden routes

## Troubleshooting
### Is My Request Served by Istio? Is it mTLS encrypted?

If it is a request coming to the public or private gateway, then it will be served by the Istio ingress gateway and gets encrypted in ztunnel. In any other case the outgoing pod decides: if the outgoing pod is a participant of the mesh, then yes; otherwise the request will omit ztunnel and waypoint - no routing rules and no encryption applied.

### Is My Pod Participant of Mesh?

Steps to check:
1. Check labels on namespace:
  * `istio.io/dataplane-mode: ambient` - enables interception by ztunnel for workloads in namespace;
  * `istio.io/use-waypoint: waypoint` - makes ztunnel forward traffic to waypoint for workloads in namespace;
2. Check labels on deployment (set on the pod template, i.e. `spec.template.metadata.labels`) - they may override namespace-level labels for this specific workload:
  * `istio.io/dataplane-mode: ambient` - explicitly includes the pod into the ambient mesh (interception by ztunnel) even if the namespace is not labeled;
  * `istio.io/dataplane-mode: none` - excludes the pod from the ambient mesh even if the namespace is labeled `ambient`;
  * `istio.io/use-waypoint: <waypoint-name>` - makes ztunnel forward this pod's traffic to the named waypoint, overriding the namespace-level waypoint;
  * `istio.io/use-waypoint: none` - disables waypoint usage for this pod (traffic still goes through ztunnel but bypasses any waypoint);
  * `istio.io/use-waypoint-namespace: <namespace>` - selects a waypoint located in a different namespace (used together with `istio.io/use-waypoint`);
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

All gateways (Istio, waypoint, legacy core mesh) have access logs. Ztunnel logs are better checked in a logging aggregator (e.g. graylog) in case the cluster has many worker nodes, since ztunnel is a daemon set.

### Request/Watch Stuck After Namespace Enrollment into Mesh

As mentioned in https://github.com/Netcracker/qubership-istio/blob/main/docs/public/namespace-enrollment.md it is required to restart all workloads in a namespace when enrolling the namespace into the mesh, otherwise long-living sessions will be dropped silently and not all network clients can detect that and re-establish the connection.

### Monitoring

The dashboard provided with the Istio distribution displays metrics on Istio components, gateways HW resource consumption, and traffic statistics.