---
name: httproute-from-code
description: >
  Generate Gateway API HTTPRoute CRs from Go or Java route-registration code
  (routeregistration.Route / RouteEntry call sites). Use when asked to generate
  HTTPRoutes from source code, convert route registrations to HTTPRoute YAML, or
  extract routes from Go/Java files.
---

# Generate GatewayAPI HTTPRoute CRs from Go or Java route registration code

## Invocation

Run this skill against a file or directory of route-registration code. Examples:

- `internal/routes`
- `src/main/java/com/example/config/RouteConfig.java`
- `.`

---

## Access control model (read before Step 4)

Per [`docs/istio/allowed-and-forbidden-routes.md`](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md),
Istio does not reproduce the legacy per-gateway PUBLIC/PRIVATE/INTERNAL visibility or
forbidden-route (404) mechanism:

- `Public`, `Private`, `Internal` routes (and any route whose `Gateway` field names a border
  gateway — `public-gateway-service`, `private-gateway-service`, `internal-gateway-service`) are
  all registered on **all three** gateways: `public-gateway`, `private-gateway`,
  `internal-gateway-service`. There is no narrower registration anymore.
- `Forbidden: true` (Go) / `allowed: false` (Java) routes are never emitted as a 404 rule — they
  are dropped from the HTTPRoute entirely and recorded for the access-control report (Step 4).
