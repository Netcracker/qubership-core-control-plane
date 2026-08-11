---
name: core-mesh-crs-to-istio
description: >
  Convert Qubership Cloud-Core Mesh CRs in a Helm chart (FacadeService, Gateway,
  RouteConfiguration, StatefulSession, LoadBalance, HttpFilters) to Istio Ambient
  Mesh resources (Gateway API Gateway + HTTPRoute, DestinationRule,
  TrafficExtension), keeping the chart deployable on both mesh types. Use when
  asked to migrate, convert, or transform mesh CRs in a Helm chart to Istio, or
  as a sub-skill of core-mesh-to-istio-migration.
---

# Qubership Cloud Core Mesh CRs → Istio Ambient Mesh — Helm Transformer

## Overview

This skill transforms Helm chart templates from the homegrown **Cloud Core Mesh** model to
**Istio Ambient Mesh** in a single pass over the chart:

| Source CR | Output |
|---|---|
| `Gateway` (`spec.type: ingress` / `egress`) | Istio `Gateway` (gatewayClassName: istio) + HTTPRoute parents |
| `Gateway` (`spec.type: mesh`) | omitted — routes become east-west `HTTPRoute` (parents are Services) |
| `FacadeService` | `Service` (HTTPRoute parent); also resolves mesh gateway name via `spec.gateway` |
| `RouteConfiguration` | `HTTPRoute` per virtualService; rule-level `statefulSession` → `DestinationRule` |
| `StatefulSession` (standalone) | `DestinationRule` with `consistentHash.httpCookie` |
| `LoadBalance` | `DestinationRule` with `consistentHash.*` |
| `HttpFilters` + `RouteConfiguration` rules with `luaFilter` | `TrafficExtension` (requires Istio ≥ 1.30) |

Source CRs appear as `core.netcracker.com/v1` `Mesh` (with `subKind`) or as legacy
`nc.core.mesh/v2` / `nc.core.mesh/v3` documents.

The transformation keeps charts deployable to **both mesh types** simultaneously by wrapping
old resources in `{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` and new Istio resources in
`{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}`. Dataplane is **ambient only**.

---

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Chart or templates folder to transform |
| `interactive` | bool | no | `true` only when a user invokes the skill directly in their own session; orchestrators and sub-agent wrappers pass `false` |
| `resolutions` | map `<unresolved id>: <answer>` | no | Answers to a previous run's `unresolved:` entries |

With `interactive: false` (the default for orchestrated and sub-agent runs),
never ask the user: every blocking question becomes an `unresolved:` entry.
Skip only the work that depends on the answer, continue with everything else,
and set `status: partial` when writing the final report. With
`interactive: true`, ask blocking questions in chat and wait for the answer. A
sub-agent has no user channel — its questions die in its transcript — so a
delegated run must always be `interactive: false`.

### Outputs

In addition to the chat Output Summary, write a machine-readable report to
`.mesh-migration/reports/core-mesh-crs-to-istio.yaml` (create the directory, and ensure
`.mesh-migration/` is listed in the repo's `.gitignore` — reports are working
files, never committed; the orchestrator handles both in orchestrated runs):

```yaml
reportSchema: 1
skill: core-mesh-crs-to-istio
status: complete            # complete | partial (unresolved items block part of the output)
generatedAt: <ISO-8601>
filesModified: [<paths>]
filesGenerated: [<paths>]
resources:
  facadeService: <N>
  gatewayIngressEgress: <N>
  gatewayMesh: <N>
  routeConfiguration: <N>
  statefulSession: <N>
  loadBalance: <N>
  luaFilters: <N>
  skipped: <N>
backendRef:
  name: <value or null>
  port: <value or null>
  unresolvedReason: <string or null>
labels:
  values: <map or null>
  unresolvedReason: <string or null>
unresolved:                 # empty when status is complete
  - id: gateway/<gateway-name>
    question: "Gateway '<gateway-name>' is referenced in routes but not defined in this chart — ingress or mesh?"
    options: [ingress, mesh]
    default: null
    referencedBy: [<CR names>]
needsReview:
  - <one line per ⚠ MANUAL REVIEW hit>
```

Consumers must ignore unknown report fields. A consumer that sees a
`reportSchema` newer than its own documentation must stop and report a contract
mismatch instead of guessing field meanings.

### Side effects

