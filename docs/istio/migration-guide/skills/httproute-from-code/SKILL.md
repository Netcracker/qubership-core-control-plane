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
| `Public` | `-public-routes` | public-gateway, private-gateway, internal-gateway | Public routes only |
| `Private` | `-private-routes` | private-gateway, internal-gateway | Private routes only |
| `Internal` | `-internal-routes` | internal-gateway | Internal routes only |
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
<name>-public-routes    parentRefs: [public, private, internal]  rules: [Route A]
<name>-private-routes   parentRefs: [private, internal]          rules: [Route B]
<name>-internal-routes  parentRefs: [internal]                   rules: [Route C]
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

### Specificity ordering rules

1. Count path segments (split by `/`, ignore empty):
   `/api/v1/users/profile` → 4 segments  
   `/api/v1/users` → 3 segments  
   `/api` → 1 segment

2. More segments = higher specificity = appears first.

3. Tie-break by path length (longer string first).

4. Tie-break by lexicographic order (ascending) for stable output.

### Example — input order does not matter, output is always sorted

Input routes (any order):
```
/api/v1/mesh-test-service-go/1234   → 4 segments
/api/v1/mesh-test-service-go        → 3 segments
/api/v1                             → 2 segments
/api                                → 1 segment
```

Sorted output in rules[]:
```yaml
rules:
  - matches:
      - path:
          type: PathPrefix
          value: /api/v1/mesh-test-service-go/1234   # 4 segments — first
  - matches:
      - path:
          type: PathPrefix
          value: /api/v1/mesh-test-service-go         # 3 segments
  - matches:
      - path:
          type: PathPrefix
          value: /api/v1                              # 2 segments
  - matches:
      - path:
          type: PathPrefix
          value: /api                                 # 1 segment — last
```

### Why this matters
GatewayAPI spec defines that more specific prefix matches win regardless of
order, but not all gateway implementations (Envoy, Nginx, Istio) respect this
consistently. Sorting longest-prefix-first ensures correct behaviour across
all implementations.

---

## Step 10 — Generate HTTPRoute

Generate one CR per RouteType. Wrap ALL CRs together in a single Istio conditional block.
```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: <microservice-name>-public-routes
  namespace: {{ .Values.NAMESPACE }}
spec:
  parentRefs:
    - name: public-gateway
    - name: private-gateway
    - name: internal-gateway
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
        - name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
          port: 8080
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: <microservice-name>-private-routes
  namespace: {{ .Values.NAMESPACE }}
spec:
  parentRefs:
    - name: private-gateway
    - name: internal-gateway
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
        - name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
          port: 8080
{{- end }}
```

The `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` opens before the first CR and `{{- end }}` closes after the last CR. The `---` separators between CRs remain inside the block.

---

## Step 11 — Output formatting

Separate multiple CRs with `---`.

Output file:

```
helm-templates/<service name>/source-code-httproutes.yaml
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

---

## Error handling

Stop and report if:

- `from` / `From` is missing
- Mesh/MESH route has no gateway field
- RouteType conflicts with explicit gateway value
- Java constructor argument types are ambiguous
- No routes detected in any file

Error format:

```
ERROR:
file: src/main/java/com/example/config/RouteConfig.java
line: 42
reason: RouteEntry missing 'from' field
```

---

## Non-goals

Do NOT generate:

VirtualService
Ingress
GRPCRoute
TCPRoute

Only HTTPRoute.
