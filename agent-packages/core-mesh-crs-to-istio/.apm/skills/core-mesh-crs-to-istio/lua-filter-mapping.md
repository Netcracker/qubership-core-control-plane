## Lua filter → Istio extension resources

Source (two configuration applied via control-plane apply API, registration order does not matter):

    apiVersion: nc.core.mesh/v3
    kind: HttpFilters          — defines named Lua scripts (spec.luaFilters[])

    apiVersion: nc.core.mesh/v3
    kind: RouteConfiguration   — rules reference a script by name (Rule.luaFilter)

Or CRs `core.netcracker.com/v1` `Mesh` with
`subKind: HttpFilters` and `subKind: RouteConfiguration` (identical `spec` shape).

Output by Istio version (resolve `.Values.ISTIO_VERSION` → cluster `istioctl version` → ask user):

| Istio version | Resource | Scope |
|---|---|---|
| **&lt; 1.30** | `EnvoyFilter` | Ingress/egress gateways and waypoint (different patch rules) |
| **≥ 1.30** | `TrafficExtension` | All gateway types |

```
Istio ≥ 1.30 ?
  yes → TrafficExtension (targetRefs → resolved Gateway or Service)
  no  → gateway is waypoint / mesh ?
          yes → EnvoyFilter (HTTP_FILTER, no context, inlineCode + path guard)
          no  → EnvoyFilter (GATEWAY: base filter + LuaPerRoute)
```

Optional Helm guard:

```yaml
{{- if semverCompare ">=1.30.0" .Values.ISTIO_VERSION }}
# TrafficExtension ...
{{- else }}
# EnvoyFilter (ingress/egress and/or waypoint per gateway type)
{{- end }}
```

> **Core mesh constraint:** Lua scripts attach to individual `RouteV3.Rule` entries only.
> Preserve per-rule granularity during migration.

Reference: [docs/istio/lua-filtering.md](../../../../../docs/istio/lua-filtering.md)

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
  luaScript   string   → `LuaPerRoute` / `TrafficExtension.spec.lua.inlineCode`

---

### Rule.luaFilter resolution

For every `RouteV3.Rule` with non-empty `luaFilter`:

1. Find matching `LuaFilter` in `HttpFilters.spec.luaFilters` by `name`.
2. Not found → `# ⚠ MANUAL REVIEW`.
3. Copy `luaScript` verbatim (preserve Helm expressions).
4. Resolve gateway names from the intersection of `HttpFilters.spec.gateways` and
   `RouteConfiguration.spec.gateways` (or whichever is present).