Modifies only mesh-CR files and their `-istio` siblings, `values.yaml`, and
`values.schema.json` under `chartPath`, plus the report file.

---

## Scope — only touch mesh-entity files

This skill operates **exclusively** on the Core Mesh custom resources listed above.
Everything else in the chart must be left byte-for-byte unchanged.

**Only modify a file if it actually contains one of those mesh CR documents**
(detected in Step 1). For such files you may wrap the mesh CR documents in Core
guards and create the `-istio.yaml` sibling — but do not rewrite unrelated
documents in the same file.

**Do NOT touch** (do not edit, wrap, reformat, or generate siblings for):

- Deployments, Services, ConfigMaps, Secrets, ServiceAccounts, HPAs, PVCs,
  Ingresses, NetworkPolicies, CronJobs, or any other non-mesh kind.
- `_helpers.tpl` / any `*.tpl` files and the named template helpers
  (`{{- define }}` / `{{- include }}`) they contain. Do **not** trigger on a
  template helper just because it appears in a chart — only the rendered mesh CR
  documents are in scope.
- `Chart.yaml`, `NOTES.txt`, `.helmignore`, CRD definitions, tests, and docs.
- `values.yaml` / `values.schema.json` — the **only** exception, edited solely
  to add the `SERVICE_MESH_TYPE` key per Step 7. Make no other value changes.

If a mesh CR is produced by a template helper (a `{{- include }}` that renders a
mesh CR), do not edit the helper — flag it with `# ⚠ MANUAL REVIEW` and
leave it to the user.

`HttpFilters` entries for `wasmFilters` / `extAuthzFilter` alone (no `luaFilters`)
are out of scope.

---

## Step-by-Step Transformation Procedure

Log each step in chat.

### Step 1 — Discover

Scan the folder once for files containing any Core Mesh CR:

```bash
grep -rl \
  -e 'kind: FacadeService' \
  -e 'kind: Gateway' \
  -e 'kind: Mesh' \
  -e 'kind: RouteConfiguration' \
  -e 'kind: StatefulSession' \
  -e 'kind: LoadBalance' \
  -e 'kind: HttpFilters' \
  --include="*.yaml" --include="*.yml" \
  <folder>
```

List each discovered file and its contained kinds before proceeding. Do not proceed
with transformation until the full file list is confirmed.

**Scope gate:** the files matched here are the **only** files this skill may
modify (plus `values.yaml` / `values.schema.json` in Step 7). A document is in
scope only if its `apiVersion`/`subKind` identifies a Core Mesh CR:

- `apiVersion: core.netcracker.com/v1`, `kind: Mesh` with a mesh `subKind`, or
- legacy `apiVersion: nc.core.mesh/v2` / `nc.core.mesh/v3` documents.

Matching `kind: Gateway` may catch unrelated kinds — drop false positives (for
example generated Istio output files). Ignore `*.tpl` files and files that only
reference mesh values through helpers; flag helper-rendered mesh CRs for manual
review instead.

### Step 2 — Resolve gateway types (mandatory before generating HTTPRoutes)

**Definition — resolved vs unresolved gateway:**

- **Resolved:** a gateway name is resolved if its type is known. Resolution uses the following priority order (higher wins):

    1. **Well-known platform gateway name** — takes highest priority regardless of any local CR:
        * `"egress-gateway"` → **egress** type
        * `"public-gateway-service"` → **ingress** type
        * `"private-gateway-service"` → **ingress** type
        * `"internal-gateway-service"` → **mesh** type (parentRef = Service;
          Lua and other extensions target the mesh Gateway from discovery,
          `waypoint` on Cloud Core)
    2. **Gateway CR in the scanned chart/folder** — used only if the name is not well-known:
        - `spec.gatewayType: ingress` or `egress` → ingress/egress Gateway
        - `spec.gatewayType: mesh` or absent → mesh (parentRef = Service)
    3. **FacadeService reference** — used only if no Gateway CR and name is not well-known:
        - Appears as `spec.gateway` of a FacadeService → treat as **mesh**
        - FacadeService without `spec.gateway` → resolved gateway = `FacadeService.metadata.name + "-gateway"` (mesh)

- **Unresolved:** a gateway name referenced in any `spec.gateways[]` that the rules above cannot type.

**Checkpoint — do not generate Istio output until this is done:**

