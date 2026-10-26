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

Before emitting, every rule goes through the
[regex-routes-migration](../regex-routes-migration/SKILL.md) worksheet (see
[Worksheet row](#worksheet-row)); its `emit` column decides whether and how the
rule is rendered.

After flattening, sort the resulting `rules[]` by path specificity using the
shared procedure in
[`path-specificity-sorting.md`](../path-specificity-sorting/SKILL.md)
— sort on the path value each rule emits (after the cut).

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
  prefixRewrite   string             → URLRewrite filter path.ReplacePrefixMatch (when non-empty);
                                       the value is the worksheet `R` (for a path without
                                       variables `R` = prefixRewrite). Exact rules from
                                       regex-routes-migration Step 4 use ReplaceFullPath
  hostRewrite     string             → URLRewrite filter hostname (when non-empty).
                                       Egress external destinations always set hostname
                                       to the endpoint host even if this field is empty
                                       — see [tls-def-mapping.md](tls-def-mapping.md)
  addHeaders      []HeaderDefinition → RequestHeaderModifier add[] (rule-level, merged with VS-level)
                  (filters[] order: RequestHeaderModifier first, then URLRewrite. Gateway API does
                   not order these two, so this is for diffability against a regenerated file)
  removeHeaders   []string           → RequestHeaderModifier remove[] (rule-level, merged with VS-level)
  timeout         *int64             → timeouts.request: "<value>ms"  (value is milliseconds)
  allowed         *bool              → when false then refer to `Not allowed rule processing`
                                       (absent or true → allowed)
  idleTimeout     *int64             OMIT  ⚠ flag for MANUAL REVIEW if non-nil
  statefulSession *StatefulSession   → DestinationRule (see [stateful-session-rule-mapping.md](stateful-session-rule-mapping.md))
  rateLimit       string             OMIT  ⚠ flag for MANUAL REVIEW if non-empty
  deny            *bool              OMIT  ⚠ flag for MANUAL REVIEW if non-nil
  luaFilter       string             OMIT from HTTPRoute → TrafficExtension in the same pass, see [lua-filter-mapping.md](lua-filter-mapping.md)

---

### Not allowed rule processing

A rule with `allowed: false` is a forbidden row (`forbidden: explicit`) in the
worksheet of every gateway of its CR. What it becomes is decided by
[regex-routes-migration](../regex-routes-migration/SKILL.md):

| Gateway | Path | Output |
|---|---|---|
| `public-gateway-service`, `private-gateway-service` | any | `AuthorizationPolicy` DENY rule (Step 6) — no HTTPRoute rule |
| any other gateway | no `{variables}` | rule **without** `backendRefs` on `PathPrefix <prefix>` — Istio returns 404 for the matched path (Step 7) |
| any other gateway | with `{variables}` | no rule; `# ⚠ MANUAL REVIEW` when Istio now routes it (Step 7) |

When the CR attaches to both kinds of gateway, the rule without `backendRefs`
is rendered once in the shared HTTPRoute; on public/private the DENY rule
answers first.

### Worksheet row

Every `Rule` becomes one row in the worksheet of each gateway of its CR (Step 4a
of the main skill). Fill the consumer columns like this:

| Column | Value |
|---|---|
| `#` | `R<n>`, numbered in discovery order across the chart (file, document, virtualService, route, rule) |
| `source` | `<file> <CR metadata.name> vs <virtualService.name> route <i> rule <j>` |
| `owner` | the HTTPRoute name ([HTTPRoute name resolution](#httproute-name-resolution)): the CR's `metadata.name` when the CR has one virtualService — **not** the virtualService name — else `<CR metadata.name>-<virtualService.name>` |
| `path` | `match.prefix`, else `match.path` (kind `exact`), else `match.regExp` (kind `regex`) — verbatim, Helm expressions included |
| `headers` | the converted header matches ([HeaderMatcher](#headermatcher)): `name=value` for an exact value, `name~<regex>` for a regular expression, sorted; `-` when none |
| `allowed` | `no` when `allowed: false`, otherwise `yes` |
| `forbidden` | `explicit` when `allowed: false`, otherwise `-` |
| `behavior` | one id per distinct combination of backend `name:port` (from `destination.endpoint`), `hostRewrite`, and the merged virtualService + rule `addHeaders` / `removeHeaders` |
| `rewrite` | `prefixRewrite`, or `-` |

A row's timeout (`Rule.timeout`) is not part of its behavior: a merged rule takes
the largest timeout of its rows.

### RouteMatch

  JSON key  Go type          Transformation
  ───────────────────────────────────────────────────────────────────────────────────────
  prefix    string           → path.type: PathPrefix,        value: <worksheet cut>
                               (= <prefix> without trailing `/` when it has no `{variables}`;
                               cut before the first variable otherwise — never a regex)
  path      string           → path.type: Exact,             value: <path>
  regExp    string           → path.type: RegularExpression, value: <regexp>  ⚠ flag for MANUAL REVIEW
                               (Istio ranks regex below every PathPrefix and has no regex rewrite)
  headers   []HeaderMatcher  → matches[].headers[]  (`:method` → matches[].method)

  Path match type — mutually exclusive, apply first non-empty in this priority:
    1. prefix  → PathPrefix
    2. path    → Exact
    3. regExp  → RegularExpression

---

### HeaderMatcher

  JSON key        Go type    Transformation
  ────────────────────────────────────────────────────────────────────────────────
  name            string     → matches[].headers[].name. `:method` with exactMatch/value →
                               matches[].method: <VALUE> instead (Gateway API rejects pseudo-
                               headers as header names); `:method` with any other specifier →
                               OMIT ⚠ flag for MANUAL REVIEW
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
| `RouteV3.Rule` | `idleTimeout` / `rateLimit` / `deny` | non-empty / non-nil |
| `HeaderMatcher` | `invertMatch: true` or `presentMatch: false` | Gateway API has no negated header match; dropping it widens the route |
| `HeaderMatcher` | `rangeMatch` | numeric range has no Gateway API equivalent |
| `RouteMatch` | `regExp` | raw regex match: lowest Istio tier, no regex rewrite |
| `Rule` | `prefix` with `{variables}` | flagged when [regex-routes-migration](../regex-routes-migration/SKILL.md) cannot convert it: rewrite not expressible, conflict answered `manual`, forbidden path routed on a gateway without DENY policies, DENY rule with a partial segment or a non-`:method` header |
