## RouteConfiguration.spec.gateways to HTTPRoute.spec.parentRefs

Resolving which Gateway or Service an HTTPRoute attaches to, in priority order. Split out of
[route-configuration-mapping.md](route-configuration-mapping.md), which walks the CR's fields; this
is a decision procedure rather than a field mapping, and is also consulted on its own.

Source field: 
    
    RouteConfiguration.spec.gateways

Target field: 

    HTTProute.spec.parentRefs

Mapping:

  PRIORITY 1 — Platform gateway table:

    parentRef type: Gateway
    mapping: one-to-one 
    condition: gateway is in list of platform Gateways
    parentRef name resolution:
        source name              parentRef name
        public-gateway-service  → public-gateway
        private-gateway-service → private-gateway
        egress-gateway          → egress-gateway

        kind: Gateway
        group: gateway.networking.k8s.io

Example:
```yaml
spec: 
    parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: <platform Gateway name, e.g. public-gateway>
```

  PRIORITY 2 — ingress/egress gateway:

    parentRef type: Gateway
    mapping: one-to-one 
    condition: gateway is in list of discovered ingress/egress Gateways
    parentRef name resolution:
        name = ingress/egress Gateway name

        kind: Gateway
        group: gateway.networking.k8s.io
        name: <gateway metadata.name value>

Example:
```yaml
spec: 
    parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway    
      name: <ingress/egress Gateway name>
```

  PRIORITY 3 — Internal gateway or mesh Gateway:

    parentRef type: Service
    mapping: one-to-many (one parentRef per host entry)
    condition: gateway = `internal-gateway-service` OR gateway is in list of discovered mesh Gateways
    parentRef name resolution:
        normalized host from virtualService.hosts[]

Example:
```yaml
spec:
    parentRefs:
    - kind: Service
      group: ''
      name: <normalized host from virtualService.hosts[0]>
    - kind: Service
      group: ''
      name: <normalized host from virtualService.hosts[1]>
    ...        
```
