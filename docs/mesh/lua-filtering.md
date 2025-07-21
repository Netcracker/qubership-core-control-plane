
[[_TOC_]]

## Overview

You can configure lua filter by setting script code for any route, but please note, that per-virtualService lua filter configuration is forbidden. 

## How to set up lua filter
To set up lua filter you need to register two configurations in control-plane (order does not matter): 

1. LuaFilter configuration with the unique LuaFilter config name;
2. RouteConfiguration with route that refers to the LuaFilter configuration by its name. 

There are several ways to register both LuaFilter and RouteConfiguration: 

1. [Control-Plane REST API](../api/control-plane-api.md)
2. [Declarative API](./development-guide.md#routes-registration-using-configuration-files)

Below are some examples of declarative lua filter configuration.  

## Example

Here is an example of `routes-configuration.yaml` with explanation:

```yaml
---
apiVersion: nc.core.mesh/v3
kind: HttpFilters
spec:
  gateways:
    - some-gateway-service
  luaFilters:
    - name: test-lua-filter
      luaScript: |
        function envoy_on_request(request_handle)
            local path = request_handle:headers():get(":path")
            request_handle:logInfo("Path: " .. path)
            local uuid = string.match(path, ".*/([a-z0-9-]+)$")
            if uuid then
                request_handle:logInfo("UUID: " .. uuid)
                request_handle:headers():add("X-Uuid", uuid)
            else
                request_handle:logInfo("no uuid found")
            end
        end
`	
---
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: ${ENV_SERVICE_NAME}-routes
  namespace: "${ENV_NAMESPACE}"
spec:
  gateways: ["some-gateway-service"]
  virtualServices:
  - name: "${ENV_SERVICE_NAME}"
    hosts: ["${ENV_SERVICE_NAME}"]
    routeConfiguration:
      version: "${ENV_DEPLOYMENT_VERSION}"
      routes:
      - destination:
          cluster: "${ENV_SERVICE_NAME}"
          endpoint: http://${DEPLOYMENT_RESOURCE_NAME}:8080
        rules:
        - match:
            prefix: /api/{version}/service/catalogManagement
          prefixRewrite: /api/{version}/catalogManagement
          luaFilter: test-lua-filter`
        - match:
            prefix: /api/{version}/service/catalogExport
          prefixRewrite: /api/{version}/catalogExport
```

In this example lua filter configured for one route only:
* Route `/api/{version}/service/catalogManagement` has the reference to script that allows to copy uuid from path to header, so the request like `/api/v1/service/catalogManagement/abcd1234-e5f6-7890-1234-567890abcdef` will be sent with heaader `X-Uuid: abcd1234-e5f6-7890-1234-567890abcdef`
* Request to `/api/{version}/service/catalogExport`does not have per-route LuaFilter configuration, so they will be processed without any scripting