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

### RouteConfiguration.spec.gateways to HTTPRoute.spec.parentRefs

A priority-ordered procedure of its own — see
[parent-refs-resolution.md](parent-refs-resolution.md).

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

Scan the whole chart for the name before emitting virtual-service-level headers. A CR that declares
none — every header list rule-level — cannot collide and needs no scan. When the scan is not
possible, say so in the report rather than assuming either answer. When more than one
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
— sort on each rule's match value after `regExp` root reduction (`match.prefix` / `match.path` /
the reduced `PathPrefix` value for a former `match.regExp`).

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
                                 See "tlsEndpoint on an egress destination" in
                                 [tls-def-mapping.md](tls-def-mapping.md)
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

#### Endpoint to backendRef resolution

On an egress external destination, resolve the address from `tlsEndpoint` first when it is set —
see [tls-def-mapping.md](tls-def-mapping.md), "tlsEndpoint on an egress destination". The parser
below is otherwise unchanged.


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
                  (filters[] order: RequestHeaderModifier first, then URLRewrite. Gateway API does
                   not order these two, so this is for diffability against a regenerated file)
  removeHeaders   []string           → RequestHeaderModifier remove[] (rule-level, merged with VS-level)
  timeout         *int64             → timeouts.request: "<value>ms"  (value is milliseconds)
  allowed         *bool              → when false then refer to `Forbidden rule processing (allowed: false)`
  idleTimeout     *int64             OMIT  ⚠ flag for MANUAL REVIEW if non-nil
  statefulSession *StatefulSession   → DestinationRule (see [stateful-session-rule-mapping.md](stateful-session-rule-mapping.md))
  rateLimit       string             OMIT  ⚠ flag for MANUAL REVIEW if non-empty
  deny            *bool              → when non-nil (true) refer to `Deny rule processing (deny: true)`
  luaFilter       string             OMIT from HTTPRoute → TrafficExtension in the same pass, see [lua-filter-mapping.md](lua-filter-mapping.md)

---

### Forbidden rule processing (`allowed: false`)

Per [docs/istio/allowed-and-forbidden-routes.md](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md),
Istio's Gateway API has no "allowed but return 404" rule. **Do not emit the rule at all** — no
`backendRefs`-less rule, no direct-response filter. If every rule in a `RouteV3` (or an entire
virtualService) is forbidden, omit that `RouteV3`/virtualService's output entirely.

Instead, record it in the skill's report under `accessControl.forbidden[]`:

- `from`: the rule's match value (post path-type resolution, pre any `regExp` root reduction).
- `cr`: the source `RouteConfiguration`/`Mesh` CR name.
- `methods`: include **only** when the rule's `match.headers[]` contains a matcher on the
  pseudo-header `:method` (i.e. a legacy method restriction); omit the field otherwise.

If the CR's `spec.gateways` includes a border gateway name (`public-gateway-service` —
see [parent-refs-resolution.md](parent-refs-resolution.md) PRIORITY 0), add one
`# ⚠ MANUAL REVIEW` line and a `needsReview:` entry: "Forbidden route `<from>` on a border
gateway — the service's public-gateway AuthorizationPolicy must not allow this path." Rules on a
non-border gateway (egress, mesh) are recorded but not flagged — they have no public-gateway
exposure to protect against.

### Deny rule processing (`deny: true`)

Legacy `deny: true` let a route override an already-allowed route (e.g. one an OOB configuration
allowed) with a forced 404 — see [docs/istio/allowed-and-forbidden-routes.md](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md).
Gateway API has no per-gateway deny rule. **Do not emit the rule at all** — same handling as
`allowed: false`. Always add `# ⚠ MANUAL REVIEW` (this is a narrower, more surprising case than a
plain forbidden route, since it was overriding something already allowed elsewhere).

Record it in the report under `accessControl.deny[]`, using the same fields as `forbidden[]`
(`from`, `cr`, optional `methods`), and a `needsReview:` entry: "Deny route `<from>` — exclude
this path from the service's public-gateway AuthorizationPolicy (it was overriding an
already-allowed OOB route)."

