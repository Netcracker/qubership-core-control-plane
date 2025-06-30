package main

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	asrt "github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

const httpsHost = TestCluster + ":8443"
const httpHost = TestCluster + ":8080"
const TestClusterUnderFacade = TestCluster + "-v1"
const testClusterHttpsEndpoint = "https://" + TestClusterUnderFacade + ":8443"
const testClusterHttpEndpoint = TestClusterUnderFacade + ":8080"
const egressGatewayName = "egress-gateway"
const internalGatewayName = "internal-gateway-service"

// this test should go first: egress-gateway should absent
func Test_IT_forEgressGateway_givenNoRoutesAndGroupNode_registerTlsConfig_NodeGroupIsCreated(t *testing.T) {
	skipTestIfDockerDisabled(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()

	assert := asrt.New(t)

	status, body := getNodeGroups(assert)
	assert.Equal(200, status)
	assert.NotContains(body, `"name":"egress-gateway"`)

	testServiceUrl := fmt.Sprintf("http://%s:%v", dockerHost, traceSrvContainer.GetPort(8080))
	tlsConfig := &dto.TlsConfig{
		Name:               "test-tls-config-egress-gateway",
		TrustedForGateways: []string{egressGatewayName},
		Tls: &dto.Tls{
			Enabled:   true,
			TrustedCA: getCertificateFromTestService(assert, testServiceUrl),
			SNI:       httpsHost,
		},
	}

	status, _ = registerTlsConfig(assert, tlsConfig)
	assert.Equal(200, status)

	defer deleteTlsConfig(assert, tlsConfig)

	status, body = getNodeGroups(assert)
	assert.Equal(200, status)
	assert.Contains(body, `"name":"egress-gateway"`)
}

func deleteTlsConfig(assert *asrt.Assertions, config *dto.TlsConfig) {
	config.Tls = nil
	registerTlsConfig(assert, config)
}

func Test_IT_TLS_http_to_https(t *testing.T) {
	skipTestIfDockerDisabled(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()

	assert := asrt.New(t)
	registerTestRoutingConfigWithTLS(assert, egressGatewayName, testClusterHttpsEndpoint, nil)
	defer cleanUp(assert, egressGatewayName)

	statusCode, _ := getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url)
	asrt.Equal(t, 503, statusCode)
}

func Test_IT_TLS_add_self_sign_cert(t *testing.T) {
	skipTestIfDockerDisabled(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()

	assert := asrt.New(t)

	testServiceUrl := fmt.Sprintf("http://%s:%v", dockerHost, traceSrvContainer.GetPort(8080))
	registerTestRoutingConfigWithTLS(assert, egressGatewayName, testClusterHttpsEndpoint, &dto.TlsConfig{
		Name: "test-tls-config",
		Tls: &dto.Tls{
			Enabled:   true,
			TrustedCA: getCertificateFromTestService(assert, testServiceUrl),
			SNI:       httpsHost,
		},
	})
	defer cleanUp(assert, egressGatewayName)

	statusCode, _ := getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url)
	asrt.Equal(t, 200, statusCode)
}

func Test_IT_TLS_Ecdh_Curves_P521(t *testing.T) {
	containerName := TestClusterUnderFacade + "P521"
	test_IT_TLS_Ecdh_Curves(t, containerName, "P-521", func(t *testing.T, statusCode int, body string) {
		asrt.Equal(t, 503, statusCode)
		asrt.Contains(t, body, "connection failure")
		asrt.Contains(t, body, "268436496:SSL routines:OPENSSL_internal:SSLV3_ALERT_HANDSHAKE_FAILURE")
		asrt.Contains(t, body, "268435610:SSL routines:OPENSSL_internal:HANDSHAKE_FAILURE_ON_CLIENT_HELLO")
	})
}

func Test_IT_TLS_Ecdh_Curves_P256_P384(t *testing.T) {
	containerName := TestClusterUnderFacade + "P384"
	test_IT_TLS_Ecdh_Curves(t, containerName, "P-256,P-384", func(t *testing.T, statusCode int, body string) {
		asrt.Equal(t, 200, statusCode)
	})
}

func Test_IT_TLS_Ecdh_Curves_P256(t *testing.T) {
	containerName := TestClusterUnderFacade + "P256"
	test_IT_TLS_Ecdh_Curves(t, containerName, "P-256", func(t *testing.T, statusCode int, body string) {
		asrt.Equal(t, 200, statusCode)
	})
}

func test_IT_TLS_Ecdh_Curves(t *testing.T, containerName, ecdhCurves string, checkResult func(*testing.T, int, string)) {
	skipTestIfDockerDisabled(t)

	traceSrvContainer := createTraceServiceContainerInternal(containerName, "v1", 8080, 8443, false, ecdhCurves)
	defer traceSrvContainer.Purge()

	assert := asrt.New(t)

	endpoint := "https://" + containerName + ":8443"
	testServiceUrl := fmt.Sprintf("http://%s:%v", dockerHost, traceSrvContainer.GetPort(8080))
	registerTestRoutingConfigWithTLS(assert, egressGatewayName, endpoint, &dto.TlsConfig{
		Name: "test-tls-config",
		Tls: &dto.Tls{
			Enabled:   true,
			TrustedCA: getCertificateFromTestService(assert, testServiceUrl),
			SNI:       httpsHost,
		},
	})
	defer cleanUp(assert, egressGatewayName)

	statusCode, body := getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url)
	checkResult(t, statusCode, body)
}

