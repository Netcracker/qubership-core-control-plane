---
name: core-mesh-crs-to-gatewayapi
description: >
  Transform Qubership Cloud Core Mesh Helm templates to Istio Ambient Mesh equivalents.
  Use this skill whenever the user asks to migrate, convert, transform, or upgrade Helm charts
  from Qubership's homegrown mesh (FacadeService, Gateway, RouteConfiguration CRs) to Istio
  (Gateway API Gateway + HTTPRoute resources). Trigger on any mention of: mesh migration,
  istio transformation, FacadeService, RouteConfiguration, SERVICE_MESH_TYPE, qubership mesh,
  helm chart migration to istio, or converting CRDs for istio ambient mesh.
  Always use this skill when working with Qubership platform repos containing mesh-related Helm templates.
---

# Qubership Cloud Core Mesh → Istio Ambient Mesh — Helm Transformer

## Overview

This skill transforms Helm chart templates from the
homegrown **Cloud Core Mesh** model to **Istio Ambient Mesh** (Gateway API).

The transformation keeps charts deployable to **both mesh types** simultaneously by wrapping
old resources in `{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` and new Istio resources in
`{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}`.

---

## Scope — only touch mesh-entity files

This skill operates **exclusively** on the Core Mesh custom resources it
converts: `FacadeService`, `Gateway`, `RouteConfiguration` (and `Mesh` CRs with
those `subKind`s). Everything else in the chart must be left byte-for-byte
unchanged.

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
  to add the `SERVICE_MESH_TYPE` key per Step 6. Make no other value changes.

The lone exception to "mesh CRs only" is Step 6 (`values.yaml` /
`values.schema.json` for `SERVICE_MESH_TYPE`). If a mesh CR is produced by a
template helper (a `{{- include }}` that renders FacadeService/Gateway/
RouteConfiguration), do not edit the helper — flag it with
`# ⚠ MANUAL REVIEW REQUIRED` per Step 7 and leave it to the user.

---

### Gateway Types and Their Disposition

`Gateway` kind has a `spec.type` field that determines the transformation:

| Gateway type | Transformation |
|---|---|
| `ingress` (custom ingress) | → Istio `Gateway` (gatewayClassName: istio) + `HTTPRoute` parents pointing to it |
| `mesh` | **Omitted** — routes become east-west `HTTPRoute` (parents are Services) |

### FacadeService Disposition

`FacadeService` instances are **omitted** from the Istio output.
`FacadeService` is equal to Gateway of `mesh` type, but also it can be linked to existing `Gateway` - by reference in `spec.gateway` field

### RouteConfiguration Disposition

`RouteConfiguration` maps to one or more **Gateway API `HTTPRoute`** resources.
Parent ref kind (Gateway vs Service) is determined by the type of the referenced Gateway CR.

---

## Target Model (Istio Ambient Mesh)

All output resources use **Gateway API** (not Istio-native CRDs like `VirtualService`):
→ See reference files for complete field-by-field rules:

- [facade-service-mapping.md](facade-service-mapping.md) — FacadeService → Service mapping
- [gateway-mapping.md](gateway-mapping.md) — Gateway → Istio Gateway mapping
- [route-configuration-mapping.md](route-configuration-mapping.md) — RouteConfiguration → HTTPRoute mapping
- [labels.md](labels.md) — Common label resolution rules

---

## Step-by-Step Transformation Procedure

Note:

  1. Log each step in chat


### Step 1 — Discover

Scan folder for files containing Core Mesh CRs:

```bash
grep -rl \
  -e 'kind: FacadeService' \
  -e 'kind: Gateway' \
  -e 'kind: Mesh' \
  --include="*.yaml" --include="*.yml" \
  <folder>
```

List each discovered file and its contained kinds before proceeding. Do not proceed
with transformation until the full file list is confirmed.

**Scope gate:** the files matched here are the **only** files this skill may
modify (plus `values.yaml` / `values.schema.json` in Step 6). A file is in scope
only if it contains an actual `FacadeService`, `Gateway`, or `RouteConfiguration`
CR document. Specifically:

- Ignore `*.tpl` / `_helpers.tpl` and any file that has no mesh CR document, even
  if it references mesh values or includes a helper.
- Matching `kind: Gateway` may catch unrelated kinds — confirm the
  `apiVersion`/`subKind` identifies a Core Mesh CR before treating a file as in
  scope; drop false positives from the list.
- If a mesh CR is rendered indirectly by a `{{- include }}` helper, keep the
  helper file out of scope and flag it for manual review (Step 7); do not edit
  the helper.


### Step 3 — Wrap originals in Core condition

In the **original files**, wrap each CR document with the Core guard:

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}
apiVersion: core.qubership.org/v1
kind: FacadeService
# ... original content unchanged ...
{{- end }}
```

For multi-document YAML files (separated by `---`): wrap each document individually.

### Step 4 — Generate Istio files

Create a **new file** for each original, with `-istio` before the extension:

```
templates/gateway.yaml       → templates/gateway-istio.yaml
templates/route-config.yaml  → templates/route-config-istio.yaml
```

Each autogenerated file must be wrapped in the ISTIO guard:

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
# ... Istio Gateway API resources ...
{{- end }}
```

