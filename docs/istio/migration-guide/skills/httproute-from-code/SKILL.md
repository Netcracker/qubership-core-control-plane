---
name: httproute-from-code
description: >
  Generate Kubernetes GatewayAPI HTTPRoute CRs from Go or Java route registration code.
  Use when the user runs /httproute-from-code, asks to generate HTTPRoute from source code,
  convert route registrations to HTTPRoute YAML, or extract routes from Go/Java files.
  Triggers on: httproute generation, route extraction, routeregistration, RouteEntry,
  GatewayAPI from code, go routes to yaml, java routes to yaml.
---

# Generate GatewayAPI HTTPRoute CRs from Go or Java route registration code

## Trigger

Use this skill when the user runs:

/httproute-from-code <path>

Examples:

/httproute-from-code internal/routes
/httproute-from-code src/main/java/com/example/config/RouteConfig.java
/httproute-from-code .

---

## Inputs / parameters

Besides `<path>`, this skill accepts optional `backendRefs` parameters and an
optional `routeLabels` parameter. They control `backendRefs[]` and
`metadata.labels` emitted in every generated HTTPRoute CR. When invoked by the
[`core-mesh-to-istio-migration`](../core-mesh-to-istio-migration/SKILL.md)
orchestrator, these are passed in already resolved — either detected from the
existing mesh CRs by the `core-mesh-crs-to-gatewayapi` skill or provided by the user.

| Parameter | Controls | Default |
|---|---|---|
| `backendRefName` | `backendRefs[].name` in every rule | `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}` |
| `backendRefPort` | `backendRefs[].port` in every rule | `8080` |
| `routeLabels` | `metadata.labels` on every generated HTTPRoute CR | default label set from |

Resolution rules:

- If a value is provided by the caller (or the orchestrator), use it verbatim for
  **all** generated CRs — do not infer per-route values.
- If `backendRefName` / `backendRefPort` are not provided, propose the defaults
  to the user and ask for confirmation before generating. When running standalone
  with no opportunity to ask, use the defaults and note the used values in the summary.
- `backendRefName` is used as-is, including Helm template expressions such as
  `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}`.
- `backendRefPort` must be a positive integer. If a non-integer value is given,
  stop with an `ERROR:` (see Error handling).
- `routeLabels` must be a map of string keys to string values. Apply exactly the
  same label set to every generated HTTPRoute CR. Do not infer per-route labels.
- If `routeLabels` is provided by caller/orchestrator, use it verbatim.
- If `routeLabels` is not provided, use this default label set for every
  generated HTTPRoute CR:
  - `app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}`
  - `app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}`
  - `app.kubernetes.io/managed-by: {{ .Values.MANAGED_BY }}`
  - `deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}`
  - `deployer.cleanup/allow: "true"`
  - `app.kubernetes.io/processed-by-operator: istiod`

---

## Step 1 — Discover files and detect language

Resolve <path>:

- if file → detect language from extension
- if directory → recursively scan, detect language per file

| Extension | Language |
|---|---|
| `*.go` | Go |
| `*.java` | Java |

Ignore:

```
vendor/
.git/
node_modules/
testdata/
target/
src/test/
```

If mixed Go and Java found → process both, merge into one HTTPRoute CR per microservice.

If no files found:

```
ERROR:
No Go or Java files found in provided path
```

---

## Step 2 — Detect route definitions

### Go patterns

```go
// Struct literal inline
registrar.WithRoutes(
    routeregistration.Route{
        From:      "/api/v1/users",
        To:        "/users",
        RouteType: routeregistration.Public,
        Timeout:   30 * time.Second,
        Forbidden: false,
        Gateway:   "",
        Hosts:     []string{"api.company.com"},
    },
)

// Chained
routeregistration.NewRegistrar().
    WithRoutes(routeregistration.Route{...}).
    Register()

// Variable
r := routeregistration.Route{...}
registrar.WithRoutes(r)

// Slice spread
routes := []routeregistration.Route{...}
registrar.WithRoutes(routes...)

// Mesh
routeregistration.Route{
    From:      "/mesh",
    RouteType: routeregistration.Mesh,
    Gateway:   "mesh-gateway",
}

// Facade
routeregistration.Route{
    From:    "/facade",
    Gateway: "facade",
}
```

### Java patterns

