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
	
/* 	routeConfig := `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: filter-routes
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   routeConfiguration:
     routes:
     - destination:
         cluster: filter-routes-test
         endpoint: filter-routes-test:8080
       rules:
       - match:
           prefix: /api/v1/test-header/a1b2c3d4-e5f6-7890-1234-567890abcdef
		   luaFilter: my-lua-filter` */


	filterConfig := `apiVersion: nc.core.mesh/v3
kind: HttpFilters
spec:
  gateways:
    - internal-gateway-service
  luaFilters:
    - name: my-lua-filter
      luaScript: "function envoy_on_request(request_handle)\n  local path = request_handle:headers():get(\":path\")\n  local match = string.match(path, \"([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\")\n    if match then\n        request_handle:headers():add(\"x-uuid\", match)\n  end\nend"
`
	
	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, filterConfig)
	//internalGateway.ApplyConfigAndWait(assert, 60*time.Second, routeConfig)
	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: TestEndpointV1},
			Rules: []dto.Rule{
				{Match: dto.RouteMatch{Prefix: prefix}, LuaFilter: "my-lua-filter"},
			},
		},
	)
	

	envoyConfigDump := internalGateway.GetEnvoyConfigJson(assert)
    log.Info("Internal-gateway config dump: \n %v", envoyConfigDump)

 	headers := make(http.Header)
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
	}) 
	//assert.False(checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix))
}