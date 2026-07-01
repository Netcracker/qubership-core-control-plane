[[_TOC_]]

## Overview

Inline Lua runs at the proxy for header manipulation, logging, path parsing, and similar logic.

> Lua runs inside Envoy on the request path. A faulty script can break routing or take down traffic on the affected proxy. Test carefully and **use at your own risk**.

Choose the resource by Istio version:

| Istio version | Resource | Scope |
|---|---|---|
| **&lt; 1.30** | `EnvoyFilter` | Ingress/egress gateways and waypoint (different patch rules) |
| **≥ 1.30** | `TrafficExtension` | All gateway types (ingress, egress, waypoint) |

In ambient mode, waypoint Lua on Istio &lt; 1.30 uses `EnvoyFilter` with a **different**
`configPatches` shape than ingress/egress gateways. Do **not** use `context: GATEWAY` or
`LuaPerRoute` on waypoint — those patches apply on ingress/egress only.

---

## Istio &lt; 1.30 — EnvoyFilter

Ingress/egress gateways and waypoint require **different** `EnvoyFilter` resources — do not
merge them in one CR because `configPatches` differ.

When Lua is needed on **both** gateway(s) and waypoint, emit **separate** resources:

| Target | EnvoyFilter | `targetRefs` |
|---|---|---|
| Ingress / egress gateway(s) | one CR with `context: GATEWAY` patches | one or more ingress/egress gateways (grouping allowed) |
| Waypoint (mesh) | separate CR with `HTTP_FILTER` + full `inline_code` | `waypoint` only |

For ingress/egress, multiple gateways (`public-gateway`, `private-gateway`, …) may share
one `EnvoyFilter` — list each in `spec.targetRefs`. Waypoint always gets its own CR.

### Ingress / egress gateway

Attach Lua via `EnvoyFilter` with `context: GATEWAY`.
Istio requires a base Lua filter in the HTTP chain and a per-route override via `LuaPerRoute`.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: public-gateway-lua-filters
  namespace: catalog-namespace
spec:
  targetRefs:
  - kind: Gateway
    group: gateway.networking.k8s.io
    name: public-gateway
  # additional ingress/egress gateways may be listed here
  configPatches:
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
  - applyTo: HTTP_ROUTE
    match:
      context: GATEWAY
      routeConfiguration:
        vhost:
          route:
            name: catalog-routes-catalog-service-0
    patch:
      operation: MERGE
      value:
        typed_per_filter_config:
          envoy.filters.http.lua:
            "@type": type.googleapis.com/envoy.extensions.filters.http.lua.v3.LuaPerRoute
            source_code:
              inline_string: |
                function envoy_on_request(request_handle)
                    local path = request_handle:headers():get(":path")
                    local uuid = string.match(path, ".*/([a-z0-9-]+)$")
                    if uuid then
                        request_handle:headers():add("X-Uuid", uuid)
                    end
                end
```

`route.name` must match the Envoy route name for the `HTTPRoute` rule:

```bash
istioctl proxy-config routes deploy/<gateway-pod> -n <namespace> --name http.8080 -o json
```

### Waypoint (ambient mesh gateway)

Use a **separate** `EnvoyFilter` targeting the mesh `Gateway` (`waypoint`).
Insert one `HTTP_FILTER` with the full script in `inline_code`. **Omit** `context: GATEWAY`.
Add a path guard at the top of the script (waypoint has no per-route `LuaPerRoute` attachment
in ambient for Istio &lt; 1.30).

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: waypoint-lua-filters
  namespace: catalog-namespace
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
              if not string.find(path, "/api/v1/service/catalogManagement", 1, true) then
                return
              end
              local uuid = string.match(path, ".*/([a-z0-9-]+)$")
              if uuid then
                request_handle:headers():add("X-Uuid", uuid)
              end
            end
```

Other `EnvoyFilter` patch types (for example `applyTo: VIRTUAL_HOST` for header removal) also
work on waypoint. `HTTP_ROUTE` + `LuaPerRoute` with `context: GATEWAY` does **not** attach on
waypoint in ambient — verified via `istioctl proxy-config`.

---

## Istio ≥ 1.30 — TrafficExtension

Use `TrafficExtension` for Lua on any gateway type. Set `targetRefs` to the target
**Gateway** (`gateway.networking.k8s.io`) or **Service** (waypoint proxy).
Set `phase: STATS` and put the script in `spec.lua.inlineCode`.

