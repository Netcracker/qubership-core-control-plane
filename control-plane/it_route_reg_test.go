package main

import (
	"bytes"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/lib"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	asrt "github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func Test_IT_RouteRegistration_InvalidHttpVersion(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	configFormat := `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: echo-route-demo-mytest22
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   routeConfiguration:
     routes:
     - destination:
         cluster: test-service
         endpoint: test-service-v1:8080
         httpVersion: %v
       rules:
       - match:
           prefix: /myawesometest
         prefixRewrite: /echoservice`

	configWithHttpVersion := fmt.Sprintf(configFormat, "v1")
	verifyApplyConfigError(assert, http.StatusInternalServerError, configWithHttpVersion)

	configWithHttpVersion = fmt.Sprintf(configFormat, "HTTP1")
	verifyApplyConfigError(assert, http.StatusInternalServerError, configWithHttpVersion)

	configWithHttpVersion = fmt.Sprintf(configFormat, "HTTP/1")
	verifyApplyConfigError(assert, http.StatusInternalServerError, configWithHttpVersion)

	configWithHttpVersion = fmt.Sprintf(configFormat, "HTTP/2.0")
	verifyApplyConfigError(assert, http.StatusInternalServerError, configWithHttpVersion)

	configWithHttpVersion = fmt.Sprintf(configFormat, "0")
	verifyApplyConfigError(assert, http.StatusInternalServerError, configWithHttpVersion)

	configWithHttpVersion = fmt.Sprintf(configFormat, "3")
	verifyApplyConfigError(assert, http.StatusInternalServerError, configWithHttpVersion)

	configWithHttpVersion = fmt.Sprintf(configFormat, "-2")
	verifyApplyConfigError(assert, http.StatusBadRequest, configWithHttpVersion)
}

func verifyApplyConfigError(assert *asrt.Assertions, expectedStatus int, config string) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v3/config", bytes.NewReader([]byte(config)))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/yaml"}

	log.InfoC(ctx, "Applying config %v", config)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.GreaterOrEqual(resp.StatusCode, 400)
}

func Test_IT_RouteRegistration_HeaderMatchers_Yaml(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	config := `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: echo-route-demo-mytest22
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   hosts: ["*"]
   routeConfiguration:
     routes:
     - destination:
         cluster: test-service
         endpoint: test-service-v1:8080
       rules:
       - match:
           prefix: /myawesometest
           headerMatchers:
           - name: X-External-Id
             exactMatch: demo2
         prefixRewrite: /echoservice`

	const cluster1 = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster1, "v1", true)
	defer traceSrvContainer1.Purge()

	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, config)

	envoyConfigDump := internalGateway.GetEnvoyRouteConfig(assert)
	assert.True(checkIfRouteWithHeadersIsPresent(assert, envoyConfigDump, cluster1, map[string]string{"X-External-Id": "demo2"}))

	internalGateway.verifyRequestWithHeaders(assert, cluster1, "/myawesometest", "/echoservice", map[string]string{"X-External-Id": "demo2"})
	internalGateway.verifyRequestStatus(assert, "/myawesometest", nil, http.StatusNotFound)

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{domain.InternalGateway},
		VirtualService: domain.InternalGateway,
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: "/myawesometest"}},
			Version: "v1",
		},
	})
	envoyConfigDump = internalGateway.GetEnvoyRouteConfig(assert)
	assert.False(checkIfRouteWithHeadersIsPresent(assert, envoyConfigDump, cluster1, map[string]string{"X-External-Id": "demo2"}))
}

func Test_IT_RouteRegistration_HostRewrite(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	config := `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: echo-route-demo-mytest22
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   hosts: ["*"]
   routeConfiguration:
     routes:
     - destination:
         cluster: test-service
         endpoint: test-service-v1:8080
       rules:
       - match:
           prefix: /test-custom-host
         prefixRewrite: /echoservice
         hostRewrite: my-custom-host`

	const cluster1 = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster1, "v1", true)
	defer traceSrvContainer1.Purge()

	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, config)

	resp, code := GetFromTraceService(assert, internalGateway.Url+"/test-custom-host")
	assert.Equal(200, code)
	assert.Equal("my-custom-host", resp.RequestHost)

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{domain.InternalGateway},
		VirtualService: domain.InternalGateway,
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: "/test-custom-host"}},
			Version: "v1",
		},
	})
}

