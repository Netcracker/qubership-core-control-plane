package main

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	asrt "github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

const (
	cluster = "test-service"
)

func Test_IT_ResponseTest_ResponseDoesNotContainServerHeader(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const cluster = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster, "v1", true)
	defer traceSrvContainer1.Purge()

	prefix := "/api/v1/test-server-header"

	internalGateway.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{"internal-gateway-service"},
		VirtualServices: []dto.VirtualService{
			{
				Name:  "internal-gateway-service",
				Hosts: []string{"*"},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster, Endpoint: cluster + "-v1:8080"},
							Rules: []dto.Rule{
								{
									Match: dto.RouteMatch{
										Prefix: prefix,
									},
								},
							},
						},
					},
				},
			},
		},
	})

	assert.True(checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix))

	headers := make(http.Header)
	headers.Set("server", "Server header must be removed in response")
	testResponseHeaders(assert, internalGateway.Url+prefix, headers, http.StatusOK,
		map[string]string{"server": ""})

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{"internal-gateway-service"},
		VirtualService: "internal-gateway-service",
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: prefix}},
			Version: "v1",
		},
	})
	assert.False(checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix))
}

func Test_IT_ResponseTest_ResponseContainUuidHeader(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	
	traceSrvContainer1 := createTraceServiceContainer(cluster, "v1", true)
	defer traceSrvContainer1.Purge()

	prefix :=  "/api/v1/test-header/a1b2c3d4-e5f6-7890-1234-567890abcdef"

	internalGateway.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{"internal-gateway-service"},
		VirtualServices: []dto.VirtualService{
			{
				Name:  "internal-gateway-service",
				Hosts: []string{"*"},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster, Endpoint: cluster + "-v1:8080"},
							Rules: []dto.Rule{
								{
									Match: dto.RouteMatch{
										Prefix: prefix,
									},
								},
							},
						},
					},
				},
			},
		},
	})

	assert.True(checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix))

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
	assert.False(checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix))
}


func checkIfTestRouteWithPrefixIsPresent(assert *asrt.Assertions, cluster string, prefix string) bool {
	envoyConfigDump := internalGateway.GetEnvoyRouteConfig(assert)
	msAddress := msaddr.NewMicroserviceAddress(cluster+"-v1:8080", "")
	clusterKey := clusterkey.DefaultClusterKeyGenerator.GenerateKey(cluster, msAddress)
	assert.Equal(1, len(envoyConfigDump.RouteConfig.VirtualHosts))
	for _, vHost := range envoyConfigDump.RouteConfig.VirtualHosts {
		for _, route := range vHost.Routes {
			if route.Route.Cluster == clusterKey {
				if prefix == route.Match.Prefix {
					return true
				}
			}
		}
	}
	return false
}

func testResponseHeaders(assert *asrt.Assertions, url string, requestHeaders http.Header, expectedStatus int, expectedResponseHeaders map[string]string) {
	testResponseHeadersForMethod(assert, http.MethodGet, url, requestHeaders, expectedStatus, expectedResponseHeaders)
}

func testResponseHeadersForMethod(assert *asrt.Assertions, method, url string, requestHeaders http.Header, expectedStatus int, expectedResponseHeaders map[string]string) {
	req, err := http.NewRequest(method, url, nil)
	assert.Nil(err)
	req.Header = requestHeaders

	response, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(expectedStatus, response.StatusCode)
	log.Infof("Trace request response headers: %+v", response.Header)
	for expectedHeader, expectedVal := range expectedResponseHeaders {
		assert.Equal(expectedVal, response.Header.Get(expectedHeader))
	}
}