`TrafficExtension` has no per-route attachment — add a path guard at the top of `inlineCode`
when the script must run only for specific paths.

```yaml
apiVersion: extensions.istio.io/v1alpha1
kind: TrafficExtension
metadata:
  name: catalog-service-uuid-from-path
  namespace: catalog-namespace
spec:
  targetRefs:
  - kind: Gateway
    group: gateway.networking.k8s.io
    name: waypoint
  phase: STATS
  lua:
    inlineCode: |
      function envoy_on_request(request_handle)
        local path = request_handle:headers():get(":path")
        if not string.find(path, "/api/v1/service/catalogManagement", 1, true) then
          return
        end
        local uuid = string.match(path, ".*/([a-z0-9-]+)$")
        if uuid then
          request_handle:headers():add("X-Uuid", uuid)
        end
      end
```

Reference: [TrafficExtension](https://istio.io/latest/docs/reference/config/proxy_extensions/traffic_extension/),
[Extend waypoints with Lua scripts](https://istio.io/latest/docs/ambient/usage/extend-waypoint-lua/)

---

## Inline script examples

### Log request and response for a particular external host

Writes method, authority, path, and status to **proxy logs** (`logInfo`). Filters by
`:authority` so only traffic to `example.com` is logged.

Can be used on **egress gateway** for outbound external calls. Body logging is optional and limited — see notes below.

```lua
local TARGET_AUTHORITY = "example.com"
local MAX_BODY = 8192

local function matches_target(handle)
  local authority = handle:headers():get(":authority") or ""
  return authority == TARGET_AUTHORITY
      or string.find(authority, TARGET_AUTHORITY, 1, true) ~= nil
end

local function log_body(phase, handle)
  local body = handle:body()
  if body == nil then
    return
  end
  local len = body:length()
  if len == 0 or len > MAX_BODY then
    handle:logInfo(phase .. " body skipped, length=" .. tostring(len))
    return
  end
  handle:logInfo(phase .. " body=" .. body:getBytes(0, len))
end

function envoy_on_request(request_handle)
  if not matches_target(request_handle) then
    return
  end
  local h = request_handle:headers()
  request_handle:logInfo(
    "EXT REQ "
    .. (h:get(":method") or "") .. " "
    .. (h:get(":authority") or "") .. " "
    .. (h:get(":path") or "")
  )
  log_body("EXT REQ", request_handle)
end

function envoy_on_response(response_handle)
  if not matches_target(response_handle) then
    return
  end
  local h = response_handle:headers()
  response_handle:logInfo("EXT RESP status=" .. (h:get(":status") or ""))
  log_body("EXT RESP", response_handle)
end
```

**Notes**

- Request/response **bodies** may be empty unless the route buffers them; large bodies affect
  memory and latency. For full raw HTTP capture at scale, prefer Envoy access logs or OpenTelemetry.
- Do not log sensitive headers (`Authorization`, cookies) without redaction.

### Add a routing or tracing header from the incoming request

Copies an incoming header to a different name, or sets a default when absent.

```lua
function envoy_on_request(request_handle)
  local h = request_handle:headers()
  local tenant = h:get("x-tenant-id")
  if tenant then
    h:add("x-upstream-tenant", tenant)
  else
    h:add("x-upstream-tenant", "default")
  end
end
```

### Reject requests that do not match an allowlist

Returns HTTP 403 before the request is forwarded. Useful for an extra check at the gateway.

```lua
function envoy_on_request(request_handle)
  local path = request_handle:headers():get(":path") or ""
  if not string.find(path, "/api/v1/public/", 1, true) then
    request_handle:respond(
      {[":status"] = "403", ["content-type"] = "text/plain"},
      "forbidden"
    )
  end
end
```

### Path guard template (`TrafficExtension` / waypoint)

`TrafficExtension` and waypoint `EnvoyFilter` run on the whole listener — guard at the top of
`envoy_on_request` (and `envoy_on_response` if used) to limit scope:

```lua
function envoy_on_request(request_handle)
  local path = request_handle:headers():get(":path") or ""
  if not string.find(path, "/api/v1/partner/acme/", 1, true) then
    return
  end
  -- script logic for this route only
end
```

[Envoy documentation - Examples of Lua scripts](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/lua_filter#script-examples)
[Lua — Stream handle API](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/lua_filter#config-http-filters-lua-stream-handle-api)
