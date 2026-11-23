---
name: httproute-from-code
description: >
  Generate Gateway API HTTPRoute CRs from Go or Java route-registration code
  (routeregistration.Route / RouteEntry call sites), plus AuthorizationPolicy DENY
  rules for forbidden routes on the public/private gateways. Use when asked to
  generate HTTPRoutes from source code, convert route registrations to HTTPRoute
  YAML, or extract routes from Go/Java files.
---

# Generate GatewayAPI HTTPRoute CRs from Go or Java route registration code

> **Before you start:** open the sibling skill `regex-routes-migration` with the
> Read tool — the folder next to this skill's folder
> ([link](../regex-routes-migration/SKILL.md)) — and read **all four** of its
> files in full: `SKILL.md`, `procedure.md`, `render.md`, `worked-example.md`.
> Step 4 runs its procedure; the worksheet, the row command, the conflict
> questions and the AuthorizationPolicy template are defined only there. This
> skill is split as well: read [`route-detection.md`](route-detection.md) at
> Step 2 and [`rendering.md`](rendering.md) at Step 10.

## Invocation

Run this skill against a file or directory of route-registration code. Examples:

- `internal/routes`
- `src/main/java/com/example/config/RouteConfig.java`
- `.`

---

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
worksheets: [<.mesh-migration/work/httproute-from-code-<gateway>.md, one per gateway>]
routesGenerated: <N>
authorizationPoliciesGenerated: <N>
unresolved: []              # blocking user decisions, each {id, question, options, default}
                            # e.g. id microservice-name when it fell back to <microservice-name>,
                            # or route-conflict/<#A>-<#B> from regex-routes-migration Step 5
needsReview:
  - <one line per ERROR, ⚠ MANUAL REVIEW note, answered route conflict,
     and the AuthorizationPolicy precondition lines>
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

## Steps 2–3 — Detect route definitions and extract fields

**Read [`route-detection.md`](route-detection.md) now, in full, with the Read
tool.** It lists the Go `routeregistration.Route` and Java `RouteEntry` call-site
patterns to detect (annotation-based Java routes are out of scope — the Maven
plugin owns them), the unified field table (`from`, `to`, `routeType`,
`forbidden`, `namespace`, `timeout`, `gateway`, `hosts`), Java constructor
disambiguation, and RouteType normalization.

---

## Step 4 — Build the gateway worksheets

Routes with `{variables}` and forbidden routes cannot be converted one by one:
Istio has no regex routes, so a path is cut before its first variable, and the
legacy 404s of forbidden routes must become `AuthorizationPolicy` DENY rules.
Read [`regex-routes-migration`](../regex-routes-migration/SKILL.md) in full and
build its worksheets (`.mesh-migration/work/httproute-from-code-<gateway>.md`).

Legacy route registration posts every Public / Private / Internal route to **all
three** border gateways and marks it forbidden (`allowed: false`, 404) on the
gateways wider than its type — and on all of them when the route itself is
forbidden. Add one row per route and gateway accordingly:

| Route | `public-gateway-service` | `private-gateway-service` | `internal-gateway-service` |
|---|---|---|---|
| `Public` | allowed | allowed | allowed |
| `Private` | forbidden `implicit` | allowed | allowed |
| `Internal` | forbidden `implicit` | forbidden `implicit` | allowed |
| any type with `Forbidden: true` / `.allowed(false)` | forbidden `explicit` | forbidden `explicit` | forbidden `explicit` |

A `Mesh` or `Facade` route adds one row to the worksheet of its own gateway
(Step 5): allowed, or forbidden `explicit` when the route is forbidden.

Consumer columns:

| Column | Value |
|---|---|
| `#` | `R<n>`, in discovery order (file path, then line). One route keeps its id in every worksheet |
| `source` | `<file>:<line>` |
| `owner` | the CR of the route's RouteType (Step 6) |
| `path` | `from` |
| `headers` | `-` (route registration has no header matchers) |
| `allowed` / `forbidden` | per the table above |
| `behavior` | **`B1` for every allowed row** — all rules share one backend; never a new id per route |
| `rewrite` | `to` (defaults to `from`); `-` for forbidden rows |

Example — three routes:

```text
R1  Public    /api/v1/svc/{id}        → /svc/{id}
R2  Internal  /api/v1/svc/{id}/admin  → /svc/{id}/admin
R3  Public    /api/v1/svc/debug       Forbidden: true
```

give **three rows in each** border worksheet (the other columns omitted):

