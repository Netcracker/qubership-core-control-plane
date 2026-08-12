[[_TOC_]]

## Overview

Inline Lua runs at the proxy for header manipulation, logging, path parsing, and similar logic.

> Lua runs inside Envoy on the request path. A faulty script can break routing or take down traffic
> on the affected proxy. Test carefully and **use at your own risk**.

Lua filtering uses the `TrafficExtension` resource and requires **Istio 1.30 or later**.
It applies to all gateway types (ingress, egress, waypoint).

---

## TrafficExtension

Use `TrafficExtension` for Lua on any gateway type. Set `targetRefs` to the target
**Gateway** (`gateway.networking.k8s.io`). Set `phase: STATS` and put the script in
`spec.lua.inlineCode`.

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

Can be used on **egress gateway** for outbound external calls. Body logging is optional and
limited — see notes below.

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

### Path guard template

`TrafficExtension` runs on the whole listener — guard at the top of `envoy_on_request`
(and `envoy_on_response` if used) to limit scope:

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