func Test_IT_RouteRegistration_HeaderMatchers(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const cluster1 = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster1, "v1", true)
	defer traceSrvContainer1.Purge()
	const cluster2 = "test-service2"
	traceSrvContainer2 := createTraceServiceContainer(cluster2, "v1", true)
	defer traceSrvContainer2.Purge()

	internalGateway.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{domain.InternalGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  domain.InternalGateway,
				Hosts: []string{"*"},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{{
								Match: dto.RouteMatch{
									Prefix:         "/",
									HeaderMatchers: []dto.HeaderMatcher{{Name: "X-Header", ExactMatch: "val1"}},
								},
							}},
						},
						{
							Destination: dto.RouteDestination{Cluster: cluster2, Endpoint: cluster2 + "-v1:8080"},
							Rules: []dto.Rule{{
								Match: dto.RouteMatch{
									Prefix:         "/",
									HeaderMatchers: []dto.HeaderMatcher{{Name: "X-Header", ExactMatch: "val2"}},
								},
							}},
						},
					},
				},
			},
		},
	})

	assert.True(checkIfTestRoutesPresent(assert, cluster1, cluster2))

	internalGateway.verifyRequestWithHeaders(assert, cluster1, "/api/v3/test/req-headers", "/api/v3/test/req-headers", map[string]string{"X-Header": "val1"})
	internalGateway.verifyRequestWithHeaders(assert, cluster2, "/api/v3/test/req-headers", "/api/v3/test/req-headers", map[string]string{"X-Header": "val2"})

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{domain.InternalGateway},
		VirtualService: domain.InternalGateway,
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: "/"}},
			Version: "v1",
		},
	})
	assert.False(checkIfTestRoutesPresent(assert, cluster1, cluster2))
}

func Test_IT_RouteRegistration_BadVirtualServiceName(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const cluster1 = "test-service"
	traceSrvContainer := createTraceServiceContainer(cluster1, "v1", true)
	defer traceSrvContainer.Purge()

	resp, err := sendCloudAdminRequest(assert, http.MethodPost, "http://localhost:8080/api/v3/routes", &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{domain.InternalGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  "invalid-name",
				Hosts: []string{cluster1},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{{
								Match: dto.RouteMatch{
									Prefix: "/",
								},
							}},
						},
					},
				},
			},
		},
	})
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)

	resp, err = sendCloudAdminRequest(assert, http.MethodPost, "http://localhost:8080/api/v3/routes", &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{domain.PublicGateway, domain.PrivateGateway, domain.InternalGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  domain.PublicGateway,
				Hosts: []string{cluster1},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			}, {
				Name:  domain.PrivateGateway,
				Hosts: []string{cluster1},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			}, {
				Name:  domain.InternalGateway,
				Hosts: []string{cluster1},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			},
		},
	})
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)

	resp, err = sendCloudAdminRequest(assert, http.MethodPost, "http://localhost:8080/api/v3/routes", &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{domain.PublicGateway, domain.PrivateGateway, domain.InternalGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  domain.PublicGateway,
				Hosts: []string{cluster1},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			},
		},
	})
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)

	resp, err = sendCloudAdminRequest(assert, http.MethodPost, "http://localhost:8080/api/v3/routes", &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{domain.PublicGateway, domain.PrivateGateway, domain.InternalGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  domain.InternalGateway,
				Hosts: []string{cluster1},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			},
		},
	})
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
}

