## RouteConfiguration to HTTPRoute
Source:

    apiVersion: core.netcracker.com/v1
    kind: Mesh
    subKind: RouteConfiguration  ← must be present to identify as RouteConfiguration

Target:

    apiVersion: gateway.networking.k8s.io/v1
    kind: HTTPRoute

Input fields → Output fields:

    metadata:

        name: <n>       → used in HTTPRoute.metadata.name. Refer to `HTTPRoute name resolution`
        labels: { ... } → refer to common label resolution rules

    spec:

        namespace         string             OMIT
        gateways          []string           → spec.parentRefs          refer to parentRef resolution
        listenerPort      int                → spec.parentRef[].port   
        tlsSupported      bool               ignore
        virtualServices   []VirtualService   → one HTTPRoute per entry
        overridden        bool               OMIT ⚠ flag for MANUAL REVIEW if non-empty

### HTTPRoute name resolution

  Single virtualService in Mesh CR:
    
    HTTPRoute.metadata.name = Mesh CR metadata.name

  Multiple virtualServices in Mesh CR:
    
    HTTPRoute.metadata.name = Mesh CR metadata.name + "-" + virtualService.name

###  RouteConfiguration.spec.gateways to HTTPRoute.spec.parentRefs resolution (priority order)

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
---

### VirtualService

  JSON key            Go type              Transformation
  ────────────────────────────────────────────────────────────────────────────────
  name                string               → HTTPRoute name suffix when multiple VSes exist
  hosts               []string             → parentRef Service names (mesh routes)
  rateLimit           string               OMIT  ⚠ flag for MANUAL REVIEW if non-empty
  addHeaders          []HeaderDefinition   → RequestHeaderModifier filter add[] (virtualService-level)
  removeHeaders       []string             → RequestHeaderModifier filter remove[] (virtualService-level)
  routeConfiguration  RouteConfig          → HTTPRoute rules[]
  overridden          bool                 OMIT  ⚠ flag for MANUAL REVIEW if non-empty

#### One virtualService name, several RouteConfigurations

Core Mesh keys a virtual host on `(gateway, virtualServices[].name)`, so every RouteConfiguration
that reuses a name on the same gateway contributes routes to one Envoy virtual host, and the
virtual-host-level `addHeaders` / `removeHeaders` of those CRs collapse into a single list —
one wins and the others are silently dropped. Charts hit this on `egress-gateway`, where several
RouteConfigurations conventionally use the same `egress-gw` virtual service.

Istio has no shared object: each RouteConfiguration becomes its own HTTPRoute and carries its own
copy of that CR's virtual-service-level headers, so after migration every CR's headers apply to its
own routes. A header the collapse used to discard starts being applied.

Scan the whole chart for the name before emitting virtual-service-level headers. When more than one
RouteConfiguration on the same gateway declares it with a different `addHeaders` / `removeHeaders`,
emit each HTTPRoute's own list and add `# ⚠ MANUAL REVIEW` recording that Core Mesh applied only one
of them. Rule-level headers are unaffected — they belong to a single route in both meshes.


### HTTPRoute.spec.hostnames resolution

Omit `hostnames` field in HTTPRoute.spec

#### Host normalization

  IF host = `*` - ignore it, do not propagate to spec.hostnames[]. If `*` occurs in east-west Route - MANUAL REVIEW required
  ELSE IF host contains ".":
    result = host.split(".")[0]
    e.g. "my-svc.namespace"    → "my-svc"
    e.g. "my-svc.ns:8080"     → "my-svc"
  ELSE:
    result = host unchanged
    e.g. "{{ .Values.SERVICE }}" → "{{ .Values.SERVICE }}"

---

### RouteConfig

  JSON key  Go type     Transformation
  ──────────────────────────────────────────────────
  version   string      OMIT
  routes    []RouteV3   → flatten all rules into HTTPRoute rules[]

After flattening, sort the resulting `rules[]` by path specificity using the
shared procedure in
[`path-specificity-sorting.md`](../path-specificity-sorting/SKILL.md)
— sort on each rule's match value (`match.prefix` / `match.path` / `match.regExp`).

---

### RouteV3

  JSON key     Go type           Transformation
  ──────────────────────────────────────────────────────────────────────────
  destination  RouteDestination  → backendRefs[] shared by all rules in this RouteV3
  rules        []Rule            → one HTTPRoute.Rule Rule entry

---

