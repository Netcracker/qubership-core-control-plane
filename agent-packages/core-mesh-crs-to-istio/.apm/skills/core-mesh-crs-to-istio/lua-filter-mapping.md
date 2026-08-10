## Lua filter → TrafficExtension

> **Precondition:** requires Istio ≥ 1.30 (`TrafficExtension` API). Older Istio versions are not supported
> by this migration.

Source (two configurations applied via the control-plane apply API, registration order does not matter):

```text
apiVersion: nc.core.mesh/v3
kind: HttpFilters          — defines named Lua scripts (spec.luaFilters[])

apiVersion: nc.core.mesh/v3
kind: RouteConfiguration   — rules reference a script by name (Rule.luaFilter)
```

Or CRs `core.netcracker.com/v1` `Mesh` with
`subKind: HttpFilters` and `subKind: RouteConfiguration` (identical `spec` shape).

Output: one `TrafficExtension` per `luaFilter` name per resolved gateway target, for all gateway
types (ingress, egress, waypoint).

> **Core mesh constraint:** Lua scripts attach to individual `RouteV3.Rule` entries only.
> Preserve per-rule granularity during migration.

Reference:
[TrafficExtension](https://istio.io/latest/docs/reference/config/proxy_extensions/traffic_extension/),
[Extend waypoints with Lua scripts](https://istio.io/latest/docs/ambient/usage/extend-waypoint-lua/)

---

### HttpFilters.spec

  JSON key     Go type        Transformation
  ─────────────────────────────────────────────────────────────────────────────
  gateways     []string       → [gateway name resolution](#gateway-name-resolution); OMIT from output
  luaFilters   []LuaFilter    → script library; resolve `Rule.luaFilter` references
  wasmFilters  []WasmFilter   OMIT
  extAuthzFilter *ExtAuthz   OMIT

If `luaFilters` is empty or absent → do **not** generate extension resources.

---

### LuaFilter

  JSON key    Go type  Transformation
  ────────────────────────────────────────────────────────────────
  name        string   → lookup key for `Rule.luaFilter`; used in output resource naming
  luaScript   string   → `TrafficExtension.spec.lua.inlineCode`

---

### Rule.luaFilter resolution

For every `RouteV3.Rule` with non-empty `luaFilter`:

1. Find matching `LuaFilter` in `HttpFilters.spec.luaFilters` by `name`.
2. Not found → `# ⚠ MANUAL REVIEW`.
3. Copy `luaScript` verbatim (preserve Helm expressions).
4. Resolve gateway names from the intersection of `HttpFilters.spec.gateways` and
   `RouteConfiguration.spec.gateways` (or whichever is present).
5. Emit a `TrafficExtension` per resolved gateway (see below).

Rules without `luaFilter` → no extension resource.

---

### Gateway name resolution

Resolve platform gateway names with the same table the `core-mesh-crs-to-gatewayapi` skill uses
for `parentRefs` (reproduced here in full — no need to consult that skill):

| Gateway value | Resolved `targetRefs.name` |
|---|---|
| `public-gateway-service` | `public-gateway` |
| `private-gateway-service` | `private-gateway` |
| `egress-gateway` | `egress-gateway` |
| `internal-gateway-service` | mesh Gateway from discovery (Cloud Core: `waypoint`) |
| Custom `Gateway` CR (`spec.gatewayType` ingress/egress/mesh) | `metadata.name` |

Unresolvable gateway → `# ⚠ MANUAL REVIEW`.

---

## TrafficExtension

Generate **one** `TrafficExtension` per `luaFilter` name per resolved gateway target.

Name the resource `<destination-service>-<lua-filter-name>`, where `<destination-service>` is the
`cluster` of the route whose rule references the filter (e.g. `catalog-service-uuid-from-path`).

```yaml
apiVersion: extensions.istio.io/v1alpha1
kind: TrafficExtension
metadata:
  name: <destination-service>-<lua-filter-name>
  namespace: <RouteConfiguration.metadata.namespace>
spec:
  targetRefs:
  - kind: Gateway
    group: gateway.networking.k8s.io
    name: <resolved gateway name>
  phase: STATS
  lua:
    inlineCode: |
      function envoy_on_request(request_handle)
        local path = request_handle:headers():get(":path")
        if not string.find(path, "<rule-prefix>", 1, true) then
          return
        end
        <luaScript body — unwrap outer function if already wrapped>
      end
```

`TrafficExtension` has no per-route attachment — embed a path guard per rule prefix.
Guard on the rule's `match.prefix`, **not** `prefixRewrite`: the Lua filter runs before the
route-level rewrite, so it sees the original `:path`.
Multiple rules share one `luaFilter` name but have different prefixes → `# ⚠ MANUAL REVIEW`.

---

## Helm dual-mesh guards

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}
apiVersion: core.netcracker.com/v1
kind: Mesh
subKind: HttpFilters
metadata:
  name: catalog-lua-filters
  labels:
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  gateways:
    - public-gateway-service
  luaFilters:
    - name: uuid-from-path
      luaScript: |
        ...
---
apiVersion: core.netcracker.com/v1
kind: Mesh
subKind: RouteConfiguration
metadata:
  name: catalog-routes
  labels:
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  gateways: ["public-gateway-service"]
  virtualServices:
    - name: public-gateway-service
      hosts: ["*"]
      routeConfiguration:
        routes:
          - destination:
              cluster: catalog-service
              endpoint: http://catalog-service:8080
            rules:
              - match:
                  prefix: /api/v1/service/catalogManagement
                allowed: true
                luaFilter: uuid-from-path
{{- end }}
```

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
# TrafficExtension resources
{{- end }}
```

File naming:

```text
templates/http-filters.yaml          → templates/http-filters-istio.yaml
templates/routes-configuration.yaml  → append to templates/routes-configuration-istio.yaml
```

---

## Fields that MUST be flagged with `# ⚠ MANUAL REVIEW`

| Source | Trigger |
|---|---|
| `Rule.luaFilter` | name not found in `HttpFilters.spec.luaFilters` |
| `TrafficExtension` | same `luaFilter` on rules with different prefixes |
| `gateways` | gateway name cannot be resolved |
| `luaScript` | empty |

---

## Output examples

See [tests/lua-expected-output.yaml](tests/lua-expected-output.yaml) — `TrafficExtension` for an
ingress gateway and for the waypoint.
