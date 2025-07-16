package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	asrt "github.com/stretchr/testify/assert"
)

func Test_IT_ResponseTest_ResponseContainsUUIDHeader(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const cluster = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster, "v1", true)
	defer traceSrvContainer1.Purge()

	prefix := "/api/v1/test-header/a1b2c3d4-e5f6-7890-1234-567890abcdef"

	filterConfig := `apiVersion: nc.core.mesh/v3
kind: HttpFilters
spec:
  gateways:
    - internal-gateway-service
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
	
	internalGateway.ApplyConfigAndWaitLuaFiltersAppear(assert, 60*time.Second, filterConfig)
	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: TestEndpointV1},
			Rules: []dto.Rule{
				{Match: dto.RouteMatch{Prefix: prefix}, LuaFilter: "test-lua-filter"},
			},
		},
	)

 	resp, statusCode := GetFromTraceService(assert, internalGateway.Url+prefix)
	assert.Equal(http.StatusOK, statusCode)
	if resp == nil {
		log.InfoC(ctx, "Didn't receive TraceResponse; status code: %d", statusCode)
	} else {
		log.InfoC(ctx, "Trace service response: %v", resp)
		assert.Equal(prefix, resp.Path)
		// verify request header x-uuid 
		assert.Equal("a1b2c3d4-e5f6-7890-1234-567890abcdef", resp.Headers.Get("X-Uuid"))
	}

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{"internal-gateway-service"},
		VirtualService: "internal-gateway-service",
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: prefix}},
			Version: "v1",
		},
	}) 
}