### RouteDestination

  JSON key       Go type         Transformation
  ──────────────────────────────────────────────────────────────────────────────────
  endpoint       string          → parse host + port for HTTPRoute.spec.rules[].backendRefs[].name and .port
  cluster        string          ignore for in-cluster backends; egress external
                                 destinations use it as ServiceEntry metadata.name
                                 (see [tls-def-mapping.md](tls-def-mapping.md))
  tlsSupported   bool            ignore
  tlsEndpoint    string          egress external destination: parse host and port from it
                                 instead of `endpoint` when non-empty, and ⚠ MANUAL REVIEW.
                                 In-cluster destination: ignore, no flag.
                                 See "tlsEndpoint on an egress destination" below
  httpVersion    *int32          OMIT ⚠ flag for MANUAL REVIEW if non-empty
  tlsConfigName  string          ignore for in-cluster backends; on an egress
                                 gateway this selects a cluster-level TlsDef
                                 (see [tls-def-mapping.md](tls-def-mapping.md))
  circuitBreaker CircuitBreaker  OMIT ⚠ flag for MANUAL REVIEW if non-empty
  tcpKeepalive   *TcpKeepalive   OMIT ⚠ flag for MANUAL REVIEW if non-empty

When this RouteConfiguration attaches to a resolved **egress** gateway, resolve each
destination with [tls-def-mapping.md](tls-def-mapping.md) **before** the in-cluster
parser below. That mapping emits ServiceEntry, Secret, DestinationRule, Hostname
`backendRef`, and Host rewrite for `https://` / `tlsConfigName` / FQDN endpoints.

#### tlsEndpoint on an egress destination

`tlsEndpoint` is a supported way to give an egress route its HTTPS address — the RoutingV3 validator
accepts it and validates it against the egress gateway's own address rules. Core Mesh picks between
the two addresses at request-registration time, not in the CR:

```go
// services/route/registration/v3.go
if tlsSupported && tlsmode.GetMode() == tlsmode.Preferred && destination.TlsEndpoint != "" {
    return destination.TlsEndpoint
}
return destination.Endpoint
```

So the same CR resolves to `endpoint` or `tlsEndpoint` depending on whether the control plane runs
with internal TLS enabled. A chart cannot reproduce that, so the migration has to commit to one.

**On an egress external destination, take `tlsEndpoint` when it is non-empty.** It is the address
Core Mesh uses whenever internal TLS is on, and the migrated route originates TLS through the
DestinationRule either way, so the TLS address is the one that stays correct. Parse its host and
port for the ServiceEntry, the `Hostname` backendRef and the `URLRewrite` hostname, exactly as the
parser below does for `endpoint`.

Ignoring it instead produces a route that looks converted and is not: with
`endpoint: http://ext.example.com` and `tlsEndpoint: https://ext.example.com:8443`, reading only
`endpoint` yields a ServiceEntry on port 80 with `protocol: HTTP` and no origination, while Core Mesh
was reaching `:8443` over TLS.

Flag it regardless, because the choice is not free: a deployment running with core TLS **disabled**
uses the plain `endpoint`, and always taking the TLS address changes that. The reviewer confirms
which the target environment runs.

For an in-cluster destination, keep ignoring `tlsEndpoint` and raise no flag. It exists for
Core Mesh's internal TLS, which Istio replaces with mesh mTLS, so the plain endpoint is equivalent.

#### Endpoint to backendRef resolution

Endpoint parsing — pattern: http://<name>:<port>
    
    name: everything between "http://" and last ":"
          preserve Helm expressions exactly
          e.g. http://{{ .Values.DEPLOYMENT_RESOURCE_NAME }}:8080
               → name: "{{ .Values.DEPLOYMENT_RESOURCE_NAME }}"
          e.g. http://public-gateway-service:8080
               → name: "public-gateway-service"
    port: number after last ":"
          e.g. :8080 → 8080

Examples:

    "http://{{ .Values.DEPLOYMENT_RESOURCE_NAME }}:8080" → host="{{ .Values.DEPLOYMENT_RESOURCE_NAME }}", port=8080
    "http://public-gateway-service:8080"                 → host="public-gateway-service",                 port=8080
    "my-service:9090"                                    → prepend http:// → host="my-service",            port=9090

Always set `weight: 1` for every backendRef

Output:

```yaml
  backendRefs:
  - group: ''
    kind: Service
    name: <hostname from destination.endpoint>
    port: <port from destination.endpoint>
    weight: 1
```

---

