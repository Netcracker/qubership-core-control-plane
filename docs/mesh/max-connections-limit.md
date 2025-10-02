# Max Connections Limit

This article covers max connections limit in Service Mesh gateways. 

## Overview

You can configure max connection limit by setting amount of allowed connections for any cluster. All the requests above the allowed limit will be suspended by gateway until previous requests is performed.

Please note, that it works ONLY WITH CUSTOM EGRESS GATEWAYS, because you should configure concurrency(=1) envoy parameter. If you do not set envoy concurrency value 1, behaviour will be unstable, because every limit is applied for every worker(entity which process requests inside envoy), so the worst situation actual limit will be envoy_limit*number_of_workers.
If your custom egress has envoy_concurrency=1, you can set max connection limit for several clusters(You do not have to create several custom egresses).

You should create custom egress gateway by creating facadeService CR with `gatewayType: egress` - see [Description of Facade Operator CR fields](https://github.com/Netcracker/qubership-core-facade-operator/tree/main/README.md#description-of-facade-operator-cr-fields).


## How to set up max connection limit

To set up rate limit you need to apply two configurations: 

### 1. Apply custom egress (facade service)  with envoy_concurrency=1 (spec.env.facadeGatewayConcurrency: 1) (Declarative: put the file deployments/charts/{SERVICE_NAME}/templates or another way apply to Kubernetes)
```yaml
apiVersion: qubership.org/v1alpha
kind: FacadeService
metadata:
  name: custom-egress-gateway
  namespace: "{{ .Values.NAMESPACE }}"
spec:
  port: 8080
  env:
    facadeGatewayMemoryLimit: 128Mi
    facadeGatewayConcurrency: 1
```
### 2. Apply route configuration with connection limit (Declarative: in file deployments/routes-configuration and [apply via control-plane](./development-guide.md#routes-registration-using-configuration-files))
```yaml
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: custom-egress-routes
  namespace: "${ENV_NAMESPACE}"
spec:
  gateways: ["custom-egress-gateway"]
  virtualServices:
    - name: custom-egress-gateway
      routeConfiguration:
        routes:
          - destination:
              cluster: external-cluster
              endpoint: http://external-cluster:8080
              circuitBreaker:
                threshold:
                  maxConnections: 2
            rules:
              - match:
                  prefix: /api/v1/external-service/do
                allowed: true
                prefixRewrite: /api/v1/do
```
## To delete max connection limit, apply same configuration without circuitBreaker or with value circuitBreaker.threshold.maxConnections: 0
```yaml
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: custom-egress-routes
  namespace: "${ENV_NAMESPACE}"
spec:
  gateways: ["custom-egress-gateway"]
  virtualServices:
    - name: custom-egress-gateway
      routeConfiguration:
        routes:
          - destination:
              cluster: external-cluster
              endpoint: http://external-cluster:8080
              circuitBreaker:
                threshold:
                  maxConnections: 0
            rules:
              - match:
                  prefix: /api/v1/external-service/do
                allowed: true
                prefixRewrite: /api/v1/do
```
OR
```yaml
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: custom-egress-routes
  namespace: "${ENV_NAMESPACE}"
spec:
  gateways: ["custom-egress-gateway"]
  virtualServices:
    - name: custom-egress-gateway
      routeConfiguration:
        routes:
          - destination:
              cluster: external-cluster
              endpoint: http://external-cluster:8080
            rules:
              - match:
                  prefix: /api/v1/external-service/do
                allowed: true
                prefixRewrite: /api/v1/do
```
