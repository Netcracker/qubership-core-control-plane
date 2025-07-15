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
      luaScript: "
		function envoy_on_request(request_handle)\n
			local path = request_handle:headers():get(\":path\")\n
			local match = string.match(path, \"([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\")\n
			if match then\n
				request_handle:headers():add(\"x-uuid\", match)\n
			end\n
			request_handle:logInfo(\"URL is: \"..request_handle:headers():get(\":x-original-url\"))
            request_handle:logInfo(\"Path is: \"..request_handle:headers():get(\":path\"))
			request_handle:logInfo(\"x-uuid is: \"..request_handle:headers():get(\":x-uuid\"))
		end"
`
	
	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, filterConfig)
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

	envoyConfigDump := internalGateway.GetEnvoyConfigJson(assert)
    log.Info("Internal-gateway config dump: \n %v", envoyConfigDump)

	headers := make(http.Header)
	headers.Set("Test-header", "Test header must be traced in response")

/* 	resp, statusCode := GetFromTraceServiceWithHeaders(assert, internalGateway.Url+prefix, headers)
	assert.Equal(http.StatusOK, statusCode)
	if resp == nil {
		log.InfoC(ctx, "Didn't receive TraceResponse; status code: %d", statusCode)
	} else {
		log.InfoC(ctx, "Trace service response: %v", resp)
		assert.Equal(prefix, resp.Path)
		// verify request header x-uuid 
		assert.Equal("a1b2c3d4-e5f6-7890-1234-567890abcdef", resp.Headers.Get("x-uuid"))
	} */

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