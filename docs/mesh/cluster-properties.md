# How to configure envoy cluster properties are propagated by control-plane

[[_TOC_]]

## Overview

This document describes how cluster properties can be configured


max_requests_per_connection
(UInt32Value) Optional maximum requests for a single upstream connection. This parameter is respected by both the HTTP/1.1 and HTTP/2 connection pool implementations. If not specified, there is no limit. Setting this parameter to 1 will effectively disable keep alive.

## Mesh CR Cluster example

Please note, that any other configuration with empty or ommitted `max_requests_per_connection` section will delete this setting for the cluster. 

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
  maxRequestsPerConnection: 20
```