### RouteMatch

  JSON key  Go type          Transformation
  ───────────────────────────────────────────────────────────────────────────────────────
  prefix    string           → path.type: PathPrefix,        value: <prefix>
  path      string           → path.type: Exact,             value: <path>
  regExp    string           → reduce to a PathPrefix root, see `regExp reduction` below
  headers   []HeaderMatcher  → matches[].headers[]

  Path match type — mutually exclusive, apply first non-empty in this priority:
    1. prefix  → PathPrefix
    2. path    → Exact
    3. regExp  → PathPrefix root (never RegularExpression — see below)

#### regExp reduction

Gateway API `RegularExpression` path matches are no longer emitted for `RouteMatch.regExp` (header
matcher `safeRegexMatch` etc. are unaffected — see HeaderMatcher below). Reduce the regex to a
`PathPrefix` root instead, mirroring the root reduction `httproutes-generator-maven-plugin` /
`httproutes-generator-spring-maven-plugin` apply to annotation routes:

1. Strip a leading `^` anchor if present.
2. Cut the pattern at the first regex metacharacter (`(`, `[`, `.`, `*`, `+`, `?`, `{`, `|`, `\`,
   `$`) or templated segment. Everything before the cut, up to the last complete `/`-separated
   segment, becomes the `PathPrefix` value.
3. Apply `prefixRewrite` (if set) as usual — it is independent of this reduction.
4. **Too-general root — flag, do not emit:** if the reduced prefix is exactly `/`, `/api`, `/vN`,
   `/api/vN`, or `/vN/api` (N = one or more digits; case-insensitive; trailing slash ignored), it
   names no API resource or service and would shadow every other route on the gateway it
   attaches to. Do not emit the rule. Add `# ⚠ MANUAL REVIEW` quoting the original `regExp` value,
   and record it exactly like a forbidden route (see `Forbidden rule processing`) but under a
   `needsReview:` entry worded "regExp `<value>` reduces to the too-general prefix `<prefix>` —
   express it as an Exact/PathPrefix match by hand."
5. **Specific root — no flag.** Any other reduced prefix is emitted as a normal `PathPrefix` rule.
   Note the conversion in the chat Output Summary (not `needsReview:`) so the regex-to-prefix
   narrowing/widening is visible, but it does not block the migration.
6. **Merge against an existing rule with the same reduced root:** if another rule in the same
   `RouteV3`/virtualService already emits that exact `PathPrefix` value with the same rewrite and
   backend, drop the duplicate silently (same rule, same target). If the rewrite or backend
   differs, keep both and add `# ⚠ MANUAL REVIEW` — the regex conversion is ambiguous and a human
   must confirm which one is authoritative.

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

---

### Fields that MUST be flagged with `# ⚠ MANUAL REVIEW`

| Source | Field | Trigger |
|---|---|---|
| `RouteConfiguration.spec` | `overridden` | non-empty |
| `VirtualService` | `rateLimit` / `overridden` | non-empty |
| `VirtualService.hosts[]` | `*` host | appears on an east-west (mesh) route |
| `RouteDestination` | `cluster` / `httpVersion` / `circuitBreaker` / `tcpKeepalive` | non-empty; `cluster` is **not** flagged on egress external destinations (used as ServiceEntry name) |
| `VirtualService.name` | reused on the same gateway **and this CR emits virtual-service-level headers** | the reused names carry different `addHeaders` / `removeHeaders`; Core Mesh keeps one list, Istio gives each HTTPRoute its own. A CR whose headers are all rule-level does not fire this |
| `RouteV3.Rule` | `idleTimeout` / `rateLimit` | non-empty / non-nil |
| `RouteV3.Rule` | `deny: true` | always flagged — rule omitted, see `Deny rule processing (deny: true)` |
| `RouteMatch` | `regExp` reducing to a too-general `PathPrefix` root (`/`, `/api`, `/vN`, `/api/vN`, `/vN/api`) | see `regExp reduction` — rule omitted |
| `HeaderMatcher` | `invertMatch: true` or `presentMatch: false` | Gateway API has no negated header match; dropping it widens the route |
| `HeaderMatcher` | `rangeMatch` | numeric range has no Gateway API equivalent |