```java
// Builder
RouteEntry.builder()
    .from("/api/v1/users")
    .to("/users")
    .type(RouteType.PUBLIC)
    .timeout(30000L)
    .allowed(true)
    .namespace("default")
    .gateway("my-gateway")
    .hosts(Set.of("api.company.com"))
    .build()

// Constructors — all variants
new RouteEntry("/api/v1/users", RouteType.PUBLIC)
new RouteEntry("/api/v1/users", RouteType.PUBLIC, 30000L)
new RouteEntry("/api/v1/users", RouteType.PUBLIC, "prod-namespace")
new RouteEntry("/api/v1/users", RouteType.PUBLIC, "prod-namespace", 30000L)
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC)
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC, 30000L)
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC, "prod-namespace")
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC, "prod-namespace", 30000L)

// Collections
List.of(new RouteEntry(...), RouteEntry.builder()...build())
routes.add(new RouteEntry(...))

// postRoutes call sites
processor.postRoutes(List.of(new RouteEntry(...)))
processor.postRoutes(microserviceUrl, routes)
```

---

## Step 3 — Extract fields

### Unified field table

| Field | Go source | Java source | Default |
|---|---|---|---|
| `from` | `From:` | `.from(...)` / 1st path arg | REQUIRED |
| `to` | `To:` | `.to(...)` / 2nd path arg | same as `from` |
| `routeType` | `RouteType:` | `.type(RouteType.X)` | Public / PUBLIC |
| `skip` | `Forbidden: true` | `allowed: false` | false |
| `namespace` | n/a (from config) | `.namespace(...)` / namespace arg | `default` |
| `timeout` | `Timeout:` | `.timeout(...)` / timeout arg | omit |
| `gateway` | `Gateway:` | `.gateway(...)` | derived |
| `hosts` | `Hosts:` | `.hosts(Set.of(...))` | omit |

### Java constructor disambiguation
3-arg `new RouteEntry(path, type, X)`:
- X is `Long` or numeric literal → timeout
- X is `String` → namespace

4-arg `new RouteEntry(from, to, type, X)`:
- X is `Long` or numeric literal → timeout
- X is `String` → namespace

### RouteType normalization

| Go | Java | Canonical |
|---|---|---|
| `routeregistration.Public` | `RouteType.PUBLIC` | `Public` |
| `routeregistration.Private` | `RouteType.PRIVATE` | `Private` |
| `routeregistration.Internal` | `RouteType.INTERNAL` | `Internal` |
| `routeregistration.Mesh` | `RouteType.MESH` | `Mesh` |
| n/a | `RouteType.FACADE` | `Facade` |

---

## Step 4 — Skip routes

### Go
Skip if `Forbidden: true`

### Java
Skip if `allowed: false`

Include skipped routes in summary.

---

## Step 5 — Map RouteType → gateways

| RouteType | Target gateways |
|---|---|
| `Public` | `public-gateway`, `private-gateway`, `internal-gateway` |
| `Private` | `private-gateway`, `internal-gateway` |
| `Internal` | `internal-gateway` only |
| `Mesh` | `Gateway` / `gateway` field value |
| `Facade` | `{{ .Values.SERVICE_NAME }}` |

### Gateway field override
If `gateway` is explicitly set AND type is Public/Private/Internal:
→ use gateway field value instead of derived gateways

If `gateway` is set AND type is empty → infer type:

| gateway value | inferred type |
|---|---|
| `public-gateway` | Public |
| `private-gateway` | Private |
| `internal-gateway` | Internal |
| `facade` / `facade-gateway` | Facade |
| anything else | Mesh |

---

## Step 6 — Group routes by RouteType → one CR per RouteType

Generate ONE HTTPRoute CR per RouteType present in the source.

### CR structure per RouteType

| RouteType | CR name suffix | parentRefs | rules[] contains |
|---|---|---|---|
| `Public` | `-public-routes` | public-gateway, private-gateway, internal-gateway-service | Public routes only |
| `Private` | `-private-routes` | private-gateway, internal-gateway-service | Private routes only |
| `Internal` | `-internal-routes` | internal-gateway-service | Internal routes only |
| `Mesh` | `-mesh-routes` | gateway field value | Mesh routes only |
| `Facade` | `-facade-routes` | `{{ .Values.SERVICE_NAME }}` | Facade routes only |

### Algorithm

1. Collect all routes grouped by RouteType.
2. For each RouteType that has at least one route → generate one CR.
3. `parentRefs` = the fixed gateway list for that RouteType (see table above).
4. `rules[]` = all routes of that RouteType only.
5. If no routes of a given RouteType exist → skip that CR entirely.