### Step 5 — Resolve gateway types (mandatory before generating HTTPRoutes)

**Definition — resolved vs unresolved gateway:**

- **Resolved:** A gateway name is resolved if its type is known. Resolution uses the following priority order (higher wins):

    1. **Well-known platform gateway name** — takes highest priority regardless of any local CR:
        * `"egress-gateway"` → **egress** type
        * `"public-gateway-service"` → **ingress** type
        * `"private-gateway-service"` → **ingress** type
        * `"internal-gateway-service"` → **ingress** type
    2. **Gateway CR in the scanned chart/folder** — used only if the name is not well-known:
        - `spec.gatewayType: ingress` or `egress` → ingress/egress Gateway
        - `spec.gatewayType: mesh` or absent → mesh (parentRef = Service)
    3. **FacadeService reference** — used only if no Gateway CR and name is not well-known:
        - Appears as `spec.gateway` of a FacadeService → treat as **mesh**
        - FacadeService without `spec.gateway` → resolved gateway = `FacadeService.metadata.name + "-gateway"` (mesh)

- **Unresolved:** A gateway name is unresolved if it is referenced in any RouteConfiguration/Mesh `spec.gateways[]` but is **not** resolved by the rules above (e.g. no Gateway CR and no FacadeService with that gateway name in the chart).

**Checkpoint — do not generate Istio HTTPRoutes until this is done:**

1. Collect every gateway name referenced in RouteConfiguration/Mesh `spec.gateways[]`.
2. Resolve each using only Gateway CRs and FacadeService resources in the **current chart/folder** (strict name match).
3. For every **unresolved** gateway name:
   - **Ask the user explicitly** (in your reply): "Gateway '<name>' is referenced in routes but not defined in this chart. Should it be treated as **ingress** (HTTPRoute parentRef = Gateway) or **mesh** (HTTPRoute parentRef = Service)?"
   - **Do not infer** gateway type from the gateway name alone (e.g. "ingress" in the name does not make it resolved).
   - **Do not generate** any `routes-configuration-istio.yaml` (or HTTPRoute output) until the user has specified the type for every unresolved gateway.
4. If any RouteConfiguration references only unresolved gateways and the user has not yet answered, do not process that RouteConfiguration; wait for the user's response.

### Step 5b — Apply transformation rules (after all gateway types are known)

**Attention to mandatory requirements in Step 5**

