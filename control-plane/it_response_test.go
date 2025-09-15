package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	asrt "github.com/stretchr/testify/assert"
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

	assert.True(checkIfTestRouteWithPrefixIsPresentOnInternal(assert, cluster, prefix))

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
	assert.False(checkIfTestRouteWithPrefixIsPresentOnInternal(assert, cluster, prefix))
}

func checkIfTestRouteWithPrefixIsPresent(assert *asrt.Assertions, cluster string, prefix string, envoyConfigDump *EnvoyRouteConfigDump) bool {
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

func checkIfTestRouteWithPrefixIsPresentOnInternal(assert *asrt.Assertions, cluster string, prefix string) bool {
	envoyConfigDump := internalGateway.GetEnvoyRouteConfig(assert)
	return checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix, envoyConfigDump)
}

func checkIfTestRouteWithPrefixIsPresentOnEgress(assert *asrt.Assertions, cluster string, prefix string) bool {
	envoyConfigDump := egressGateway.GetEnvoyRouteConfig(assert)
	fmt.Printf("EnvoyConfigDump: \n%v", envoyConfigDump)
	return checkIfTestRouteWithPrefixIsPresent(assert, cluster, prefix, envoyConfigDump)
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

func Test_IT_ResponseTest_ResponseViaErgressDoesNotContainDefaultHeaders(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const cluster = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster, "v1", true)
	defer traceSrvContainer1.Purge()

	prefix := "/api/v1/test-egress-headers"

	egressGateway.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{"egress-gateway"},
		VirtualServices: []dto.VirtualService{
			{
				Name:  "egress-gateway",
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

	assert.True(checkIfTestRouteWithPrefixIsPresentOnEgress(assert, cluster, prefix))
	headers := make(http.Header)
	headers.Set("X-Forwarded-For", "X-Forwarded-For header must be removed in response")
	headers.Set("Some-Other-Header", "Header-value")
	testResponseHeaders(assert, egressGateway.Url+prefix, headers, http.StatusOK,
		map[string]string{"X-Forwarded-For": ""})

	// cleanup routes
	egressGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{"egress-gateway"},
		VirtualService: "egress-gateway",
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: prefix}},
			Version: "v1",
		},
	})
	assert.False(checkIfTestRouteWithPrefixIsPresentOnEgress(assert, cluster, prefix))
	deleteVirtualService(assert, "egress-gateway", "egress-gateway")
}