func Test_IT_TLS_ignore_self_sign_cert(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()

	registerTestRoutingConfigWithTLS(assert, egressGatewayName, testClusterHttpsEndpoint, &dto.TlsConfig{
		Name: "test-tls-config",
		Tls: &dto.Tls{
			Enabled:  true,
			Insecure: true,
			SNI:      "sni",
		},
	})
	defer cleanUp(assert, egressGatewayName)

	statusCode, _ := getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url)
	asrt.Equal(t, 200, statusCode)
}

func Test_IT_TLS_trusted_for_gateways(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()

	registerTestRoutingConfigWithPrefix(assert, egressGatewayName, testClusterHttpsEndpoint, "/https", "")
	registerTestRoutingConfigWithPrefix(assert, egressGatewayName, testClusterHttpEndpoint, "/http", "")
	defer cleanUp(assert, egressGatewayName, "/https", "/http")

	// HTTPS
	testServiceStatusCode, _ := getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url+"/https")
	assert.Equal(503, testServiceStatusCode)

	// HTTP
	testServiceStatusCode, _ = getFromTraceServiceWithHost(assert, httpHost, egressGateway.Url+"/http")
	assert.Equal(200, testServiceStatusCode)

	// start checking the enabling `accepting untrusted`
	clusterVersionBefore := egressGateway.getEnvoyDynamicClusterConfigVersion(assert)
	status, _ := registerTlsConfig(assert, &dto.TlsConfig{
		Name:               "test-tls-config",
		TrustedForGateways: []string{egressGatewayName},
		Tls: &dto.Tls{
			Enabled:  true,
			Insecure: true,
			SNI:      "sni",
		},
	})
	assert.Equal(200, status)
	waitUntil(func() bool {
		version := egressGateway.getEnvoyDynamicClusterConfigVersion(assert)
		return version != 0 && clusterVersionBefore != version
	})

	// HTTPS
	// Even the cluster version is changed the new version of cluster does not start working immediately.
	// New cluster is being warmed and located in `dynamic_warming_clusters` in clusterConfigDump during the warming.
	// And after few moments it replaces old one in `dynamic_active_clusters`.
	waitUntil(func() bool {
		testServiceStatusCode, _ = getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url+"/https")
		return 200 == testServiceStatusCode
	})
	assert.Equal(200, testServiceStatusCode)

	// HTTP
	testServiceStatusCode, _ = getFromTraceServiceWithHost(assert, httpHost, egressGateway.Url+"/http")
	assert.Equal(200, testServiceStatusCode)

	// start checking the disabling `accepting untrusted`
	status, _ = registerTlsConfig(assert, &dto.TlsConfig{
		Name:               "test-tls-config",
		TrustedForGateways: []string{egressGatewayName},
		Tls: &dto.Tls{
			Enabled:  false,
			Insecure: false,
			SNI:      "sni",
		},
	})
	assert.Equal(200, status)
	waitUntil(func() bool {
		version := egressGateway.getEnvoyDynamicClusterConfigVersion(assert)
		return version != 0 && clusterVersionBefore != version
	})

	// HTTPS
	waitUntil(func() bool {
		testServiceStatusCode, _ = getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url+"/https")
		return 503 == testServiceStatusCode
	})
	assert.Equal(503, testServiceStatusCode)

	// HTTP
	testServiceStatusCode, _ = getFromTraceServiceWithHost(assert, httpHost, egressGateway.Url+"/http")
	assert.Equal(200, testServiceStatusCode)
}

func Test_IT_TLS_trusted_for_egress_only(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()

	registerTestRoutingConfig(assert, internalGatewayName, testClusterHttpsEndpoint, "")
	defer cleanUp(assert, internalGatewayName)

	status, body := registerTlsConfig(assert, &dto.TlsConfig{
		Name:               "test-tls-config",
		TrustedForGateways: []string{internalGatewayName},
		Tls: &dto.Tls{
			Enabled:  true,
			Insecure: true,
		},
	})
	assert.Equal(400, status)
	assert.Contains(body, "global TLS supported only for "+egressGatewayName)
}

func Test_IT_TLS_trusted_for_cluster_prior(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()
	defer cleanUp(assert, egressGatewayName)

	registerTestRoutingConfigWithTLS(assert, egressGatewayName, testClusterHttpsEndpoint, &dto.TlsConfig{
		Name: "test-tls-config",
		Tls: &dto.Tls{
			Enabled:  true,
			Insecure: true,
		},
	})
	traceStatusCode, _ := getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url)
	asrt.Equal(t, 200, traceStatusCode)

	testServiceUrl := fmt.Sprintf("http://%s:%v", dockerHost, traceSrvContainer.GetPort(8080))
	status, _ := registerTlsConfig(assert, &dto.TlsConfig{ //add global TLS config
		Name:               "global-test-tls-config",
		TrustedForGateways: []string{egressGatewayName},
		Tls: &dto.Tls{
			Enabled:   true,
			TrustedCA: getCertificateFromTestService(assert, testServiceUrl),
			Insecure:  false,
		},
	})
	assert.Equal(200, status)
	traceStatusCode, _ = getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url)
	assert.Equal(200, traceStatusCode) //same result. no impact from gateway tls.
}