- Public-gateway access control belongs to a separate layer: a hand-written `AuthorizationPolicy`.
  This skill does **not** generate one — see [Non-goals](#non-goals).

## Inputs / parameters

Besides `<path>`, this skill accepts optional `backendRefs` parameters and an
optional `routeLabels` parameter. They control `backendRefs[]` and
`metadata.labels` emitted in every generated HTTPRoute CR. When invoked by the
`core-mesh-to-istio-migration`
orchestrator, these are passed in already resolved — either detected from the
existing mesh CRs by the `core-mesh-crs-to-istio` skill or provided by the user.

| Parameter | Controls | Default |
|---|---|---|
| `backendRefName` | `backendRefs[].name` in every rule | `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}` |
| `backendRefPort` | `backendRefs[].port` in every rule | `8080` |
| `routeLabels` | `metadata.labels` on every generated HTTPRoute CR | default label set below |
| `interactive` | ability to ask the user; `true` only for direct user invocation | `false` |
| `resolutions` | answers to a previous run's `unresolved:` entries, by id | — |

Resolution rules:

- If a value is provided by the caller (or the orchestrator), use it verbatim for
  **all** generated CRs — do not infer per-route values.
- If `backendRefName` / `backendRefPort` are not provided: with
  `interactive: true`, propose the defaults to the user and ask for
  confirmation before generating; with `interactive: false`, use the defaults
  and record the used values under `inputsUsed:` in the report.
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

## Contract — report output

In addition to the chat summary, write a machine-readable report to
`.mesh-migration/reports/httproute-from-code.yaml` (create the directory, and ensure
`.mesh-migration/` is listed in the repo's `.gitignore` — reports are working
files, never committed; the orchestrator handles both in orchestrated runs):

```yaml
reportSchema: 1
skill: httproute-from-code
status: complete            # complete | partial | failed
generatedAt: <ISO-8601>
inputsUsed:
  backendRefName: <value>
  backendRefPort: <value>
  routeLabels: <map>
filesGenerated: [<paths>]
routesGenerated: <N>
accessControl:                     # omit entirely when all three lists are empty
  legacyPublic:                    # routes that used to be reachable from the public gateway
    - from: <original un-reduced path>
      source: <file:line>
  forbidden:                       # Forbidden:true / allowed:false routes — never emitted
    - from: <path>
      legacyType: <Public|Private|Internal>
      source: <file:line>
  # this skill has no deny-route input (Go/Java registration has no `deny` flag);
  # `deny` entries only ever come from core-mesh-crs-to-istio
unresolved: []              # blocking user decisions, each {id, question, options, default}
                            # e.g. id microservice-name when it fell back to <microservice-name>
needsReview:
  - <one line per skipped row or ERROR>
```

Consumers must ignore unknown report fields. A consumer that sees a
`reportSchema` newer than its own documentation must stop and report a contract
mismatch instead of guessing field meanings.

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

> **IMPORTANT — Annotation-based routes are explicitly OUT OF SCOPE.**
> `@Route` / `@Gateway` class/method annotations are processed at compile time
> by `httproutes-generator-maven-plugin` / `httproutes-generator-spring-maven-plugin`. Do NOT
> extract routes from these annotations here — doing so would duplicate the
> plugin's output. Only extract routes from **`RouteEntry` builder/constructor
> call sites** as shown below.

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

## Step 4 — Skip forbidden routes and record access-control entries

### Go
Skip if `Forbidden: true`

### Java
Skip if `allowed: false`

A skipped route is **never emitted** — legacy forbidden routes have no Istio equivalent (see
[Access control model](#access-control-model-read-before-step-4)). Instead:

1. Add it to the report's `accessControl.forbidden[]`: `from` (original path), `legacyType`
   (`Public`/`Private`/`Internal`), `source` (`file:line`).
2. Include it in the Step 12 summary with `Skipped: yes (forbidden)`.

Do not generate any 404 / no-backend rule for it, and do not infer an `AuthorizationPolicy` from
it here — that is [`public-gateway-authz-policy`](../public-gateway-authz-policy/SKILL.md)'s job,
fed by this report.

---

## Step 5 — Map RouteType → gateways

Legacy `Public`/`Private`/`Internal` visibility differences no longer exist: every one of these
types — and any route whose `gateway` field is explicitly one of the three border gateway
service names — is registered on the **same fixed set of three gateways**.

| RouteType | Target gateways |
|---|---|
| `Public` | `public-gateway`, `private-gateway`, `internal-gateway-service` |
| `Private` | `public-gateway`, `private-gateway`, `internal-gateway-service` |
| `Internal` | `public-gateway`, `private-gateway`, `internal-gateway-service` |
| `Mesh` | `Gateway` / `gateway` field value |
| `Facade` | `{{ .Values.SERVICE_NAME }}` |

Public, Private and Internal routes all belong to one **gateway group** (Step 6) — the
distinction between the three legacy types no longer changes `parentRefs` or which CR a route
lands in. It is still recorded as `legacyType` per route (see [Step 4](#step-4--skip-forbidden-routes-and-record-access-control-entries)
and the Step 12 summary) because it feeds the access-control report: a route whose `legacyType`
was `Public` used to be reachable from the public gateway and is a candidate for the service's
hand-written `AuthorizationPolicy`; `Private`/`Internal` routes were never reachable there and are
not candidates.

**Populate `accessControl.legacyPublic[]`:** for every emitted (non-skipped) route whose
`legacyType` is `Public`, add its original `from` path and `file:line` source to the report's
`accessControl.legacyPublic[]`. Do not add `Private`/`Internal`/`Mesh`/`Facade` routes there.

### Gateway field override
If `gateway` is explicitly set to one of the three border gateway service names
(`public-gateway-service`, `private-gateway-service`, `internal-gateway-service`) → treat the
route as gateway-group (same as Public/Private/Internal), regardless of `routeType`. Use the
gateway value to infer `legacyType` for the access-control report only: `public-gateway-service`
→ `Public`, `private-gateway-service` → `Private`, `internal-gateway-service` → `Internal`.

If `gateway` is set to anything else:

| gateway value | inferred type |
|---|---|
| `facade` / `facade-gateway` | Facade |
| anything else | Mesh |

---

## Step 6 — Group routes → one CR per group

Generate ONE HTTPRoute CR for the whole gateway group (Public + Private + Internal + any route
explicitly naming a border gateway), plus one CR per other RouteType present in the source.

### CR structure per group

| Group | CR name suffix | parentRefs | rules[] contains |
|---|---|---|---|
| gateway group (`Public`/`Private`/`Internal`/border-gateway) | `-source-code-routes` | `public-gateway`, `private-gateway`, `internal-gateway-service` | every gateway-group route |
| `Mesh` | `-source-code-mesh-routes` | gateway field value | Mesh routes only |
| `Facade` | `-source-code-facade-routes` | `{{ .Values.SERVICE_NAME }}` | Facade routes only |

### Algorithm

1. Collect all non-skipped routes, classified into the gateway group, `Mesh`, or `Facade`.
2. For each group that has at least one route → generate one CR.
3. `parentRefs` = the fixed list for that group (see table above).
4. `rules[]` = all routes of that group, **after root reduction** (see
   [Step 6a](#step-6a--reduce-to-pathprefix-roots)).
5. If no routes exist for a given group → skip that CR entirely.

### Full example

Input:
```
Route A  From=/api/v1/users    To=/users    RouteType=Public
Route B  From=/api/v1/private  To=/private  RouteType=Private
Route C  From=/api/v1/admin    To=/admin    RouteType=Internal
```

Produces ONE CR (all three share the same fixed gateway set):

```
<name>-source-code-routes   parentRefs: [public-gateway, private-gateway, internal-gateway-service]  rules: [Route A, Route B, Route C]
```

### Step 6a — Reduce to PathPrefix roots

Apply this reduction **within each group**, before sorting (Step 9) and before rendering (Step
10). It mirrors the reduction `httproutes-generator-maven-plugin` /
`httproutes-generator-spring-maven-plugin` perform on annotation-based routes, so code-registered
and annotation-registered routes collapse the same way.

1. For each route's `from`, cut the path right before the first segment that contains a
   template placeholder (Go: none — Go routes are already concrete; Java: `{var}`) or a wildcard
   (`*`). Everything before the cut is the **match** (a `PathPrefix`); the route has no cut point
   when `from` has no such segment, and the whole `from` is the match.
2. If there is a cut (a variable/wildcard tail exists):
   - the `to` path must end with the same tail shape as the `from` tail, ignoring variable names
     (e.g. `from=/api/v1/users/{id}`, `to=/users/{id}` → tail `/{id}` matches on both sides);
   - if the tails don't match in shape → `ERROR` (see [Error handling](#error-handling));
   - the rewrite target is `to` with that tail removed.
3. If there is no cut, the rewrite target is `to` verbatim (or omit the rewrite filter entirely
   when `to == from`).
4. **Merge against shorter roots already collected in the same group:** if a shorter root's match
   is a prefix of this route's match, and applying that shorter root's rewrite to this route's
   match already produces this route's target, drop this route — it is already covered. Its
   `timeout` becomes `max(existing, this route's timeout)`.
5. **Conflict:** if two routes in the same group reduce to the identical match but different
   rewrite targets → `ERROR` naming both routes.
6. **Root `/`:** a route (or a merge) whose match reduces to `/` → `ERROR` — this would shadow
   every other route in the service and every other service behind the same waypoint/gateway.
7. Never emit `RegularExpression` or `Exact` matches from this step — the output is always
   `PathPrefix`, optionally with a `URLRewrite`/`ReplacePrefixMatch` filter.

### Deduplication within a CR

Dedup is now subsumed by root reduction (Step 6a): two routes with identical `from`+`to` reduce to
the same root and merge automatically. Two routes with the same `from` but different `to` are a
**conflict** (see Step 6a.5) — they are no longer both kept as separate rules, because a single
`PathPrefix` match can only carry one rewrite target.

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

7. Fallback: check the `resolutions` input for id `microservice-name`; if
   absent, with `interactive: true` ask the user for the service name,
   otherwise use the literal `<microservice-name>` placeholder, add an
   `unresolved:` entry (id `microservice-name`, question "What is the
   microservice name for the generated HTTPRoutes?"), and set
   `status: partial` in the report

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

Before generating the CR, sort all collected roots (after [Step 6a](#step-6a--reduce-to-pathprefix-roots)
reduction) so that the most specific `match` paths appear first in `rules[]`.

Apply the shared procedure in
[`path-specificity-sorting.md`](../path-specificity-sorting/SKILL.md)
— sort on each rule's reduced `match` path. That file defines the segment-count ordering,
tie-breaks, a worked example, and why ordering matters across gateway
implementations.

---

## Step 10 — Generate HTTPRoute

Generate one CR per group (see [Step 6](#step-6--group-routes--one-cr-per-group)). Wrap ALL CRs
together in a single Istio conditional block.

**Rule order:** emit `rules[]` in the path-specificity order produced by
[Step 9](#step-9--sort-rules-by-path-specificity) (shared procedure
[`path-specificity-sorting.md`](../path-specificity-sorting/SKILL.md)).
Most specific match first — never in source/discovery order.

### ParentRef resolution

Resolve every target gateway name to a Gateway API `parentRefs` entry before rendering:

| Target | Rendered parentRef |
|---|---|
| `public-gateway` | `- name: public-gateway` with `kind: Gateway` and `group: gateway.networking.k8s.io` |
| `private-gateway` | `- name: private-gateway` with `kind: Gateway` and `group: gateway.networking.k8s.io` |
| `internal-gateway-service` | `- name: internal-gateway-service` with `kind: Service` and `group: ''` (no `port`) |

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
routes are not emitted at all (see [Step 4](#step-4--skip-forbidden-routes-and-record-access-control-entries)),
so there is no rule with a missing `backendRefs`.

### Labels resolution

If `routeLabels` is provided:

- Render a `metadata.labels` section on every generated HTTPRoute CR.
- Copy all labels exactly as provided (including Helm template expressions).
- Keep the same label set for all generated CRs in this run.

If `routeLabels` is not provided:

- Render `metadata.labels` using the default label set from
  [Inputs / parameters](#inputs--parameters).

### HTTPRoute naming schema

Generated HTTPRoute names follow this fixed pattern:

`<microservice-name>-source-code-<suffix>`

Where `<suffix>` is:

| Group | Name suffix |
|---|---|
| gateway group (Public/Private/Internal/border-gateway) | `routes` |
| `Mesh` | `mesh-routes` |
| `Facade` | `facade-routes` |

Examples:

- `billing-service-source-code-routes`
- `billing-service-source-code-mesh-routes`
- `billing-service-source-code-facade-routes`

> The previous per-type names `-public-routes` / `-private-routes` / `-internal-routes` are no
> longer generated. Regenerating `source-code-httproutes.yaml` on a chart that still has those
> files from an older run replaces them with the single `-routes` CR — delete the stale file if
> Helm keeps a copy around from a previous commit.

Naming rules:

- Use the microservice name resolved in [Step 7](#step-7--resolve-microservice-name).
- Emit exactly one HTTPRoute name per group that has routes (see Step 6).
- If Step 7 cannot resolve the service name, use `<microservice-name>` in the
  generated name and record the `unresolved:` entry per Step 7 — the caller
  supplies the real name via the `resolutions` input.

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: <microservice-name>-source-code-routes
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

| # | File | From | To | Legacy type | Group | Timeout | Hosts | Skipped |
|---|---|---|---|---|---|---|---|---|
| 1 | routes.go | /api/v1/users/profile | /users/profile | Public | gateway | 30s | - | no |
| 2 | RouteConfig.java | /api/v1/users | /users | Public | gateway | - | - | no |
| 3 | routes.go | /mesh | /mesh | Mesh | mesh | - | - | no |
| 4 | RouteConfig.java | /admin | /admin | Public | gateway | - | - | yes (forbidden) |
```

Note: summary rows reflect sorted order (most specific first) within each CR; forbidden/skipped
rows are listed after the emitted rows for that file.

Also report the `backendRefs` values applied to all rules:

```
backendRefName: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}   (detected | user-provided | default)
backendRefPort: 8080                                     (detected | user-provided | default)
```

Also report labels applied to generated CRs:

```
routeLabels: <map or "default label set">
```

Also report access control, when any routes were legacy-Public or forbidden/skipped:

```
## Access control

Legacy-public routes now registered on public-gateway, private-gateway and internal-gateway-service:
- /api/v1/users (routes.go:12)

Forbidden routes (never emitted — no Istio equivalent):
- /admin (RouteConfig.java:42, legacy type: Public)

These are only informational here. No AuthorizationPolicy is generated by this skill — see
public-gateway-authz-policy for the recommended manual-authoring workflow (generation only on
explicit request).
```

---

## Error handling

Stop and report if:

- `from` / `From` is missing
- Mesh/MESH route has no gateway field
- Java constructor argument types are ambiguous
- No routes detected in any file
- `backendRefPort` is provided but is not a positive integer
- `routeLabels` is provided but is not a string-to-string map
- [Step 6a](#step-6a--reduce-to-pathprefix-roots) root reduction fails: a variable/wildcard tail
  mismatch between `from` and `to`, two routes in the same group reducing to the same match with
  different targets, or a match reducing to `/`

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

- VirtualService
- Ingress
- GRPCRoute
- TCPRoute
- `RegularExpression` / `Exact` path matches (every match is `PathPrefix`, see [Step 6a](#step-6a--reduce-to-pathprefix-roots))
- a 404 / no-backend rule for a forbidden route (see [Step 4](#step-4--skip-forbidden-routes-and-record-access-control-entries))
- an `AuthorizationPolicy` of any kind — public-gateway access control is a separate, normally
  hand-authored resource. Use the `accessControl` report block as input to
  [`public-gateway-authz-policy`](../public-gateway-authz-policy/SKILL.md) if the user explicitly
  asks for a generated draft.

Only HTTPRoute.