1. Collect every gateway name referenced in `spec.gateways[]` across all discovered CRs.
2. Resolve each using the `resolutions` input first (key `gateway/<name>`), then
   Gateway CRs and FacadeService resources in the **current chart/folder**
   (strict name match).
3. For every gateway name still **unresolved**:
   - **`interactive: true`:** ask the user explicitly (in your reply): "Gateway '<name>' is referenced in routes but not defined in this chart. Should it be treated as **ingress** (HTTPRoute parentRef = Gateway) or **mesh** (HTTPRoute parentRef = Service)?" Wait for the answer.
   - **`interactive: false`:** do not ask. Skip Istio output for CRs that
     reference only unresolved gateways, record each under `unresolved:` in the
     report (id `gateway/<name>`, options `[ingress, mesh]`), and finish with
     `status: partial`. The caller obtains the answers and delivers them via the
     `resolutions` input; on the follow-up run, process only the previously
     skipped CRs — already-wrapped originals and already-generated `-istio`
     files must not be produced twice (see the Step 3 and Step 4 idempotency
     checks).
   - **Do not infer** gateway type from the gateway name alone.

### Step 3 — Wrap originals in Core condition

**Idempotency check:** a document already enclosed in
`{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` … `{{- end }}` is left
untouched — never nest a second guard. This matters on a follow-up run with
`resolutions`, where previously processed documents are already wrapped.