func Test_IT_RouteRegistration_UniqueHostsValidation(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	testGatewayContainer1 := CreateGatewayContainer("test-gateway1")
	defer testGatewayContainer1.Purge()

	const cluster1 = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster1, "v1", true)
	defer traceSrvContainer1.Purge()
	const cluster2 = "test-service2-customport"
	traceSrvContainer2 := createTraceServiceContainerOnPort(cluster2, "v1", 8081, 8082, true)
	defer traceSrvContainer2.Purge()

	service1 := dto.VirtualService{
		Name:  cluster1,
		Hosts: []string{cluster1, "same-host"},
		RouteConfiguration: dto.RouteConfig{
			Version: "v1",
			Routes: []dto.RouteV3{
				{
					Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
					Rules: []dto.Rule{
						{Match: dto.RouteMatch{Prefix: "/"}},
					},
				},
			},
		},
	}
	service2 := dto.VirtualService{
		Name:  cluster2,
		Hosts: []string{cluster2, "same-host"},
		RouteConfiguration: dto.RouteConfig{
			Version: "v1",
			Routes: []dto.RouteV3{
				{
					Destination: dto.RouteDestination{Cluster: cluster2, Endpoint: cluster2 + "-v1:8081"},
					Rules: []dto.Rule{
						{Match: dto.RouteMatch{Prefix: "/"}},
					},
				},
			},
		},
	}

	resp, err := sendCloudAdminRequest(assert, http.MethodPost, "http://localhost:8080/api/v3/routes", &dto.RoutingConfigRequestV3{
		Namespace:       "",
		Gateways:        []string{testGatewayContainer1.Name},
		VirtualServices: []dto.VirtualService{service1, service2},
	})
	assert.Nil(err)
	assert.LessOrEqual(http.StatusBadRequest, resp.StatusCode)

	// new register services one by one: the 1st one will register, the 2nd one must fail

	testGatewayContainer1.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace:       "",
		Gateways:        []string{testGatewayContainer1.Name},
		VirtualServices: []dto.VirtualService{service1},
	})

	resp, err = sendCloudAdminRequest(assert, http.MethodPost, "http://localhost:8080/api/v3/routes", &dto.RoutingConfigRequestV3{
		Namespace:       "",
		Gateways:        []string{testGatewayContainer1.Name},
		VirtualServices: []dto.VirtualService{service2},
	})
	assert.Nil(err)
	assert.LessOrEqual(http.StatusBadRequest, resp.StatusCode)
}

func Test_IT_RouteRegistration_ExpandingServiceHosts(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const testGateway1 = "test-gw1"
	testGatewayContainer1 := CreateGatewayContainer(testGateway1)
	defer testGatewayContainer1.Purge()

	const cluster1 = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster1, "v1", true)
	defer traceSrvContainer1.Purge()
	const cluster2 = "test-service2-customport"
	traceSrvContainer2 := createTraceServiceContainerOnPort(cluster2, "v1", 8081, 8082, true)
	defer traceSrvContainer2.Purge()

	testGatewayContainer1.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{testGateway1},
		VirtualServices: []dto.VirtualService{
			{
				Name:  cluster1,
				Hosts: []string{cluster1},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster1, Endpoint: cluster1 + "-v1:8080"},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			},
			{
				Name:  cluster2,
				Hosts: []string{cluster2 + ":8081"},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: cluster2, Endpoint: cluster2 + "-v1:8081"},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			}},
	})

	envoyConfigDump := testGatewayContainer1.GetEnvoyRouteConfig(assert)
	srv1ExpectedDomains := []string{
		"test-service.local.svc.cluster.local:8080",
		"test-service.local.svc:8080",
		"test-service.local:8080",
		"test-service:8080"}
	srv2ExpectedDomains := []string{
		"test-service2-customport.local.svc.cluster.local:8081",
		"test-service2-customport.local.svc:8081",
		"test-service2-customport.local:8081",
		"test-service2-customport:8081"}
	srv1IsPresent := false
	srv2IsPresent := false
	for _, vHost := range envoyConfigDump.RouteConfig.VirtualHosts {
		if vHost.Name == cluster1 {
			srv1IsPresent = true
			for _, expectedDomain := range srv1ExpectedDomains {
				assert.Contains(vHost.Domains, expectedDomain)
			}
		} else if vHost.Name == cluster2 {
			srv2IsPresent = true
			for _, expectedDomain := range srv2ExpectedDomains {
				assert.Contains(vHost.Domains, expectedDomain)
			}
		}
	}
	assert.True(srv1IsPresent)
	assert.True(srv2IsPresent)

	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster1, "test-service:8080", "test-service:8080")
	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster1, "test-service.local:8080", "test-service.local:8080")
	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster1, "test-service.local.svc:8080", "test-service.local.svc:8080")
	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster1, "test-service.local.svc.cluster.local:8080", "test-service.local.svc.cluster.local:8080")

	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster2, "test-service2-customport:8081", "test-service2-customport:8081")
	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster2, "test-service2-customport.local:8081", "test-service2-customport.local:8081")
	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster2, "test-service2-customport.local.svc:8081", "test-service2-customport.local.svc:8081")
	verifyRequestToVirtualService(assert, testGatewayContainer1.Url, cluster2, "test-service2-customport.local.svc.cluster.local:8081", "test-service2-customport.local.svc.cluster.local:8081")

	// cleanup routes
	testGatewayContainer1.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{testGateway1},
		VirtualService: cluster1,
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: "/"}},
			Version: "v1",
		},
	})
	testGatewayContainer1.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{testGateway1},
		VirtualService: cluster2,
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: "/"}},
			Version: "v1",
		},
	})
}

