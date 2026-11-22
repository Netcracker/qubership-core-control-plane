---
name: httproute-from-code
description: >
  Generate Gateway API HTTPRoute CRs from Go or Java route-registration code
  (routeregistration.Route / RouteEntry call sites). Use when asked to generate
  HTTPRoutes from source code, convert route registrations to HTTPRoute YAML, or
  extract routes from Go/Java files.
---

# Generate Gateway API HTTPRoute CRs from Go or Java route registration code

## Invocation

Run against a file or directory of route-registration code, e.g. `internal/routes`,
`src/main/java/com/example/config/RouteConfig.java`, or `.`.

## Access control model (read first)

Per [`docs/istio/allowed-and-forbidden-routes.md`](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md),
Istio does not reproduce the legacy per-gateway PUBLIC/PRIVATE/INTERNAL visibility or forbidden (404)
routes:

- `Public`, `Private`, `Internal` routes, and any route whose gateway is `public-gateway-service`,
  `private-gateway-service` or `internal-gateway-service`, are all registered on **all three**
  gateways: `public-gateway`, `private-gateway`, `internal-gateway-service`.
- `Forbidden: true` (Go) / `allowed: false` (Java) routes are **never emitted**; they are only
  recorded in the report.
- Public-gateway access control is a separate, hand-written `AuthorizationPolicy`. This skill never
  generates one (see [Non-goals](#non-goals)).

## Inputs

| Parameter | Controls | Default |
|---|---|---|
| `path` | file/directory to scan | required |
| `chartPath` | output goes to `<chartPath>/templates/source-code-httproutes.yaml` | `helm-templates/<service name>/templates/` |
| `backendRefName` | `backendRefs[].name` in every rule | `{{ .Values.SERVICE_NAME }}` |
| `backendRefPort` | `backendRefs[].port` in every rule | `8080` |
| `routeLabels` | `metadata.labels` on every generated CR (string→string map) | default set below |
| `interactive` | may ask the user; `true` only for direct invocation | `false` |
| `resolutions` | answers to a previous run's `unresolved:` entries, by id | — |

Rules:

- Caller-provided values are used verbatim for **all** CRs (Helm expressions included); never infer
  per-route values.
- Missing `backendRefName`/`backendRefPort`: with `interactive: true` propose the defaults and ask
  for confirmation; otherwise use the defaults and record them under `inputsUsed:`.
- `backendRefPort` must be a positive integer and `routeLabels` a string→string map, otherwise stop
  with an `ERROR:` (see [Error handling](#error-handling)).
- Default labels:
  - `app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}`
  - `app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}`
  - `app.kubernetes.io/managed-by: {{ .Values.MANAGED_BY }}`
  - `deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}`
  - `deployer.cleanup/allow: "true"`
  - `app.kubernetes.io/processed-by-operator: istiod`

## Contract — report output

Also write `.mesh-migration/reports/httproute-from-code.yaml` (create the directory; ensure
`.mesh-migration/` is in the repo `.gitignore` — the orchestrator does both when orchestrated):

```yaml
reportSchema: 1
skill: httproute-from-code
status: complete            # complete | partial | failed
generatedAt: <ISO-8601>
inputsUsed: {backendRefName: <v>, backendRefPort: <v>, routeLabels: <map>}
filesGenerated: [<paths>]   # empty when no routes were found
routesGenerated: <N>        # 0 is valid
accessControl:              # omit entirely when both lists are empty
  legacyPublic:             # emitted routes whose legacy type was Public
    - {from: <original path>, source: <file:line>, matchType: prefix}
  forbidden:                # Forbidden:true / allowed:false — never emitted
    - {from: <path>, legacyType: <Public|Private|Internal>, source: <file:line>, matchType: prefix}
  # no `deny` here: code registration has no deny flag
unresolved: []              # {id, question, options, default}; e.g. id microservice-name
needsReview: [<one line per skipped route or problem>]
```

Consumers ignore unknown fields. A consumer seeing a newer `reportSchema` than it knows must stop and
report a contract mismatch.

---

## Step 1 — Discover files

- File → language from extension (`*.go` Go, `*.java` Java). Directory → scan recursively.
- Ignore `vendor/`, `.git/`, `node_modules/`, `testdata/`, `target/`, `src/test/`.
- Mixed Go and Java → process both, merge per microservice.
- **No Go/Java files, or no route registrations found: not an error.** Write the report with
  `status: complete`, `routesGenerated: 0`, `filesGenerated: []`, print "No code-registered routes
  found — nothing to generate (annotation-based routes are handled by the Maven plugin)", and stop.

## Step 2 — Detect route definitions

### Go

```go
registrar.WithRoutes(routeregistration.Route{
    From: "/api/v1/users", To: "/users", RouteType: routeregistration.Public,
    Timeout: 30 * time.Second, Forbidden: false, Gateway: "", Hosts: []string{"api.company.com"},
})
r := routeregistration.Route{...}; registrar.WithRoutes(r)           // variable
routes := []routeregistration.Route{...}; registrar.WithRoutes(routes...) // slice
routeregistration.Route{From: "/mesh", RouteType: routeregistration.Mesh, Gateway: "mesh-gateway"}
routeregistration.Route{From: "/facade", Gateway: "facade"}
```

### Java

> **Annotation-based routes (`@Route`, `@Gateway`) are OUT OF SCOPE** — the Maven plugins
> (`httproutes-generator-spring-maven-plugin` / `httproutes-generator-maven-plugin`) handle them.
> Extract only **`RouteEntry`** builder/constructor call sites.

```java
RouteEntry.builder().from("/api/v1/users").to("/users").type(RouteType.PUBLIC)
    .timeout(30000L).allowed(true).namespace("default").gateway("my-gateway")
    .hosts(Set.of("api.company.com")).build()
new RouteEntry("/api/v1/users", RouteType.PUBLIC)                       // + optional timeout (Long) / namespace (String)
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC, "ns", 30000L) // all 2-/3-/4-/5-arg variants
List.of(new RouteEntry(...)); routes.add(new RouteEntry(...));
processor.postRoutes(List.of(...)); processor.postRoutes(microserviceUrl, routes);
```

## Step 3 — Extract fields

| Field | Go | Java | Default |
|---|---|---|---|
| `from` | `From:` | `.from(...)` / 1st path arg | REQUIRED |
| `to` | `To:` | `.to(...)` / 2nd path arg | same as `from` |
| `routeType` | `RouteType:` | `.type(RouteType.X)` | Public |
| `skip` | `Forbidden: true` | `allowed: false` | false |
| `timeout` | `Timeout:` | `.timeout(...)` / timeout arg | omit |
| `gateway` | `Gateway:` | `.gateway(...)` | derived |
| `hosts`, `namespace` | `Hosts:` | `.hosts(...)`, `.namespace(...)` | read, but **never emitted** (see Step 10) |

Java constructor disambiguation: in `new RouteEntry(path, type, X)` and
`new RouteEntry(from, to, type, X)`, X is a timeout if `Long`/numeric literal, a namespace if `String`.
`RouteType`: `Public`, `Private`, `Internal`, `Mesh` (Go `routeregistration.X` / Java `RouteType.X`);
`RouteType.FACADE` = `Facade`.

## Step 4 — Skip forbidden routes

Skip `Forbidden: true` / `allowed: false`. A skipped route is **never emitted** (no 404 or
no-backend rule). Add it to `accessControl.forbidden[]` and to the summary as
`Skipped: yes (forbidden)`.

## Step 5 — Classify routes

| RouteType / `gateway` | Class | Legacy type recorded |
|---|---|---|
| `Public` / `Private` / `Internal` | gateway group | as given |
| `gateway` = `public-gateway-service` / `private-gateway-service` / `internal-gateway-service` (any RouteType) | gateway group | Public / Private / Internal |
| `Mesh`, or `gateway` is anything else except facade names | Mesh (per distinct gateway value) | — |
| `Facade`, or `gateway` is `facade` / `facade-gateway` | Facade | — |

For every emitted gateway-group route whose legacy type is `Public`, add `{from, source}` to
`accessControl.legacyPublic[]` (original un-reduced `from`; `matchType: prefix`). Never add Private,
Internal, Mesh or Facade routes there.

## Step 6 — One CR per group

| Group | CR name | parentRefs |
|---|---|---|
| gateway group | `<ms>-source-code-routes` | `public-gateway`, `private-gateway`, `internal-gateway-service` |
| Mesh | `<ms>-source-code-mesh-routes` (several mesh gateways: `<ms>-source-code-mesh-<gateway>-routes`, one CR each) | Service named by the route's `gateway` value |
| Facade | `<ms>-source-code-facade-routes` | Service `{{ .Values.SERVICE_NAME }}` |

Generate a CR only for groups that have at least one route. `<ms>` is resolved in Step 7.
Old per-type names (`-public-routes`, `-private-routes`, `-internal-routes`) are no longer produced;
tell the user to delete stale files from earlier runs.

### Step 6a — Reduce to PathPrefix roots (within each CR)

Mirrors the reduction the Maven plugins apply to annotation routes.

1. Cut `from` right before the first segment containing a placeholder (`{var}`) or wildcard (`*`).
   Everything before the cut is the **match** (`PathPrefix`); with no such segment the whole `from`
   is the match. Applies to Go and Java alike.
2. With a cut: the `to` path must end with the same tail shape (ignoring variable names; e.g.
   `/users/{id}` ↔ `/{id}`). The rewrite target is `to` minus that tail. Mismatched tails → skip this
   route with a `needsReview:` entry (do not abort the whole file).
3. No cut: rewrite target is `to` verbatim; omit the rewrite filter when `to == from`.
4. **Merge into shorter roots:** compare by whole **path segments** (`/api/v1/user` is not a prefix of
   `/api/v1/users`). If a shorter root in the same CR, with its rewrite applied to this match,
   already yields this route's target, drop this route; `timeout` becomes the max of both.
5. Two routes reducing to the same match with different rewrite targets → skip both and add a
   `needsReview:` entry naming them.
6. A match reducing to `/` → skip it and add a `needsReview:` entry (it would shadow every other
   route).
7. Output is always `PathPrefix` (optionally with `URLRewrite`/`ReplacePrefixMatch`); never `Exact`
   or `RegularExpression`.

## Step 7 — Resolve microservice name (`<ms>`)

First hit wins: `application.yaml` `microservice.name`; `application.properties`
`microservice.name`; Go env `MICROSERVICE_NAME`; Java `@ConfigProperty(name = "cloud.microservice.name")`;
Go `go.mod` last module path segment; Java `pom.xml` `artifactId`; else use `resolutions` id
`microservice-name`; else with `interactive: true` ask; otherwise use the literal `<microservice-name>`,
add `unresolved:` (id `microservice-name`, question "What is the microservice name for the generated
HTTPRoutes?") and set `status: partial`.

## Step 8 — Convert timeout

Go `time.Duration` literals and sums → `30 * time.Second` = `30s`, `1 * time.Minute` = `1m`,
`time.Minute + 30*time.Second` = `90s`; unsupported expressions → omit.
Java `Long` ms → divisible by 60000: `Xm`; by 1000: `Xs`; else `Xms` (`1500L` = `1500ms`).
Rendered as `timeouts.request` on the rule.

## Step 9 — Sort rules

Sort each CR's rules by reduced match path, most specific first, per
[`path-specificity-sorting`](../path-specificity-sorting/SKILL.md) (segment count, then longer
string, then lexicographic). Never keep source order.

## Step 10 — Render

Wrap **all** CRs in one `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}` block
(`---` separators inside it).

- **parentRefs** — every entry renders `group`, `kind`, `name`:
  - `public-gateway`, `private-gateway`: `group: gateway.networking.k8s.io`, `kind: Gateway`;
  - `internal-gateway-service` and any Mesh/Facade target: `group: ''`, `kind: Service`, no `port`.
- **backendRefs** — every rule renders `group: ''`, `kind: Service`, `name: <backendRefName>`,
  `port: <backendRefPort>`, `weight: 1`.
- **labels** — `routeLabels` verbatim, else the default set, on every CR.
- **Omitted always:** `metadata.namespace` and `spec.hostnames` (Go `Hosts`/Java `hosts` and
  `namespace` are not carried over; if `hosts` was set mention it in the summary — headless-service
  routes need `hostnames` added by hand, see `docs/istio/known-issues.md`).
- Filter order per rule: `URLRewrite` (if any); `timeouts.request` when a timeout exists.

Example (gateway group; defaults shown):

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: mesh-test-service-go-source-code-routes
  labels:
    app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}
    app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}
    app.kubernetes.io/managed-by: {{ .Values.MANAGED_BY }}
    deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}
    deployer.cleanup/allow: "true"
    app.kubernetes.io/processed-by-operator: istiod
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
          name: '{{ .Values.SERVICE_NAME }}'
          port: 8080
          weight: 1
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
          name: '{{ .Values.SERVICE_NAME }}'
          port: 8080
          weight: 1
{{- end }}
```

Output file: `<chartPath>/templates/source-code-httproutes.yaml`, or
`helm-templates/<service name>/templates/source-code-httproutes.yaml` when `chartPath` is not given.

## Step 11 — Summary

Print a table (`# | File | From | To | Legacy type | Group | Timeout | Skipped`) in emitted order,
then forbidden rows. Also print the applied `backendRefName` / `backendRefPort` / `routeLabels` with
their origin (detected, user-provided, default), and — when any exist — the access-control note:

```
Legacy-public routes now registered on public-gateway, private-gateway and internal-gateway-service:
- /api/v1/users (routes.go:12)
Forbidden routes (never emitted): /admin (RouteConfig.java:42)
No AuthorizationPolicy is generated here — see public-gateway-authz-policy.
```

## Error handling

Stop with `ERROR:` + `file:` / `line:` / `reason:` only for: missing `from`/`From`; Mesh route with no
gateway; ambiguous Java constructor argument types; non-integer `backendRefPort`; non-map
`routeLabels`. Individual route reduction failures (Step 6a) are `needsReview:` entries, not stops.

## Non-goals

Do not modify source code (read-only input). Generate only `HTTPRoute` — no VirtualService, Ingress,
GRPCRoute, TCPRoute, `RegularExpression`/`Exact` path matches, 404/no-backend rules, or any
`AuthorizationPolicy` (use the `accessControl` report with
[`public-gateway-authz-policy`](../public-gateway-authz-policy/SKILL.md) if the user asks for a draft).