func Test_IT_TLS_add_self_sign_cert_client_cert(t *testing.T) {
	skipTestIfDockerDisabled(t)

	traceSrvContainer := createTraceServiceContainer(TestClusterUnderFacade, "v1", false)
	defer traceSrvContainer.Purge()

	assert := asrt.New(t)

	testServiceUrl := fmt.Sprintf("http://%s:%v", dockerHost, traceSrvContainer.GetPort(8080))

	registerTestRoutingConfigWithTLS(assert, egressGatewayName, testClusterHttpsEndpoint, &dto.TlsConfig{
		Name: "test-tls-config",
		Tls: &dto.Tls{
			Enabled:    true,
			TrustedCA:  getCertificateFromTestService(assert, testServiceUrl),
			SNI:        httpsHost,
			ClientCert: getClientCertificateFromTestService(assert, testServiceUrl),
			PrivateKey: getPrivateKeyFromTestService(assert, testServiceUrl),
		},
	})
	defer cleanUp(assert, egressGatewayName)

	statusCode, _ := getFromTraceServiceWithHost(assert, httpsHost, egressGateway.Url)
	asrt.Equal(t, 200, statusCode)
}

func cleanUp(assert *asrt.Assertions, gateway string, prefixes ...string) {
	if len(prefixes) == 0 {
		cleanUp(assert, gateway, "/")
	}

	for _, prefix := range prefixes {
		gatewayByName(gateway).DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
			Gateways:       []string{gateway},
			VirtualService: gateway,
			RouteDeleteRequest: dto.RouteDeleteRequest{
				Routes:  []dto.RouteDeleteItem{{Prefix: prefix}},
				Version: "v1",
			},
		})
	}

}

func getFromTraceServiceWithHost(assert *asrt.Assertions, host, url string) (int, string) {
	log.Info("getFromTraceServiceWithHost: %s, %s", host, url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	assert.Nil(err)
	req.Host = host
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		log.ErrorC(ctx, "Failed to execute request: %d: %s", resp.StatusCode, body)
	}

	assert.Nil(err)
	return resp.StatusCode, string(body)
}

func registerTestRoutingConfigWithTLS(assert *asrt.Assertions, gateway, endpoint string, tlsConfig *dto.TlsConfig) {
	tlsConfigName := ""
	if tlsConfig != nil {
		status, _ := registerTlsConfig(assert, tlsConfig)
		assert.Equal(200, status)
		tlsConfigName = tlsConfig.Name
	}
	registerTestRoutingConfig(assert, gateway, endpoint, tlsConfigName)
}

func registerTestRoutingConfig(assert *asrt.Assertions, gateway, endpoint, tlsConfigName string) {
	registerTestRoutingConfigWithPrefix(assert, gateway, endpoint, "/", tlsConfigName)
}

func registerTestRoutingConfigWithPrefix(assert *asrt.Assertions, gateway, endpoint, prefix, tlsConfigName string) {
	gatewayByName(gateway).RegisterRoutingConfigAndWait(assert, 60*time.Second, &dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{gateway},
		VirtualServices: []dto.VirtualService{
			{
				Name: gateway,
				RouteConfiguration: dto.RouteConfig{
					Version: "v1",
					Routes: []dto.RouteV3{
						{
							Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: endpoint, TlsConfigName: tlsConfigName},
							Rules: []dto.Rule{
								{Match: dto.RouteMatch{Prefix: prefix}},
							},
						},
					},
				},
			},
		},
	})
}

func getCertificateFromTestService(assert *asrt.Assertions, serviceUrl string) string {
	resp, err := http.Get(serviceUrl + "/certificate")
	assert.Nil(err)

	defer resp.Body.Close()

	certificateBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	return string(certificateBytes)
}

func getClientCertificateFromTestService(assert *asrt.Assertions, serviceUrl string) string {
	resp, err := http.Get(serviceUrl + "/client_certificate")
	assert.Nil(err)

	defer resp.Body.Close()

	certificateBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	return string(certificateBytes)
}

func getPrivateKeyFromTestService(assert *asrt.Assertions, serviceUrl string) string {
	resp, err := http.Get(serviceUrl + "/private_key")
	assert.Nil(err)

	defer resp.Body.Close()

	keyBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	return string(keyBytes)
}

func gatewayByName(name string) *GatewayContainer {
	switch name {
	case egressGatewayName:
		return egressGateway
	case internalGatewayName:
		return internalGateway
	}
	panic(fmt.Sprintf("no gateway found for name='%s'", name))
}

func waitUntil(condition func() bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		if condition() || ctx.Err() != nil {
			break
		}
	}
}
