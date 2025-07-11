package main

import (
	asrt "github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_IT_ResponseTest_ResponseContainsUUIDHeader(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const cluster = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster, "v1", true)
	defer traceSrvContainer1.Purge()

	prefix := "/api/v1/test-header/a1b2c3d4-e5f6-7890-1234-567890abcdef"

	luaScript := "function envoy_on_request(request_handle)\n" +
	"  local path = request_handle:headers():get(\":path\")\n" +
	"  local match = string.match(path, \"([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\")\n" +
	"    if match then\n" +
	"        request_handle:headers():add(\"x-uuid\", match)\n" +
	"  end\n" +
	"end"

	internalGateway.ApplyConfigAndWaitLuaFiltersAppear(assert, 60*time.Second, luaScript)

	envoyConfigDump := internalGateway.getEnvoyConfig(assert)
    log.Info("Internal-gateway config dump: \n %v", envoyConfigDump)
/* 	headers := make(http.Header)
	testResponseHeaders(assert, internalGateway.Url+prefix, headers, http.StatusOK,
		map[string]string{"x-uuid": "a1b2c3d4-e5f6-7890-1234-567890abcdef"})

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{"internal-gateway-service"},
		VirtualService: "internal-gateway-service",
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: prefix}},
			Version: "v1",
		},
	}) */
	assert.False(checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix))
}