func Test_IT_RouteRegistration_MergeVirtualServiceHosts(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer1 := createTraceServiceContainer(TestCluster, "v1", true)
	defer traceSrvContainer1.Purge()

	internalGateway.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{domain.InternalGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  domain.InternalGateway,
				Hosts: []string{TestCluster},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: TestEndpointV1},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			}},
	})

	envoyConfigDump := internalGateway.GetEnvoyRouteConfig(assert)
	expectedDomains := []string{
		"test-service.local.svc.cluster.local:8080",
		"test-service.local.svc:8080",
		"test-service.local:8080",
		"test-service:8080"}
	srvIsPresent := false
	for _, vHost := range envoyConfigDump.RouteConfig.VirtualHosts {
		if vHost.Name == domain.InternalGateway {
			srvIsPresent = true
			for _, expectedDomain := range expectedDomains {
				assert.Contains(vHost.Domains, expectedDomain)
			}
		}
	}
	assert.True(srvIsPresent)

	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service:8080", "test-service-v1:8080")
	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service.local:8080", "test-service-v1:8080")
	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service.local.svc:8080", "test-service-v1:8080")
	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service.local.svc.cluster.local:8080", "test-service-v1:8080")

	internalGateway.RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{domain.InternalGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  domain.InternalGateway,
				Hosts: []string{TestCluster, "some-custom-host:8080"},
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: TestEndpointV1},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: "/"}},
							},
						},
					},
				},
			}},
	})

	envoyConfigDump = internalGateway.GetEnvoyRouteConfig(assert)
	expectedDomains = []string{
		"some-custom-host.local.svc.cluster.local:8080",
		"some-custom-host.local.svc:8080",
		"some-custom-host.local:8080",
		"some-custom-host:8080",
		"test-service.local.svc.cluster.local:8080",
		"test-service.local.svc:8080",
		"test-service.local:8080",
		"test-service:8080"}
	srvIsPresent = false
	for _, vHost := range envoyConfigDump.RouteConfig.VirtualHosts {
		if vHost.Name == domain.InternalGateway {
			srvIsPresent = true
			for _, expectedDomain := range expectedDomains {
				assert.Contains(vHost.Domains, expectedDomain)
			}
		}
	}
	assert.True(srvIsPresent)

	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "some-custom-host:8080", "test-service-v1:8080")
	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service:8080", "test-service-v1:8080")
	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service.local:8080", "test-service-v1:8080")
	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service.local.svc:8080", "test-service-v1:8080")
	verifyRequestToVirtualService(assert, internalGateway.Url, TestCluster, "test-service.local.svc.cluster.local:8080", "test-service-v1:8080")

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{domain.InternalGateway},
		VirtualService: domain.InternalGateway,
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  []dto.RouteDeleteItem{{Prefix: "/"}},
			Version: "v1",
		},
	})
}

func verifyRequestToVirtualService(assert *asrt.Assertions, gatewayUrl, clusterName, hostToSet, expectedHost string) {
	const requestPath = "/api/v2/test/virtual-hosts"
	req, err := http.NewRequest(http.MethodGet, gatewayUrl+requestPath, nil)
	assert.Nil(err)
	req.Host = hostToSet

	response, statusCode := SendToTraceSrvWithRetry503(assert, req)
	assert.Equal(http.StatusOK, statusCode)
	assert.NotNil(response)
	log.InfoC(ctx, "Trace service response: %v", response)
	assert.Equal(requestPath, response.Path)
	assert.Equal(expectedHost, response.RequestHost)
	assert.Equal(clusterName, response.FamilyName)
}