5. Emit `EnvoyFilter` or `TrafficExtension` per [decision rule](#lua-filter--istio-extension-resources) above.

Rules without `luaFilter` → no extension resource.

---

### Gateway name resolution

Resolve platform gateway names the same way as
[route-configuration-mapping.md](../core-mesh-crs-to-gatewayapi/route-configuration-mapping.md):

| Gateway value | Resolved `targetRefs.name` |
|---|---|
| `public-gateway-service` | `public-gateway` |
| `private-gateway-service` | `private-gateway` |
| `egress-gateway` | `egress-gateway` |
| `internal-gateway-service` | mesh Gateway from discovery (Cloud Core: `waypoint`) |
| Custom `Gateway` CR (`spec.gatewayType` ingress/egress/mesh) | `metadata.name` |

Unresolvable gateway → `# ⚠ MANUAL REVIEW`.

---

## Istio &lt; 1.30 — EnvoyFilter

Ingress/egress gateways and waypoint require **different** `EnvoyFilter` resources — do not
merge them in one CR because `configPatches` differ.

When Lua is needed on **both** gateway(s) and waypoint, emit **separate** resources:

| Target | EnvoyFilter | `targetRefs` |
|---|---|---|
| Ingress / egress gateway(s) | one CR with `context: GATEWAY` patches | one or more ingress/egress gateways (grouping allowed) |
| Waypoint (mesh) | separate CR with `HTTP_FILTER` + full `inline_code` | `waypoint` only |

### Ingress / egress gateway

Generate **one** `EnvoyFilter` for all ingress/egress gateways that share the same Lua
scripts. List each gateway in `spec.targetRefs` (grouping is supported for this patch shape).

#### Patch 1 — base Lua filter (once per gateway)

```yaml
- applyTo: HTTP_FILTER
  match:
    context: GATEWAY
    listener:
      filterChain:
        filter:
          name: envoy.filters.network.http_connection_manager
          subFilter:
            name: envoy.filters.http.router
  patch:
    operation: INSERT_BEFORE
    value:
      name: envoy.filters.http.lua
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua
        stat_prefix: lua
        inline_code: ""
```

#### Patch 2 — per-route LuaPerRoute (one per rule with luaFilter)

```yaml
- applyTo: HTTP_ROUTE
  match:
    context: GATEWAY
    routeConfiguration:
      vhost:
        route:
          name: <istio-route-name>
  patch:
    operation: MERGE
    value:
      typed_per_filter_config:
        envoy.filters.http.lua:
          "@type": type.googleapis.com/envoy.extensions.filters.http.lua.v3.LuaPerRoute
          source_code:
            inline_string: |
              <luaScript from resolved LuaFilter>
```

#### EnvoyFilter shell (ingress / egress)

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: <gateway-name>-lua-filters
  namespace: <RouteConfiguration.metadata.namespace>
spec:
  targetRefs:
  - kind: Gateway
    group: gateway.networking.k8s.io
    name: <resolved gateway name>
  # additional ingress/egress gateways may be listed here
  configPatches:
  # Patch 1 + Patch 2 entries
```

### Waypoint (ambient mesh gateway)

Generate a **separate** `EnvoyFilter` when the resolved gateway is `waypoint` (from
`internal-gateway-service` or mesh `Gateway` CR).

Use a single `HTTP_FILTER` patch with the full script in `inline_code`. **Omit**
`context: GATEWAY`. Do **not** use `LuaPerRoute` — `HTTP_ROUTE` patches with
`context: GATEWAY` do not attach on waypoint in ambient for Istio &lt; 1.30.

Wrap `luaScript` with a path guard per rule prefix (same pattern as `TrafficExtension`):

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: <virtualService-name>-<lua-filter-name>
  namespace: <RouteConfiguration.metadata.namespace>
spec:
  targetRefs:
  - kind: Gateway
    group: gateway.networking.k8s.io
    name: waypoint
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
            subFilter:
              name: envoy.filters.http.router
    patch:
      operation: INSERT_BEFORE
      value:
        name: envoy.filters.http.lua
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua
          stat_prefix: lua
          inline_code: |
            function envoy_on_request(request_handle)
              local path = request_handle:headers():get(":path")
              if not string.find(path, "<rule-prefix>", 1, true) then
                return
              end
              <luaScript body — unwrap outer function if already wrapped>
            end
```

Multiple rules share one `luaFilter` name but have different prefixes → `# ⚠ MANUAL REVIEW`

### Route name resolution (ingress / egress only)

1. Derive from generated HTTPRoute rule index: `<httproute-name>-<rule-index>`.
2. Cannot determine → `# ⚠ MANUAL REVIEW` on `route.name` with expected rule prefix.

---

## Istio ≥ 1.30 — TrafficExtension

Generate **one** `TrafficExtension` per `luaFilter` name per resolved gateway target.

```yaml
apiVersion: extensions.istio.io/v1alpha1
kind: TrafficExtension
metadata:
  name: <virtualService-name>-<lua-filter-name>
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
# EnvoyFilter or TrafficExtension per ISTIO_VERSION
{{- end }}
```

File naming:

```
templates/http-filters.yaml          → templates/http-filters-istio.yaml
templates/routes-configuration.yaml  → append to templates/routes-configuration-istio.yaml
```

---

## Fields that MUST be flagged with `# ⚠ MANUAL REVIEW`

| Source | Trigger |
|---|---|
| `Rule.luaFilter` | name not found in `HttpFilters.spec.luaFilters` |
| `EnvoyFilter` `route.name` | cannot be resolved from HTTPRoute |
| `TrafficExtension` | same `luaFilter` on rules with different prefixes |
| `gateways` | gateway name cannot be resolved |
| Waypoint `EnvoyFilter` | same `luaFilter` on rules with different prefixes |
| `luaScript` | empty |

---

## Output examples

Gateway(s) + waypoint, Istio &lt; 1.30 → separate `EnvoyFilter` per patch shape: one for
ingress/egress (optionally grouped `targetRefs`), one for waypoint (see
[tests/lua-expected-output-lt130.yaml](tests/lua-expected-output-lt130.yaml)).

Any gateway, Istio ≥ 1.30 → `TrafficExtension` (see [tests/lua-expected-output.yaml](tests/lua-expected-output.yaml)).
