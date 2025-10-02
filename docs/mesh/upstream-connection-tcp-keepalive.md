# How to configure tcp keepalive for upstream gateway connections

[[_TOC_]]

## Overview

This document describes how tcp keepalive can be configured for gateway upstream connections. 

## TCP Keepalive settings description

**keepalive_probes**

(UInt32Value) Maximum number of keepalive probes to send without response before deciding the connection is dead. Default is to use the OS level configuration (unless overridden, Linux defaults to 9.)

**keepalive_time**

(UInt32Value) The number of seconds a connection needs to be idle before keep-alive probes start being sent. Default is to use the OS level configuration (unless overridden, Linux defaults to 7200s (i.e., 2 hours.)

**keepalive_interval**

(UInt32Value) The number of seconds between keep-alive probes. Default is to use the OS level configuration (unless overridden, Linux defaults to 75s.)

## Mesh CR RouteConfiguration example

Please note, that any other configuration with empty or ommitted `tcpKeepalive` section will delete tcp keepalive settings for the cluster. 

```yaml
apiVersion: core.qubership.org/v1
kind: Mesh
subKind: RouteConfiguration
metadata:
  name: tenant-manager-public-routes
  namespace: cloudbss-kube-core-dev-1
  labels:
    deployer.cleanup/allow: "true"
    app.kubernetes.io/managed-by: saasDeployer
    app.kubernetes.io/part-of: "Cloud-Core"
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  gateways: ["public-gateway-service"]
  tlsSupported: true
  virtualServices:
    - name: public-gateway-service
      hosts: ["*"]
      routeConfiguration:
        version: v1
        routes:
          - destination:
              cluster: tenant-manager
              endpoint: http://tenant-manager-v1:8080
              tlsEndpoint: https://tenant-manager-v1:8443
              tcpKeepalive:
                probes: 3
                time: 30
                interval: 10
            rules:
              - match:
                  prefix: /api/v4/tenant-manager/public-api
                prefixRewrite: /api/v4/api
```
## Mesh CR Cluster example

Please note, that any other configuration with empty or ommitted `tcpKeepalive` section will delete tcp keepalive settings for the cluster. 

Field `spec#name` must contain full cluster key. 
Cluster key can be obtained from Mesh tab in cloud-administrator UI.

```yaml
apiVersion: core.qubership.org/v1
kind: Mesh
subKind: Cluster
metadata:
  name: custom-cluster
  namespace: cloud-core
  labels:
    deployer.cleanup/allow: "true"
    app.kubernetes.io/managed-by: saasDeployer
    app.kubernetes.io/part-of: "Cloud-Core"
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  gateways:
    - private-gateway-service
  name: tenant-manager||tenant-manager||8443
  tls: custom-cert-name
  endpoints:
    - https://tenant-manager:8443
  circuitBreaker:
    threshold:
      maxConnections: 1
  tcpKeepalive:
    probes: 3
    time: 30
    interval: 10 
```

## Control-plane REST API

POST `http://control-plane:8080/api/v3/clusters/tcp-keepalive`

Authorization: M2M, devops-client, CLOUD-ADMIN or key-manager token. 

Request body:

```json
{
  "clusterKey": "tenant-manager||tenant-manager||8443",
  "tcpKeepalive": {
    "probes": 3,
    "time": 30,
    "interval": 10
  }
}  
```

Cluster key can be obtained from Mesh tab in cloud-administrator UI.

## Delete TCP keepalive settings for cluster

To delete tcp keepalive settings use one of the following ways:  
1. apply any of the configurations (RouteConfiguration or Cluster) with empty or ommitted `tcpKeepalive` section. 
2. Perform the following control-plane REST API call:

POST `http://control-plane:8080/api/v3/clusters/tcp-keepalive`

Authorization: M2M, devops-client, CLOUD-ADMIN or key-manager token. 

Request body:

```json
{
  "clusterKey": "tenant-manager||tenant-manager||8443",
  "tcpKeepalive": null
}  
```

Cluster key can be obtained from Mesh tab in cloud-administrator UI.