In the **original files**, wrap each not-yet-guarded mesh CR document with the
Core guard:

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}
apiVersion: core.netcracker.com/v1
kind: Mesh
# ... original content unchanged ...
{{- end }}
```

Legacy declarative files keep their `nc.core.mesh/*` apiVersion inside the guard.
For multi-document YAML files (separated by `---`): wrap each document individually.

### Step 4 — Generate Istio files (single pass)

**Idempotency check:** if the `-istio` sibling already exists, add only the
resources it is missing; do not duplicate documents a previous run generated.

Create a **new file** for each original, with `-istio` before the extension:

```text
templates/gateway.yaml           → templates/gateway-istio.yaml
templates/route-config.yaml      → templates/route-config-istio.yaml
templates/stateful-session.yaml  → templates/stateful-session-istio.yaml
templates/load-balance.yaml      → templates/load-balance-istio.yaml
templates/http-filters.yaml      → templates/http-filters-istio.yaml
```

Each autogenerated file must be wrapped in the ISTIO guard:

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
# ... Istio resources ...
{{- end }}
```

Process CR kinds in this order, following the mapping reference for each:

1. `Gateway` CRs → [gateway-mapping.md](gateway-mapping.md)
2. `FacadeService` CRs → [facade-service-mapping.md](facade-service-mapping.md)
3. List in chat all resolved gateways: mesh gateways and ingress/egress gateways.
4. `RouteConfiguration` CRs → [route-configuration-mapping.md](route-configuration-mapping.md);
   rule-level `statefulSession` → [stateful-session-rule-mapping.md](stateful-session-rule-mapping.md)
   (DestinationRule in the same output file, after the HTTPRoute, `---` separated)
5. Sort each HTTPRoute's `rules[]` by path specificity per the shared procedure in
   [path-specificity-sorting](../path-specificity-sorting/SKILL.md)
   (sort on each rule's `match.prefix` / `match.path` / `match.regExp` value)
6. Standalone `StatefulSession` → [stateful-session-mapping.md](stateful-session-mapping.md)
7. `LoadBalance` → [load-balance-mapping.md](load-balance-mapping.md)
8. `HttpFilters` + `RouteConfiguration` rules with `luaFilter` →
   [lua-filter-mapping.md](lua-filter-mapping.md). Input is one pair with matching
   gateways; when a chart defines Lua on several gateways, process once per pair.

**One DestinationRule per host:** a rule-level `statefulSession` and a standalone
`StatefulSession` / `LoadBalance` CR may target the same `spec.host`, and Istio's
behavior with multiple DestinationRules on one host is merge-order-dependent.
Emit exactly one DestinationRule per host: when several sources define the same
policy, merge; when policies conflict, generate from the standalone CR and add a
`# ⚠ MANUAL REVIEW` comment naming the dropped source and its policy.

### Step 5 — Detect the service backend reference

While processing `RouteConfiguration` CRs, collect the backend reference of the
migrated service so downstream tooling (code-generated HTTPRoutes, Maven plugin)
can reuse the **same** `name` / `port`. This relies on the assumption that **one
migrated chart contains only routes for its own service**.

1. From every `RouteDestination.endpoint` (see
   [route-configuration-mapping.md](route-configuration-mapping.md) →
   "Endpoint to backendRef resolution"), collect the parsed `(name, port)` pairs.
2. Exclude destinations whose `name` is a well-known platform gateway service
   (`public-gateway-service`, `private-gateway-service`, `internal-gateway-service`,
   `egress-gateway`).
3. Determine the result:
   - **Exactly one distinct `(name, port)` remains** → that is the detected
     `backendRefName` / `backendRefPort`. Preserve Helm expressions verbatim.
   - **No destinations or more than one distinct backend** → report as
     **unresolved** and explain why (none found / conflicting values listed).

Report the result in the Output Summary. Do not ask the user here — resolution
is the orchestrator's job.

### Step 6 — Capture labels applied to generated resources

Capture the label set applied to generated Istio resources, using
[labels.md](labels.md) as the source of truth.

1. Resolve the final label map for generated resources (including Helm template
   expressions if used), merging common helpers with local overrides.
2. If labels cannot be resolved unambiguously (for example helper indirection),
   mark labels as **unresolved** and include why.
3. Record the result in the Output Summary as `Detected output labels`.
4. If `.mesh-migration/MIGRATION_LOG.md` exists: append a **Done** entry when
   resolved, or a **Needs review** entry (with reason and suggested action) when
   unresolved.

Do not invent missing label values. If uncertain, mark unresolved.

### Step 7 — Update values.yaml

* Add `SERVICE_MESH_TYPE = Core` to the end of values.yaml.
* Update values.schema.json accordingly with exact description:

```json
    "SERVICE_MESH_TYPE": {
      "$id": "#/properties/SERVICE_MESH_TYPE",
      "type": "string",
      "title": "The SERVICE_MESH_TYPE schema",
      "description": "Service mesh type. Use `Core` for Cloud Core Mesh or `Istio` for Istio Ambient Mesh.",
      "enum": ["Istio", "Core"],
      "default": "Core",
      "internal": true
    }
```

### Step 8 — Preserve Helm templating

- Keep all `{{ .Values.X }}` expressions — never hardcode parameterized values
- Always use `{{ .Release.Namespace }}` for namespace fields
- Preserve `{{- if }}` conditionals, `{{- range }}` loops, `{{- include }}` calls
- If a named helper produces mesh-specific output, add `# ⚠ MANUAL REVIEW`
- Do not add comments to generated resources

### Step 9 — Validation checklist

After generating all files, verify:

- [ ] Every gateway name was either resolved from the chart or confirmed by the user before generating HTTPRoutes
- [ ] Every original file's CRs are wrapped in `Core` condition
- [ ] Every autogenerated file's resources are wrapped in `Istio` condition
- [ ] Every `FacadeService` produces a `Service` (no `FacadeService` kind in Istio output); no `mesh` type Gateways — only their derived HTTPRoutes
- [ ] All `ingress` / `egress` type Gateways (and `egress-gateway` by name) produce an Istio Gateway
- [ ] `RouteConfiguration` → HTTPRoute parentRefs correctly use Gateway or Service kind
- [ ] Each HTTPRoute's `rules[]` are sorted by path specificity (most specific first)
- [ ] Rule-level `statefulSession` → DestinationRule added to the same output file
- [ ] `StatefulSession` with cookie → DestinationRule generated; delete/disabled requests skipped
- [ ] `StatefulSession` with `hostname`/`port` → DestinationRule generated per the endpoint-level targeting rules **and** `# ⚠ MANUAL REVIEW` comment added
- [ ] `LoadBalance` → DestinationRule from first policy; multi-policy cases get `# ⚠ MANUAL REVIEW` comment
- [ ] At most one DestinationRule per `spec.host` across all generated files; conflicts flagged
- [ ] `HttpFilters` + `RouteConfiguration` with `luaFilter` → `TrafficExtension` with `targetRefs`, `phase: STATS`, path guard in `inlineCode`
- [ ] Unresolved `luaFilter` names or gateway context → `# ⚠ MANUAL REVIEW` comment added
- [ ] `overridden: true` on any CR → `# ⚠ MANUAL REVIEW` comment added
- [ ] No hardcoded values where Helm expressions existed
- [ ] Only mesh-CR files were modified (plus `values.yaml` / `values.schema.json`)
- [ ] YAML is valid (no unclosed blocks, correct indentation)

---

## Fields that MUST be flagged with `⚠ MANUAL REVIEW`

When the listed field is non-empty / non-nil on the source CR, omit it from the
Istio output (unless a mapping says otherwise) **and** leave a `# ⚠ MANUAL REVIEW`
comment on the generated resource (or on the Core-guarded original if the
resource is fully omitted).

| Source | Field | Trigger |
|---|---|---|
| `RouteConfiguration.spec` | `overridden` | non-empty |
| `VirtualService` | `rateLimit` / `overridden` | non-empty |
| `VirtualService.hosts[]` | `*` host | appears on an east-west (mesh) route |
| `RouteDestination` | `cluster` / `httpVersion` / `circuitBreaker` / `tcpKeepalive` | non-empty |
| `RouteV3.Rule` | `idleTimeout` / `rateLimit` / `deny` | non-empty / non-nil |
| `Rule` | `luaFilter` | name not found in `HttpFilters.spec.luaFilters` |
| `StatefulSession.spec` | `hostname` / `port` | non-empty |
| `StatefulSession.spec` / `LoadBalance.spec` | `overridden` | `true` |
| `LoadBalance.spec.policies` | more than one entry | — |
| `DestinationRule` | conflicting policies for one `spec.host` | rule-level vs standalone source |
| `TrafficExtension` | path-scoped script | same `luaFilter` name used on rules with different prefixes |
| `HttpFilters` / `RouteConfiguration` | `gateways` | gateway context cannot be classified |
| `FacadeService` | neither `spec.port` nor `spec.gatewayPorts` | — |
| Any template helper | `{{- include ... }}` renders mesh CRs | — |

---

## Output Summary (report after completion)

Write the contract report file first (see [Contract → Outputs](#outputs)), then
print this summary in chat:

```
Transformation complete.

Files modified:     <list> (Core condition wrapper added)
Files generated:    <list> (Istio resources)

Resources transformed:
  FacadeService             → Service (<N> instances)
  Gateway/ingress/egress    → Istio Gateway + HTTPRoute (<N> instances)
  Gateway/mesh              → omitted, east-west HTTPRoute only (<N> instances)
  RouteConfiguration        → HTTPRoute (<N> instances)
  StatefulSession           → DestinationRule (<N> instances)
  LoadBalance               → DestinationRule (<N> instances)
  Lua filters               → TrafficExtension (<N> instances)
  Skipped (no cookie / disabled / no policies / no luaFilters): <N>

Detected backend reference (for code-generated HTTPRoutes / Maven plugin):
  backendRefName: <name or "unresolved">
  backendRefPort: <port or "unresolved">
  # if unresolved, state why: no RouteConfiguration destinations found
  #                           | conflicting backends: <list of name:port>

Detected output labels (for Maven plugin / code-generated HTTPRoutes):
  labels: <k1=v1, k2=v2, ... or "unresolved">
  # if unresolved, state why: helper indirection not resolvable
  #                           | conflicting label definitions

Items needing manual review:
  <list every ⚠ MANUAL REVIEW hit — one line per hit>
```

---

## Reference Files

Read the mapping for a CR kind before transforming it — they contain schemas,
field-by-field rules, and full examples:

- [facade-service-mapping.md](facade-service-mapping.md) — FacadeService → Service
- [gateway-mapping.md](gateway-mapping.md) — Gateway → Istio Gateway
- [route-configuration-mapping.md](route-configuration-mapping.md) — RouteConfiguration → HTTPRoute
- [stateful-session-rule-mapping.md](stateful-session-rule-mapping.md) — Rule-level StatefulSession → DestinationRule
- [stateful-session-mapping.md](stateful-session-mapping.md) — Standalone StatefulSession → DestinationRule
- [load-balance-mapping.md](load-balance-mapping.md) — LoadBalance → DestinationRule
- [lua-filter-mapping.md](lua-filter-mapping.md) — HttpFilters + RouteConfiguration → TrafficExtension
- [labels.md](labels.md) — Common label resolution
- [path-specificity-sorting](../path-specificity-sorting/SKILL.md) — Sort HTTPRoute `rules[]` by path specificity (shared with `httproute-from-code`)