func (gateway GatewayContainer) verifyRequestWithHeaders(assert *asrt.Assertions, clusterName, path, pathRewrite string, headers map[string]string) {
	req, err := http.NewRequest(http.MethodGet, gateway.Url+path, nil)
	assert.Nil(err)
	for headerName, headerValue := range headers {
		req.Header.Set(headerName, headerValue)
	}

	response, statusCode := SendToTraceSrvWithRetry503(assert, req)
	assert.Equal(http.StatusOK, statusCode)
	assert.NotNil(response)
	log.InfoC(ctx, "Trace service response: %v", response)
	assert.Equal(pathRewrite, response.Path)
	assert.Equal(clusterName, response.FamilyName)
}

func (gateway GatewayContainer) verifyRequestStatus(assert *asrt.Assertions, path string, headers map[string]string, expectedStatus int) {
	req, err := http.NewRequest(http.MethodGet, gateway.Url+path, nil)
	assert.Nil(err)
	for headerName, headerValue := range headers {
		req.Header.Set(headerName, headerValue)
	}

	response, statusCode := SendToTraceSrvWithRetry503(assert, req)
	assert.Equal(expectedStatus, statusCode)
	log.InfoC(ctx, "Trace service response: %v", response)
}

func Test_IT_RouteRegistration_AllowedV3(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestCluster, "v1", true)
	defer traceSrvContainer.Purge()

	allowed := true
	forbidden := false

	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: TestEndpointV1},
			Rules: []dto.Rule{
				{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/default-route"}},
				{Allowed: &allowed, Match: dto.RouteMatch{Prefix: "/api/v1/test-service/allowed-route"}},
				{Allowed: &forbidden, Match: dto.RouteMatch{Prefix: "/api/v1/test-service/forbidden-route"}},
			},
		},
	)

	internalGateway.VerifyGatewayRequest(assert, http.StatusOK, "/api/v1/test-service/default-route", "/api/v1/test-service/default-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusOK, "/api/v1/test-service/allowed-route", "/api/v1/test-service/allowed-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusNotFound, "/api/v1/test-service/forbidden-route", "/api/v1/test-service/forbidden-route")

	// cleanup v1 routes
	internalGateway.CleanupGatewayRoutes(assert, "v1",
		"/api/v1/test-service/default-route",
		"/api/v1/test-service/allowed-route",
		"/api/v1/test-service/forbidden-route")
}

func Test_IT_RouteRegistration_AllowedV3Update(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestCluster, "v1", true)
	defer traceSrvContainer.Purge()

	allowed := true
	forbidden := true

	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: TestEndpointV1},
			Rules: []dto.Rule{
				{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/default-route"}},
				{Allowed: &allowed, Match: dto.RouteMatch{Prefix: "/api/v1/test-service/allowed-route"}},
				{Deny: &forbidden, Match: dto.RouteMatch{Prefix: "/api/v1/test-service"}},
			},
		},
	)

	internalGateway.VerifyGatewayRequest(assert, http.StatusNotFound, "/api/v1/test-service/default-route", "/api/v1/test-service/default-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusNotFound, "/api/v1/test-service/allowed-route", "/api/v1/test-service/allowed-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusNotFound, "/api/v1/test-service", "/api/v1/test-service")

	internalGateway.CleanupGatewayRoutes(assert, "v1",
		"/api/v1/test-service/default-route",
		"/api/v1/test-service/allowed-route",
		"/api/v1/test-service/")
}

