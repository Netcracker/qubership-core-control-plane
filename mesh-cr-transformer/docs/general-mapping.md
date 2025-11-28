# Traffic Management Features Mapping

Author: @alsergs

## Feature Mapping Table

| Feature                    | Core Mesh Entity   | Istio Entity                                                                                 | Convertation complexity (easy, medium, hard) | Notes                                                                   | 
|----------------------------|--------------------|----------------------------------------------------------------------------------------------|----------------------------------------------|-------------------------------------------------------------------------|
| Gateway                    | Gateway            | Gateway                                                                                      | easy                                         |                                                                         |
| Route                      | RouteConfiguration | HTTPRoute                                                                                    | hard                                         |                                                                         |
| Stateful Sessions (strong) | StatefulSession    |                                                                                              | medium                                       |                                                                         |
| Stateful Sessions (hash)   | LoadBalance        | DestinationRule (consistentHash)                                                             | medium                                       | not working in ambient mode https://github.com/istio/istio/issues/55259 | 
| Rate Limiting              | RateLimit          | EnvoyFilter                                                                                  | hard                                         | EnvoyFilter has complicated structure                                   | 
| Route Timeouts             | RouteConfiguration | HTTPRoute (upstream), EnvoyFilter (downstream) or Destination Rule::HTTPSettings(downstream) | medium                                       |                                                                         |
| TCP Keepalive              | RouteConfiguration | DestinationRule                                                                              | easy                                         |                                                                         |
| Max Connections            | MaxConnections     | DestinationRule                                                                              | easy                                         |                                                                         |
| TLS certificate for egress | TlsDef             | k8s secret + DestinationRule                                                                 | medium                                       |                                                                         |
| High Code Extensions       | WASMplugin         | On demand                                                                                    |                                              |                                                                         |
|                            | RoutesDrop         | Not applicable                                                                               |                                              |                                                                         |
| CORS                       | ?                  | VirtualService::CorsPolicy, HTTPRoute::HTTPCORSFilter                                        | medium                                       |                                                                         |

Notes:
Configuration levels by envoy model:

- Cluster level
- Endpoint level
- Route level

Need to consider applicable levels for all capabilities

## Detailed Analysis

### 1. Stateful Sessions

#### Stateful Sessions (strong)

**Core Mesh Implementation:**

- Supports multiple configuration levels:
    - Cluster level
    - Endpoint level
    - Route level
- Cookie-based sticky sessions

**Istio Implementation:**
https://tetrate.io/blog/creating-strong-sticky-sessions-with-istio/

1. Add `stateful_session` filter in the http listener
2. Add Service label `istio.io/persistent-session-header: x-session-header`

#### Stateful Sessions (hash)

**Core Mesh Implementation:**

- Applied on endpoint level

**Istio Implementation:**

- Requires combination of resources:
    - `DestinationRule` with `consistentHash` configuration

CAUTION: https://github.com/istio/istio/issues/55259
Sticky Session [ Consistent hash ] Not working with Istio Ambient Mode

HTTPRoute::sessionPersistence is not supported at all

### 2. Rate Limiting

**Core Mesh Implementation:**

- Dedicated `RateLimit` resource

**Istio Implementation:**

- Requires `EnvoyFilter`
- More complex setup with external rate limiting service
- More flexible but requires additional infrastructure

https://istio.io/latest/docs/tasks/policy-enforcement/rate-limit/

**Key Differences:**

- Core Mesh provides simpler, built-in rate limiting
- Core Mesh has more straightforward configuration

### 3. Route Timeouts

**Key Differences:**

- Core Mesh has simpler timeout configuration
- Istio provides more detailed timeout and retry controls
- Both achieve similar basic functionality

### 4. TCP Keepalive

**Core Mesh Implementation:**

- Simple TCP keepalive configuration

**Istio Implementation:**

- Configured in `DestinationRule`

**Key Differences:**

- Both provide essential TCP keepalive functionality

### 5. Max Connections

https://istio.io/latest/docs/tasks/traffic-management/circuit-breaking/

### 6. TLS certificate for egress

Istio example

1. Create secret with certificate

```
# Create a Kubernetes Secret in the same namespace as the egress gateway is deployed in, to hold the client’s certificates
kubectl create secret -n test generic client-credential --from-file=tls.key=client.example.com.key \
  --from-file=tls.crt=client.example.com.crt --from-file=ca.crt=example.com.crt
```

2. Create DestinationRule with TLS configuration

```
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: originate-mtls-for-nginx
spec:
  host: my-nginx.mesh-external.svc.cluster.local
  trafficPolicy:
    loadBalancer:
      simple: ROUND_ROBIN
    portLevelSettings:
    - port:
        number: 443
      tls:
        mode: MUTUAL
        credentialName: client-credential # this must match the secret created earlier to hold client certs
        sni: my-nginx.mesh-external.svc.cluster.local
        # subjectAltNames: # can be enabled if the certificate was generated with SAN as specified in previous section
        # - my-nginx.mesh-external.svc.cluster.local
```

### 7. CORS

VirtualService option
https://istio.io/latest/docs/reference/config/networking/virtual-service/#CorsPolicy

Will VirtualService work along with HTTPRoute ?

HTTPRoute option
https://gateway-api.sigs.k8s.io/reference/spec/?h=cors#httpcorsfilter

HTTPCorsFilter

## Concerns

`DestinationRule` is bound to Service by host field, which contains Service host name. That means, configuration will be
applied on cluster level in envoy. Some features need to be applied on route level.