### Full example

Input:
```
Route A  From=/api/v1/users    To=/users    RouteType=Public
Route B  From=/api/v1/private  To=/private  RouteType=Private
Route C  From=/api/v1/admin    To=/admin    RouteType=Internal
```

Produces THREE CRs:

```
<name>-source-code-public-routes    parentRefs: [public-gateway, private-gateway, internal-gateway-service]  rules: [Route A]
<name>-source-code-private-routes   parentRefs: [private-gateway, internal-gateway-service]                  rules: [Route B]
<name>-source-code-internal-routes  parentRefs: [internal-gateway-service]                                   rules: [Route C]
```

### Deduplication within a CR

If two route definitions have identical `from` + `to` + `RouteType` → emit ONE rule.
If same `from` but different `to` → keep both rules.

---

## Step 7 — Resolve microservice name

Priority:

1. `application.yaml`:
```yaml
microservice:
  name: billing-service
```

2. `application.properties`:
```
microservice.name=billing-service
```

3. Go — env reference: `MICROSERVICE_NAME`

4. Java — `@ConfigProperty(name = "cloud.microservice.name")`

5. Go — `go.mod` last path segment:
```
module github.com/company/billing-service → billing-service
```

6. Java — `pom.xml` artifactId

7. Fallback: `<microservice-name>` — warn in summary

---

## Step 8 — Convert timeout

### Go — time.Duration literals
| Go | GatewayAPI |
|---|---|
| `30 * time.Second` | `30s` |
| `1 * time.Minute` | `1m` |
| `90 * time.Second` | `90s` |
| `time.Minute + 30*time.Second` | `90s` |

Unsupported expressions → omit.

### Java — Long milliseconds
| Java | GatewayAPI |
|---|---|
| `30000L` | `30s` |
| `60000L` | `1m` |
| `90000L` | `90s` |
| `1500L` | `1500ms` |

Rule: divisible by 60000 → `Xm`, divisible by 1000 → `Xs`, else → `Xms`.

---

## Step 9 — Sort rules by path specificity

Before generating the CR, sort all collected routes so that the most specific
`from` paths appear first in `rules[]`.

Apply the shared procedure in
[`../shared/path-specificity-sorting.md`](../shared/path-specificity-sorting.md)
— sort on each rule's `from` path. That file defines the segment-count ordering,
tie-breaks, a worked example, and why ordering matters across gateway
implementations.

---

## Step 10 — Generate HTTPRoute

Generate one CR per RouteType. Wrap ALL CRs together in a single Istio conditional block.

**Rule order:** emit `rules[]` in the path-specificity order produced by
[Step 9](#step-9--sort-rules-by-path-specificity) (shared procedure
[`../shared/path-specificity-sorting.md`](../shared/path-specificity-sorting.md)).
Most specific match first — never in source/discovery order.

### ParentRef resolution

Resolve every target gateway name to a Gateway API `parentRefs` entry before rendering:

| Target | Rendered parentRef |
|---|---|
| `public-gateway` | `- name: public-gateway` with `kind: Gateway` and `group: gateway.networking.k8s.io` |
| `private-gateway` | `- name: private-gateway` with `kind: Gateway` and `group: gateway.networking.k8s.io` |
| `internal-gateway-service` | `- name: internal-gateway-service` with `kind: Service` and `group: ''` |

**Mandatory fields — every `parentRefs[]` entry MUST render all three:**

- `group:` — `gateway.networking.k8s.io` for `kind: Gateway`, or `''` (empty
  string) for `kind: Service`. Always present, never omitted.
- `kind:` — `Gateway` or `Service`.
- `name:` — the resolved parent name.

Never emit a parentRef with a missing `group`, `kind`, or `name` (an empty
`group` must still be written as `group: ''`, not dropped).

### BackendRef resolution

**Mandatory fields — every rule's `backendRefs[]` entry MUST render all five:**

- `group:` — always `''` (empty string), never omitted.
- `kind:` — always `Service`.
- `name:` ← `backendRefName` (default `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}`).
- `port:` ← `backendRefPort` (default `8080`).
- `weight:` — always `1`.

