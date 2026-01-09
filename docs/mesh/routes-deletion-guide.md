# Routes Deletion guide

This guide describes ways to delete microservice routes.

> :warning: These requests don't delete **clusters** end **endpoints** even if there are no related routes left. Such orphaned **clusters** end **endpoints** must be deleted with separate deletion requests to the following API: 
> Cluster: [Delete cluster by id](../api/control-plane-api.md#delete-cluster-by-id)  
> Endpoint: [Delete endpoints](../api/control-plane-api.md#delete-endpoints-1)

Typical request contains **routes** with **prefix** in each and **namespace** or **version** field. Gateway to delete
route in is specified in **nodeGroup** field or **path**. **namespace** and **version** fields can be omitted - then
these values will be substituted in control-plane.

## Routes deletion using configuration files - Legacy approach

Routes can be deleted using legacy routes-configuration file. Supported formats are json and yaml.

Request contract:
[Apply Configuration](../api/control-plane-api.md#apply-configuration)

All configuration requests must be sent to:

```http request
POST /api/v3/control-plane/apply-config
```

Routes deletion request must have "RoutesDrop" kind. E.g.:

```yaml
apiVersion: nc.core.mesh/v3
kind: RoutesDrop
metadata:
  name: delete-route-yaml-example
  namespace: test-namespace
spec:
  - gateways:
      - facade-gateway-service
    virtualService: test-service
    routes:
      - prefix: /test-route-1
```

This request will delete route with prefix "/test-route-1" and namespace "test-namespace" in virtual service
"test-service" in gateway "facade-gateway-service". Field **namespace** in spec will be replaced with **namespace**
field from namespace.

#### Modern declarative approach
Put in your helm chart CRs with all the contract labels.
```yaml
apiVersion: core.netcracker.com/v1
kind: Mesh
subKind: RoutesDrop
metadata:
  name: delete-route-yaml-example
  namespace: test-namespace
  labels:
    deployer.cleanup/allow: "true"
    app.kubernetes.io/managed-by: saasDeployer
    app.kubernetes.io/part-of: "Cloud-Core"
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  entities:
    - gateways:
        - facade-gateway-service
      virtualService: test-service
      routes:
        - prefix: /test-route-1
```
This CR (Custom Resource) will be applied to Kubernetes and processed by core-operator, which will delete the route with prefix "/test-route-1" for namespace "test-namespace" in virtual service "test-service" in gateway "facade-gateway-service".

## Routes deletion using REST API

### API v1

API v1 requests require port forwarding to control plane. Also, all parameters must be specified as URL parameters.

Next request will delete route with prefix "/test-route" and namespace "test-namespace".

```http request
DELETE /api/v1/routes/{nodeGroup}?from=/test-route&namespace=test-namespace
```

Request contract:
[Delete routes by node group v1](../api/control-plane-api.md#delete-routes-by-node-group)

### API v2

It is possible to delete multiple routes in all microservices in gateway with request like in the following example:

```http request
DELETE /api/v2/control-plane/routes/facade-gateway-service
```

```json
[
  {
    "routes": [
      {
        "prefix": "/test-route-1"
      },
      {
        "prefix": "/test-route-2"
      }
    ],
    "version": "v1"
  }
]
```

This request will delete version "v1" routes with prefixes "/test-route-1" and "/test-route-2" in gateway
"facade-gateway-service".

If no gateway specified in path, then route will be deleted in all gateways.

Request contracts:
[Delete routes v2](../api/control-plane-api.md#delete-routes)
and
[Delete routes by node group v2](../api/control-plane-api.md#delete-routes-by-node-group-1)

It is also possible to delete route by **UUID**:

```http request
DELETE /api/v2/control-plane/routes/uuid/a635ee07-ba06-4160-aea5-eea492d3b51a
```

**UUID** is unique for each route so there will be no collisions. To get route **UUID** see:
[Get virtual service](../api/control-plane-api.md#get-virtual-service).

Request contract:
[Delete routes by UUID](../api/control-plane-api.md#delete-route-by-uuid)

### API v3

API v3 requests allow deletion of virtual service with all routes in it.

> :information_source: Delete virtual service requests will delete related **cluster** and **endpoint** if these have no relations with other routes as a result.

Only gateway and virtual service are required to be specified in **path** to delete all information about virtual
service in gateway:

```http request
DELETE /api/v3/control-plane/routes/facade-gateway-service/test-virtual-service
```

Request contract:
[Delete virtual service](../api/control-plane-api.md#delete-virtual-service)