| worksheet | # | path | allowed | forbidden | behavior | rewrite |
|---|---|---|---|---|---|---|
| public-gateway-service | R1 | `/api/v1/svc/{id}` | yes | - | B1 | `/svc/{id}` |
| public-gateway-service | R2 | `/api/v1/svc/{id}/admin` | no | implicit | - | - |
| public-gateway-service | R3 | `/api/v1/svc/debug` | no | explicit | - | - |
| private-gateway-service | R1 | `/api/v1/svc/{id}` | yes | - | B1 | `/svc/{id}` |
| private-gateway-service | R2 | `/api/v1/svc/{id}/admin` | no | implicit | - | - |
| private-gateway-service | R3 | `/api/v1/svc/debug` | no | explicit | - | - |
| internal-gateway-service | R1 | `/api/v1/svc/{id}` | yes | - | B1 | `/svc/{id}` |
| internal-gateway-service | R2 | `/api/v1/svc/{id}/admin` | yes | - | B1 | `/svc/{id}/admin` |
| internal-gateway-service | R3 | `/api/v1/svc/debug` | no | explicit | - | - |

On internal, R1 and R2 cut to the same `/api/v1/svc` with `R` `/svc` → one rule
in the Public CR (R2 `merged→R1`). On public / private, R2 becomes a DENY rule
(implicit, covered by R1's cut), R3 a DENY rule (explicit), and `/api/v1/svc` an
exposure DENY rule. On internal, R3 is a rule without `backendRefs`.

**Merge leader** (regex-routes-migration Step 3): when rows of different route
types merge into one rule, the leader is the row with the **widest** type —
`Public`, then `Private`, then `Internal` — so the rule lands in that type's CR.
Its timeout is the largest of the merged rows.

Then run the regex-routes-migration procedure (Steps 1–7) on every worksheet.
Route conflicts become `unresolved:` entries (`interactive: false`) or chat
questions (`interactive: true`).

**Gate before Step 5** — run:

```bash
for g in public private internal; do
  f=.mesh-migration/work/httproute-from-code-$g-gateway-service.md
  printf '%s rows=%s\n' "$f" "$(grep -cE '^\| R[0-9]+ ' "$f")"
done
```

All three files must exist and show the **same** `rows=` count, equal to the
number of Public / Private / Internal routes (each such route has a row in every
border worksheet; Mesh / Facade routes have their own worksheets). Every row has
`emit`; every worksheet has its `## Row command output`, `## Conflicts` and
`## DENY rules` sections. Run the regex-routes-migration self-check (its Step 9).
Do not render anything before this gate passes.

From here on, render **only** what the worksheets say, **per CR, not per
gateway**: for each RouteType CR, its rules are the routes whose `owner` is that
CR and whose rendering (regex-routes-migration Step 8, combined over all three
worksheets) is a rule — a route whose rows are all `merged→…`, `deny` or `none`
produces no rule. A route's rule is rendered once, in its own CR, even though
the route has rows in several worksheets. A CR without any rendered rule is not
generated (an `Internal` route merged into a `Public` rule leaves no internal
CR). A forbidden route that is `rule` on internal (regex-routes-migration
Step 7) is rendered — without `backendRefs` — in its own RouteType's CR.

Every rendered rule with a timeout gets `timeouts: {request: <timeout>}` (Step
8), the largest timeout of the rows merged into it.

---

## Step 5 — Map RouteType → gateways

This decides each CR's `parentRefs`. Which rules a CR gets comes from the
worksheets (Step 4).

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
2. For each RouteType that has at least one rule to render (worksheet `emit`
   `rule` / `exact` per regex-routes-migration Step 8) → generate one CR.
3. `parentRefs` = the fixed gateway list for that RouteType (see table above).
4. `rules[]` = the rendered rules whose owner is that RouteType's CR. A rule merged
   from several types sits in the widest type's CR (Step 4).
5. If no rules of a given RouteType are rendered → skip that CR entirely.

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
Routes that cut to the same `PathPrefix` are merged or reported by
regex-routes-migration (Steps 3 and 5) — never emit two rules with the same match
and different rewrites.

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

Before generating the CR, sort all collected rules so that the most specific
paths appear first in `rules[]`.

Apply the shared procedure in
[`path-specificity-sorting.md`](../path-specificity-sorting/SKILL.md)
— sort on the path each rule emits (the cut `PathPrefix` value, not `from`). That file defines the segment-count ordering,
tie-breaks, a worked example, and why ordering matters across gateway
implementations.

---

## Steps 10–12 — Generate, format and summarize

**Read [`rendering.md`](rendering.md) now, in full, with the Read tool.** It
defines the HTTPRoute CR layout, the mandatory `parentRefs` / `backendRefs`
fields, labels, the naming schema, the full YAML example, the
`AuthorizationPolicy` section, the output file, and the chat summary.

---

## Error handling

Stop and report if:

- `from` / `From` is missing
- Mesh/MESH route has no gateway field
- RouteType conflicts with explicit gateway value
- Java constructor argument types are ambiguous
- No routes detected in any file

A path with a partial variable segment (`/v{version}/x`) is **not** an error: it
is routed by its cut, and regex-routes-migration flags it only when it ends up in
a DENY rule.
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

Only HTTPRoute, plus the AuthorizationPolicy DENY rules for forbidden routes on
the public and private gateways.
