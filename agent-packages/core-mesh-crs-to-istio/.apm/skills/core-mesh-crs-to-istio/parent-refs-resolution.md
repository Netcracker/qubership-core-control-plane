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

    parentRef type: Gateway + Gateway + Service
    mapping: one-to-many
    condition: `spec.gateways` contains ANY of `public-gateway-service`,
        `private-gateway-service`, `internal-gateway-service`
    reason: per https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md,
        legacy per-gateway visibility is not copied as 404 routes. Private and internal gateways
        still receive the route. The public gateway is added only when the route was already
        public, or when the public-attachment gate below is clear.

    Already public: `spec.gateways` includes `public-gateway-service`. Emit all three parents
    below, regardless of which other border names are also listed.

    Formerly non-public: the border names are only `private-gateway-service` and/or
    `internal-gateway-service`. Emit `private-gateway` and `internal-gateway-service`. Do not
    emit `public-gateway` until the public-attachment gate is clear. An internal-only source
    still gets `private-gateway`, because every route is allowed on the private and internal
    gateways. Do not emit a private-gateway forbid for that route.

    parentRefs emitted for an already-public route (exactly these three):

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
    `public-gateway-service` and one that lists all three produce the same three parents. A CR
    that lists only `private-gateway-service` does not.

    Public-attachment gate (formerly non-public routes only):

    1. Search the chart for every `AuthorizationPolicy` whose `spec.targetRefs` includes Gateway
       `public-gateway`.
    2. An `ALLOW` rule matches the route when a `paths` entry equals the route path or, if the
       entry ends in `*`, covers it as a prefix. An `ALLOW` with no `rules` (Cloud-Core
       deny-unless-permit) matches nothing.
    3. If any in-chart `ALLOW` matches, withhold `public-gateway` even when the caller passed
       `publicGatewayAccess: clear`. Record `needsReview` and name the policy. Attaching the
       route would make it publicly reachable without a service-owned policy. The pinned
       example is section 3a of
       [docs/istio/crs-example.yaml](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/crs-example.yaml).
    4. If the chart has no matching `ALLOW`, access is still unverified. Project and platform
       policies often live outside the chart. Withhold `public-gateway` unless input
       `publicGatewayAccess` is `clear` (the caller checked those policies and confirmed none
       ALLOW the path).
    5. Absence of a service-owned policy is not proof the route is unreachable.
    6. Record every withheld route under `publicAttachment.withheld` in the skill report.

    If `spec.gateways` **also** lists non-border gateways (egress, ingress, mesh), keep them: emit
    the border parents this priority allows **plus** the parentRefs those names resolve to under
    PRIORITY 1-3, without duplicate entries.

    Legacy charts often carry one RouteConfiguration per border gateway with the same paths. After
    they share parents they can produce identical rules. Within one output file, keep such a rule
    once only when parents, normalized match (path, headers, query), filters, timeouts, and
    backend are the same. Across files, leave them — the validation skill reports them as
    duplicates for a human to resolve.

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
