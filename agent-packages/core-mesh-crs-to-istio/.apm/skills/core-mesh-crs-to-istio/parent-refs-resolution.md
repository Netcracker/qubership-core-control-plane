## RouteConfiguration.spec.gateways to HTTPRoute.spec.parentRefs

Resolving which Gateway or Service an HTTPRoute attaches to, in priority order. Split out of
[route-configuration-mapping.md](route-configuration-mapping.md), which walks the CR's fields; this
is a decision procedure rather than a field mapping, and is also consulted on its own.

Source field: 
    
    RouteConfiguration.spec.gateways

Target field: 

    HTTProute.spec.parentRefs

Mapping:

  PRIORITY 0 — Border gateway group:

    parentRef type: Gateway + Gateway + Service (fixed triple, not one-to-one)
    mapping: one-to-many — a single matching source name expands to all three
    condition: `spec.gateways` contains ANY of `public-gateway-service`,
        `private-gateway-service`, `internal-gateway-service`
    reason: per https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md, the legacy per-gateway visibility
        (PUBLIC reachable from public+private+internal, PRIVATE from private+internal, INTERNAL
        from internal only) is not reproduced on Istio. Every route that used to be registered on
        any one border gateway is now registered on all three.
    parentRefs emitted (always exactly these three, regardless of which border name(s) matched):

```yaml
spec:
    parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: public-gateway
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: private-gateway
    - group: ''
      kind: Service
      name: internal-gateway-service
```

    `internal-gateway-service` never carries a `port` and never uses `virtualService.hosts[]` for
    naming — it is always the fixed Service name above.

    This priority takes effect for the **whole** RouteConfiguration/virtualService once any one
    border name appears in `spec.gateways`; it is not narrowed per-rule. A CR that lists
    `gateways: ["public-gateway-service"]` and one that lists all three produce the identical
    parentRefs output.

    If `spec.gateways` **also** lists non-border gateways (egress, ingress, mesh), keep them: emit
    the border triple **plus** the parentRefs those names resolve to under PRIORITY 1-3, without
    duplicate entries.

    Legacy charts often carry one RouteConfiguration per border gateway with the same paths. After
    widening they would produce identical rules on the same parents. Within one output file, keep
    such a rule once (same parents, match, filters and backend); across files, leave them — the
    validation skill reports them as duplicates for a human to resolve.

  PRIORITY 1 — Platform gateway table (non-border):

    parentRef type: Gateway
    mapping: one-to-one 
    condition: gateway is in list of platform Gateways, and is NOT one of the three border names
        handled by PRIORITY 0
    parentRef name resolution:
        source name              parentRef name
        egress-gateway          → egress-gateway

        kind: Gateway
        group: gateway.networking.k8s.io

Example:
```yaml
spec: 
    parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: egress-gateway
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

  PRIORITY 3 — Mesh Gateway (discovered, not a border name):

    parentRef type: Service
    mapping: one-to-many (one parentRef per host entry)
    condition: gateway is in list of discovered mesh Gateways and is NOT `internal-gateway-service`
        (that name is always handled by PRIORITY 0, never reaches this priority)
    parentRef name resolution:
        normalized host from virtualService.hosts[]
    port: never set.

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