### Rule

  JSON key        Go type            Transformation
  ────────────────────────────────────────────────────────────────────────────────────────
  match           RouteMatch         → matches[] (see RouteMatch below)
  prefixRewrite   string             → URLRewrite filter path.ReplacePrefixMatch (when non-empty)
  hostRewrite     string             → URLRewrite filter hostname (when non-empty).
                                       Egress external destinations always set hostname
                                       to the endpoint host even if this field is empty
                                       — see [tls-def-mapping.md](tls-def-mapping.md)
  addHeaders      []HeaderDefinition → RequestHeaderModifier add[] (rule-level, merged with VS-level)
  removeHeaders   []string           → RequestHeaderModifier remove[] (rule-level, merged with VS-level)
  timeout         *int64             → timeouts.request: "<value>ms"  (value is milliseconds)
  allowed         *bool              → when false then refer to `Not allowed rule processing`
  idleTimeout     *int64             OMIT  ⚠ flag for MANUAL REVIEW if non-nil
  statefulSession *StatefulSession   → DestinationRule (see [stateful-session-rule-mapping.md](stateful-session-rule-mapping.md))
  rateLimit       string             OMIT  ⚠ flag for MANUAL REVIEW if non-empty
  deny            *bool              OMIT  ⚠ flag for MANUAL REVIEW if non-nil
  luaFilter       string             OMIT from HTTPRoute → TrafficExtension in the same pass, see [lua-filter-mapping.md](lua-filter-mapping.md)

---

### Not allowed rule processing
When Rule.allowed is false - omit `backendRefs` field for it. This will force istio to return 404 for matched path

### RouteMatch

  JSON key  Go type          Transformation
  ───────────────────────────────────────────────────────────────────────────────────────
  prefix    string           → path.type: PathPrefix,        value: <prefix>
  path      string           → path.type: Exact,             value: <path>
  regExp    string           → path.type: RegularExpression, value: <regexp>
  headers   []HeaderMatcher  → matches[].headers[]

  Path match type — mutually exclusive, apply first non-empty in this priority:
    1. prefix  → PathPrefix
    2. path    → Exact
    3. regExp  → RegularExpression

---

### HeaderMatcher

  JSON key        Go type    Transformation
  ────────────────────────────────────────────────────────────────────────────────
  name            string     → matches[].headers[].name
  exactMatch      string     → matches[].headers[].value; omit `type`, since Exact is the
                               Gateway API default and every example here omits it
  value           string     → same as exactMatch (legacy alias)
  safeRegexMatch  string     → type: RegularExpression, value verbatim
  prefixMatch     string     → type: RegularExpression, value `<escaped>.*`
  suffixMatch     string     → type: RegularExpression, value `.*<escaped>`
  presentMatch    bool       true  → type: RegularExpression, value `.*`
                              false → OMIT ⚠ flag (means "header absent", see Inversion)
  rangeMatch      RangeMatch OMIT ⚠ flag for MANUAL REVIEW if start or end is set
  invertMatch     bool       not a matcher — see Inversion below

Source YAML may use `match.headerMatchers` (docs) or `match.headers` (API json tag).
Treat both as this list.

#### Which specifier wins

Core Mesh sets exactly one specifier, in this order, and ignores the rest:

```text
suffixMatch → safeRegexMatch → rangeMatch → presentMatch → prefixMatch → exactMatch
```

Follow the same order when a CR sets more than one, so the converted route matches what the
original did rather than what the YAML appears to say.

#### Regular expressions are full matches

Core Mesh emits Envoy `safe_regex`, which evaluates as an RE2 **full** match, and Istio compiles
Gateway API `RegularExpression` to the same. Two consequences:

- `safeRegexMatch` carries over verbatim — the semantics are identical, no anchoring needed.
- A synthesized prefix must be `<value>.*` and a suffix `.*<value>`. A bare `^<value>` matches
  nothing under full-match semantics, and the route would silently stop matching.

Escape RE2 metacharacters in the literal before synthesizing: `prefixMatch: v1.2` becomes
`v1\.2.*`, not `v1.2.*`, which would also match `v1x2`.

#### Inversion

`invertMatch` is a modifier on whichever specifier is set, not a specifier of its own — Core Mesh
builds the matcher and then negates it. Gateway API has no negated header match: `HTTPHeaderMatch`
offers only `Exact` and `RegularExpression`, with no inversion field. Istio's own `VirtualService`
has `withoutHeaders`, but that is not Gateway API and not what this mapping emits.

So `invertMatch: true` makes the whole matcher unconvertible whatever else it sets: OMIT the header
match and `# ⚠ MANUAL REVIEW`. The same applies to `presentMatch: false`, which is inversion by
another name.

Dropping an inverted matcher **widens** the route — it will match requests the original excluded —
so the flag has to be acted on rather than noted.

---

### HeaderDefinition  (used in VirtualService.addHeaders and Rule.addHeaders)

  JSON key  Go type  Transformation
  ──────────────────────────────────────────────────────────────────────
  name      string   → RequestHeaderModifier.add[].name
  value     string   → RequestHeaderModifier.add[].value

---

### StatefulSession  (rule-level — generates DestinationRule)

  version, namespace, cluster, hostname, gateways, port,
  enabled, cookie, route, overridden

  → See [stateful-session-rule-mapping.md](stateful-session-rule-mapping.md).
  → A DestinationRule is generated for the route's destination host.
  → The DestinationRule is written after the HTTPRoute in the same output file (`---` separator).