Sequence:
1. Process all Gateway CRs
2. Process all FacadeService CRs
3. List in chat all resolved gateways: mesh gateways (from FacadeService or Gateway mesh) and ingress/egress gateways (from Gateway CRs).
4. Process all RouteConfiguration CRs using the resolved list plus user-provided types for any previously unresolved gateways.
5. Sort each HTTPRoute's `rules[]` by path specificity using the shared procedure
   in [shared/path-specificity-sorting.md](../shared/path-specificity-sorting.md)
   (sort on each rule's `match.prefix` / `match.path` / `match.regExp` value).


### Step 5c — Detect the service backend reference

While processing `RouteConfiguration` CRs, collect the backend reference of the
migrated service so downstream tooling (code-generated HTTPRoutes, Maven plugin)
can reuse the **same** `name` / `port`. This relies on the assumption that **one
migrated chart contains only routes for its own service**, so every self-route
destination resolves to the same backend.

Procedure:

1. From every `RouteDestination.endpoint` (see
   [route-configuration-mapping.md](route-configuration-mapping.md) →
   "Endpoint to backendRef resolution"), collect the parsed `(name, port)` pairs.
2. Exclude destinations whose `name` is a well-known platform gateway service
   (`public-gateway-service`, `private-gateway-service`, `internal-gateway-service`,
   `egress-gateway`) — these are not the service's own backend.
3. Determine the result:
   - **Exactly one distinct `(name, port)` remains** → that is the detected
     `backendRefName` / `backendRefPort`. Preserve Helm expressions verbatim
     (e.g. `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}`).
   - **No destinations** (e.g. no `RouteConfiguration` CRs) or **more than one
     distinct backend** → report `backendRefName` / `backendRefPort` as
     **unresolved** and explain why (none found / conflicting values listed).

Report the result in the Output Summary (see "Detected backend reference").
Do not ask the user here — resolution/prompting is the orchestrator's job.

### Step 6 — Update values.yaml

* Add `SERVICE_MESH_TYPE = Core` to the end of values.yaml. 
* Update values.schema.json accordingly with exact description

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


### Step 7 — Preserve Helm templating

- Keep all `{{ .Values.X }}` expressions — never hardcode parameterized values
- Always use `{{ .Release.Namespace }}` for namespace fields
- Preserve `{{- if }}` conditionals, `{{- range }}` loops, `{{- include }}` calls
- If a named helper produces mesh-specific output, add `# ⚠ MANUAL REVIEW REQUIRED`
- Do not add comments to generated resources

### Step 8 — Validation checklist

After generating all files, verify:

- [ ] Every gateway name in RouteConfiguration was either resolved from the chart (Gateway CR / FacadeService) or the user was asked and confirmed its type (ingress vs mesh) before generating HTTPRoutes
- [ ] Every original file's CR's are wrapped in `Core` condition
- [ ] Every autogenerated file's resources are wrapped in `Istio` condition
- [ ] No `FacadeService` appears in autogenerated files (Service instead of it)
- [ ] No `mesh` type Gateways appear — only their derived HTTPRoutes
- [ ] All `ingress`, `egress` type Gateways, gateway with name `egress-gateway` produce a Gateway
- [ ] `RouteConfiguration` → HTTPRoute parentRefs correctly use Gateway or Service kind
- [ ] No hardcoded values where Helm expressions existed
- [ ] Only mesh-CR files were modified (plus `values.yaml` / `values.schema.json` for `SERVICE_MESH_TYPE`); no `*.tpl` helpers, Deployments, Services, or other non-mesh files were touched
- [ ] Each HTTPRoute's `rules[]` are sorted by path specificity (most specific first) per [shared/path-specificity-sorting.md](../shared/path-specificity-sorting.md)
- [ ] YAML is valid (no unclosed blocks, correct indentation)
- [ ] `⚠ MANUAL REVIEW REQUIRED` comments added for every encountered unsupported/omitted field (see list below)

### Fields that MUST be flagged with `⚠ MANUAL REVIEW REQUIRED`

When the listed field is non-empty / non-nil on the source CR, omit it from the Istio output **and** leave a `# ⚠ MANUAL REVIEW REQUIRED` comment on the generated resource (or on the Core-guarded original if the resource is fully omitted).

| Source | Field | Trigger |
|---|---|---|
| `RouteConfiguration.spec` | `overridden` | non-empty |
| `VirtualService` | `rateLimit` | non-empty |
| `VirtualService` | `overridden` | non-empty |
| `VirtualService.hosts[]` | `*` host | appears on an east-west (mesh) route |
| `RouteDestination` | `cluster` | non-empty |
| `RouteDestination` | `httpVersion` | non-empty |
| `RouteDestination` | `circuitBreaker` | non-empty |
| `RouteDestination` | `tcpKeepalive` | non-empty |
| `RouteV3.Rule` | `idleTimeout` | non-nil |
| `RouteV3.Rule` | `statefulSession` | non-nil |
| `RouteV3.Rule` | `rateLimit` | non-empty |
| `RouteV3.Rule` | `deny` | non-nil |
| `RouteV3.Rule` | `luaFilter` | non-empty |
| `StatefulSession` (whole block) | any field set | — |
| `FacadeService` | neither `spec.port` nor `spec.gatewayPorts` | — |
| Any template helper | `{{- include ... }}` returns mesh-specific CRs | — |

---

## Output Summary (report after completion)

```
Transformation complete.

Files modified:     <list> (Core condition wrapper added)
Files generated:    <list> (Istio resources)

Resources transformed:
  FacadeService             → Service (<N> instances)
  Gateway/ingress/egress    → Istio Gateway + HTTPRoute (<N> instances)
  Gateway/mesh              → omitted, east-west HTTPRoute only (<N> instances)
  RouteConfiguration        → HTTPRoute (<N> instances)

Detected backend reference (for code-generated HTTPRoutes / Maven plugin):
  backendRefName: <name or "unresolved">
  backendRefPort: <port or "unresolved">
  # if unresolved, state why: no RouteConfiguration destinations found
  #                           | conflicting backends: <list of name:port>

Items needing manual review:
  <list every omitted `⚠ MANUAL REVIEW REQUIRED` — one line per hit, e.g.:
   - rateLimit / overridden on VirtualService <name>
   - '*' host on east-west RouteConfiguration <name>
   - cluster / httpVersion /
     circuitBreaker / tcpKeepalive on RouteDestination of <name>
   - deny / idleTimeout / statefulSession / rateLimit / luaFilter
     on Rule <path> of <name>
   - FacadeService <name> has no port defined
   - helper {{- include "<name>" }} produces mesh CRs — guards added manually
```

---

## Reference Files

Read these before transforming — they contain schemas, field mappings, and full examples:

- [facade-service-mapping.md](facade-service-mapping.md) — FacadeService → Service
- [gateway-mapping.md](gateway-mapping.md) — Gateway → Istio Gateway
- [route-configuration-mapping.md](route-configuration-mapping.md) — RouteConfiguration → HTTPRoute
- [labels.md](labels.md) — Common label resolution
- [../shared/path-specificity-sorting.md](../shared/path-specificity-sorting.md) — Sort HTTPRoute `rules[]` by path specificity (shared with `httproute-from-code`)