func Test_IT_RouteRegistration_AllowedV2(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestCluster, "v1", true)
	defer traceSrvContainer.Purge()

	allowed := true
	forbidden := false

	internalGateway.RegisterRoutesV2AndWait(
		assert,
		60*time.Second,
		dto.RouteRegistrationRequest{
			Cluster:  TestCluster,
			Endpoint: TestEndpointV1,
			Routes: []dto.RouteItem{
				{Prefix: "/api/v1/test-service/default-route"},
			},
		},
		dto.RouteRegistrationRequest{
			Cluster:  TestCluster,
			Endpoint: TestEndpointV1,
			Allowed:  &allowed,
			Routes: []dto.RouteItem{
				{Prefix: "/api/v1/test-service/allowed-route"},
			},
		},
		dto.RouteRegistrationRequest{
			Cluster:  TestCluster,
			Endpoint: TestEndpointV1,
			Allowed:  &forbidden,
			Routes: []dto.RouteItem{
				{Prefix: "/api/v1/test-service/forbidden-route"},
			},
		},
	)

	internalGateway.VerifyGatewayRequest(assert, http.StatusOK, "/api/v1/test-service/default-route", "/api/v1/test-service/default-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusOK, "/api/v1/test-service/allowed-route", "/api/v1/test-service/allowed-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusNotFound, "/api/v1/test-service/forbidden-route", "/api/v1/test-service/forbidden-route")

	// cleanup v1 routes
	internalGateway.CleanupGatewayRoutes(assert, "v1",
		"/api/v1/test-service/default-route",
		"/api/v1/test-service/allowed-route",
		"/api/v1/test-service/forbidden-route")
}

func Test_IT_RouteRegistration_AllowedV1(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestCluster, "v1", true)
	defer traceSrvContainer.Purge()

	allowed := true
	forbidden := false

	internalGateway.RegisterRoutesV1AndWait(
		assert,
		60*time.Second,
		RouteEntityRequestV1{
			MicroserviceUrl: TestEndpointV1,
			Routes: &[]dto.RouteEntry{{
				From: "/api/v1/test-service/default-route",
				To:   "/api/v1/test-service/default-route",
				Type: domain.ProfileInternal,
			}},
		},
	)
	internalGateway.RegisterRoutesV1AndWait(
		assert,
		60*time.Second,
		RouteEntityRequestV1{
			MicroserviceUrl: TestEndpointV1,
			Routes: &[]dto.RouteEntry{{
				From: "/api/v1/test-service/allowed-route",
				To:   "/api/v1/test-service/allowed-route",
				Type: domain.ProfileInternal,
			}},
			Allowed: &allowed,
		},
	)
	internalGateway.RegisterRoutesV1AndWait(
		assert,
		60*time.Second,
		RouteEntityRequestV1{
			MicroserviceUrl: TestEndpointV1,
			Routes: &[]dto.RouteEntry{{
				From: "/api/v1/test-service/forbidden-route",
				To:   "/api/v1/test-service/forbidden-route",
				Type: domain.ProfileInternal,
			}},
			Allowed: &forbidden,
		},
	)

	internalGateway.VerifyGatewayRequest(assert, http.StatusOK, "/api/v1/test-service/default-route", "/api/v1/test-service/default-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusOK, "/api/v1/test-service/allowed-route", "/api/v1/test-service/allowed-route")
	internalGateway.VerifyGatewayRequest(assert, http.StatusNotFound, "/api/v1/test-service/forbidden-route", "/api/v1/test-service/forbidden-route")

	// cleanup v1 routes
	internalGateway.CleanupGatewayRoutes(assert, "v1",
		"/api/v1/test-service/default-route",
		"/api/v1/test-service/allowed-route",
		"/api/v1/test-service/forbidden-route")
}

func checkIfRouteWithHeadersIsPresent(assert *asrt.Assertions, envoyConfigDump *EnvoyRouteConfigDump, cluster string, headers map[string]string) bool {
	msAddress := msaddr.NewMicroserviceAddress(cluster+"-v1:8080", "")
	clusterKey := clusterkey.DefaultClusterKeyGenerator.GenerateKey(cluster, msAddress)
	assert.Equal(1, len(envoyConfigDump.RouteConfig.VirtualHosts))
	for _, vHost := range envoyConfigDump.RouteConfig.VirtualHosts {
		for _, route := range vHost.Routes {
			if route.Route.Cluster == clusterKey {
				assert.Equal(len(headers), len(route.Match.Headers))
				for headerName, headerValue := range headers {
					actualHeader := route.Match.Headers[0]
					assert.Equal(headerName, actualHeader.Name)
					assert.Equal(headerValue, actualHeader.StringMatch.Exact)
				}
				return true
			}
		}
	}
	return false
}