The same `backendRefName` / `backendRefPort` apply to every rule across every CR
— they are migration-wide, not per-route (see
[Inputs / parameters](#inputs--parameters)). The examples below use the defaults;
substitute the confirmed values when they differ.

These five fields are always required on every emitted rule. Forbidden/skipped
routes are not emitted at all (see [Step 4](#step-4--skip-routes)), so there is
no rule with a missing `backendRefs`.

### Labels resolution

If `routeLabels` is provided:

- Render a `metadata.labels` section on every generated HTTPRoute CR.
- Copy all labels exactly as provided (including Helm template expressions).
- Keep the same label set for all generated CRs in this run.

If `routeLabels` is not provided:

- Render `metadata.labels` using the default label set from
  `httproute-generator/README.md`:
  - `app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}`
  - `app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}`
  - `app.kubernetes.io/managed-by: {{ .Values.MANAGED_BY }}`
  - `deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}`
  - `deployer.cleanup/allow: "true"`
  - `app.kubernetes.io/processed-by-operator: istiod`

### HTTPRoute naming schema

Generated HTTPRoute names follow this fixed pattern:

`<microservice-name>-source-code-<route-type>-routes`

Where `<route-type>` is lowercase and mapped as:

| Canonical RouteType | Name suffix |
|---|---|
| `Public` | `public-routes` |
| `Private` | `private-routes` |
| `Internal` | `internal-routes` |
| `Mesh` | `mesh-routes` |
| `Facade` | `facade-routes` |

Examples:

- `billing-service-source-code-public-routes`
- `billing-service-source-code-private-routes`
- `billing-service-source-code-internal-routes`

Naming rules:

- Use the microservice name resolved in [Step 7](#step-7--resolve-microservice-name).
- Emit exactly one HTTPRoute name per RouteType that has routes (see Step 6).
- If Step 7 cannot resolve the service name, use `<microservice-name>` in the
  generated name and report it as a warning in the summary / needs-review flow.

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: <microservice-name>-source-code-public-routes
  labels:
    app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}
    app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}
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

  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/v1/mesh-test-service-go
      filters:
        - type: URLRewrite
          urlRewrite:
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /api/v1
      backendRefs:
        - group: ''
          kind: Service
          name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
          port: 8080
          weight: 1
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: <microservice-name>-source-code-private-routes
spec:
  parentRefs:
    - group: gateway.networking.k8s.io    
      kind: Gateway    
      name: private-gateway
    - group: ''
      kind: Service
      name: internal-gateway-service
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/v1/mesh-test-service-go-private
      filters:
        - type: URLRewrite
          urlRewrite:
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /api/v1/private
      backendRefs:
        - group: ''
          kind: Service
          name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
          port: 8080
          weight: 1
{{- end }}
```

The `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` opens before the first CR and `{{- end }}` closes after the last CR. The `---` separators between CRs remain inside the block.

---

## Step 11 — Output formatting

Separate multiple CRs with `---`.

Output file:

```
helm-templates/<service name>/templates/source-code-httproutes.yaml
```

---

## Step 12 — Summary

```
## Summary

| # | File | From | To | RouteType | Gateways | Timeout | Hosts | Skipped |
|---|---|---|---|---|---|---|---|---|
| 1 | routes.go | /api/v1/users/profile | /users/profile | Public | public,private,internal | 30s | - | no |
| 2 | RouteConfig.java | /api/v1/users | /users | Public | public,private,internal | - | - | no |
| 3 | routes.go | /mesh | /mesh | Mesh | mesh-gateway | - | - | no |
| 4 | RouteConfig.java | /admin | /admin | Public | - | - | - | yes (allowed=false) |
```

Note: summary rows reflect sorted order (most specific first).

Also report the `backendRefs` values applied to all rules:

```
backendRefName: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}   (detected | user-provided | default)
backendRefPort: 8080                                     (detected | user-provided | default)
```

Also report labels applied to generated CRs:

```
routeLabels: <map or "default labels from README used">
```

---

## Error handling

Stop and report if:

- `from` / `From` is missing
- Mesh/MESH route has no gateway field
- RouteType conflicts with explicit gateway value
- Java constructor argument types are ambiguous
- No routes detected in any file
- `backendRefPort` is provided but is not a positive integer
- `routeLabels` is provided but is not a string-to-string map

Error format:

```
ERROR:
file: src/main/java/com/example/config/RouteConfig.java
line: 42
reason: RouteEntry missing 'from' field
```

---

## Non-goals

Do NOT modify source code - it is readonly input for HTTPRoute generation

Do NOT generate:

VirtualService
Ingress
GRPCRoute
TCPRoute

Only HTTPRoute.