func checkIfTestRoutesPresent(assert *asrt.Assertions, cluster1, cluster2 string) bool {
	envoyConfigDump := internalGateway.GetEnvoyRouteConfig(assert)
	header1IsPresent := false
	header2IsPresent := false
	ms1Address := msaddr.NewMicroserviceAddress(cluster1+"-v1:8080", "")
	ms2Address := msaddr.NewMicroserviceAddress(cluster2+"-v1:8080", "")
	cluster1Key := clusterkey.DefaultClusterKeyGenerator.GenerateKey(cluster1, ms1Address)
	cluster2Key := clusterkey.DefaultClusterKeyGenerator.GenerateKey(cluster2, ms2Address)
	assert.Equal(1, len(envoyConfigDump.RouteConfig.VirtualHosts))
	for _, vHost := range envoyConfigDump.RouteConfig.VirtualHosts {
		for _, route := range vHost.Routes {
			if route.Route.Cluster == cluster1Key {
				header1IsPresent = true
				assert.Equal(1, len(route.Match.Headers))
				actualHeader := route.Match.Headers[0]
				assert.Equal("X-Header", actualHeader.Name)
				assert.Equal("val1", actualHeader.StringMatch.Exact)
			} else if route.Route.Cluster == cluster2Key {
				header2IsPresent = true
				assert.Equal(1, len(route.Match.Headers))
				actualHeader := route.Match.Headers[0]
				assert.Equal("X-Header", actualHeader.Name)
				assert.Equal("val2", actualHeader.StringMatch.Exact)
			}
			if header1IsPresent && header2IsPresent {
				break
			}
		}
	}
	return header1IsPresent && header2IsPresent
}

func Test_IT_RouteRegistration_PrometheusMetricsHaveValidNames(t *testing.T) {
	testClusterWithDots := TestCluster + ".with.dots"
	testClusterNameWithDots := strings.ReplaceAll(TestClusterName, TestCluster, testClusterWithDots)
	testClusterNameNoDots := strings.ReplaceAll(testClusterNameWithDots, ".", "_")

	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	log.InfoC(ctx, "envoyConfigJson 1 %s", internalGateway.GetEnvoyConfigJson(assert))
	clusterToDelete, err := lib.GenericDao.FindClusterByName(TestClusterName)
	assert.Nil(err)
	internalGateway.DeleteClusterAndWait(
		assert,
		60*time.Second,
		clusterToDelete.Id, TestClusterName)

	log.InfoC(ctx, "envoyConfigJson 2 %s", internalGateway.GetEnvoyConfigJson(assert))

	traceSrvContainer := createTraceServiceContainer(TestCluster, "v1", true)
	defer traceSrvContainer.Purge()

	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: testClusterWithDots, Endpoint: TestEndpointV1},
			Rules: []dto.Rule{
				{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/default-route"}},
			},
		},
	)

	prometheusMetricsForTestService := requestPrometheusMetricsForTestService(assert)
	log.InfoC(ctx, "prometheusMetricsForTestService = %s", prometheusMetricsForTestService)
	expectedMetric := fmt.Sprintf("envoy_cluster_version{envoy_cluster_name=\"%s\"}", testClusterNameNoDots)
	assert.True(strings.Contains(prometheusMetricsForTestService, expectedMetric),
		"Prometheus Metrics should have metric %s", expectedMetric)

	envoyConfigJson := internalGateway.GetEnvoyConfigJson(assert)
	log.InfoC(ctx, "envoyConfigJson %s", envoyConfigJson)
	assert.False(strings.Contains(envoyConfigJson, testClusterNameWithDots),
		"envoy config should not contain cluster with name %s", testClusterNameWithDots)

	respFromService, statusCode := GetFromTraceService(assert, internalGateway.Url+"/api/v1/test-service/default-route")
	assert.Equal(http.StatusOK, statusCode)
	assert.Equal("test-service-v1", respFromService.ServiceName)

	// cleanup routes
	internalGateway.CleanupGatewayRoutes(assert, "v1", "/api/v1/test-service/default-route")
}

func requestPrometheusMetricsForTestService(assert *asrt.Assertions) string {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/stats/prometheus?filter=test-service", internalGateway.AdminUrl), nil)
	assert.Nil(err)

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)

	return string(bodyBytes)
}
