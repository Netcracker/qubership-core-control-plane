package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	asrt "github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ctx, globalCancel = context.WithCancel(
		context.WithValue(
			context.Background(), "requestId", "",
		),
	)
)

//	genericDao    *dao.InMemDao
//	eventBus      *bus.EventBusAggregator
//	logger        logging.Logger
//	shutdownHooks []func()
//)

func (gateway GatewayContainer) DeleteClusterAndWait(assert *asrt.Assertions, timeout time.Duration, clusterId int32, clusterName string) {
	gateway.performAndWaitForCondition(assert, timeout,
		func() { deleteCluster(assert, clusterId) },
		func(dump *EnvoyConfigDump) bool {
			if dump == nil {
				log.Warnf("Envoy config dump is empty in GatewayContainer#DeleteClusterAndWait")
				return false
			}
			for _, config := range dump.Configs {
				for _, cluster := range config.DynamicActiveClusters {
					if cluster.Cluster.Name == clusterName {
						return false
					}
				}
			}
			return true
		})
}

func deleteCluster(assert *asrt.Assertions, clusterId int32) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:8080/api/v1/routes/clusters/%d", clusterId), nil)
	assert.Nil(err)

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusNoContent, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

func deleteVirtualService(assert *asrt.Assertions, nodeGroup, virtualServiceName string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:8080/api/v3/routes/%s/%s", nodeGroup, virtualServiceName), nil)
	assert.Nil(err)

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

func (gateway GatewayContainer) RegisterRoutesV1AndWait(assert *asrt.Assertions, timeout time.Duration, request RouteEntityRequestV1) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		registerRoutesV1(assert, request)
	})
}

func registerRoutesV1(assert *asrt.Assertions, request RouteEntityRequestV1) {
	reqJson, err := json.Marshal(request)
	assert.Nil(err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/routes/internal-gateway-service", bytes.NewReader(reqJson))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/json"}

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusCreated, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

type RouteEntityRequestV1 struct {
	MicroserviceUrl string            `json:"microserviceUrl"`
	Routes          *[]dto.RouteEntry `json:"routes"`
	Allowed         *bool             `json:"allowed"`
}

func (gateway GatewayContainer) RegisterRoutesV2AndWait(assert *asrt.Assertions, timeout time.Duration, requests ...dto.RouteRegistrationRequest) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		registerRoutesV2(assert, requests...)
	})
}

func registerRoutesV2(assert *asrt.Assertions, requests ...dto.RouteRegistrationRequest) {
	reqJson, err := json.Marshal(requests)
	assert.Nil(err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v2/control-plane/routes/internal-gateway-service", bytes.NewReader(reqJson))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/json"}

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusCreated, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

func (gateway GatewayContainer) RegisterRoutingConfigAndWait(assert *asrt.Assertions, timeout time.Duration, routeConfig *dto.RoutingConfigRequestV3) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		registerRoutingConfig(assert, routeConfig)
	})
}

type TlsConfigReq struct {
	ApiVersion string
	Kind       string
	Spec       *dto.TlsConfig
}

func registerTlsConfig(assert *asrt.Assertions, tlsConfig *dto.TlsConfig) (int, string) {
	reqJson, err := json.Marshal(TlsConfigReq{
		ApiVersion: "nc.core.mesh/v3",
		Kind:       "TlsDef",
		Spec:       tlsConfig,
	})
	assert.Nil(err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v3/config", bytes.NewReader(reqJson))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/json"}

	log.InfoC(ctx, "Registering tls config: %s", reqJson)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	return resp.StatusCode, string(body)
}

func getNodeGroups(assert *asrt.Assertions) (int, string) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/api/v1/routes/node-groups", nil)
	assert.Nil(err)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	return resp.StatusCode, string(body)
}

func registerRoutingConfig(assert *asrt.Assertions, routeConfig *dto.RoutingConfigRequestV3) {
	log.InfoC(ctx, "Registering routes %+v", routeConfig)
	resp, err := sendCloudAdminRequest(assert, http.MethodPost, "http://localhost:8080/api/v3/routes", routeConfig)
	assert.Nil(err)
	assert.Equal(http.StatusCreated, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

func sendCloudAdminRequest(assert *asrt.Assertions, method, url string, body interface{}) (*http.Response, error) {
	var reqJson []byte
	if body == nil {
		reqJson = []byte("")
	} else {
		var err error
		reqJson, err = json.Marshal(body)
		assert.Nil(err)
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(reqJson))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/json"}

	log.InfoC(ctx, "Sending cloud-admin request to %s: %v", url, string(reqJson))
	return http.DefaultClient.Do(req)
}

type RateLimitReq struct {
	ApiVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Spec       *dto.RateLimit `json:"spec"`
}

func registerRateLimit(assert *asrt.Assertions, rateLimit *dto.RateLimit) {
	reqJson, err := json.Marshal(RateLimitReq{
		ApiVersion: "nc.core.mesh/v3",
		Kind:       "RateLimit",
		Spec:       rateLimit,
	})
	assert.Nil(err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v3/config", bytes.NewReader(reqJson))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/json"}

	log.InfoC(ctx, "Registering rate limit: %s", reqJson)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

func (gateway GatewayContainer) RegisterRoutesAndWait(assert *asrt.Assertions, timeout time.Duration, version string, routes ...dto.RouteV3) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		gateway.registerRoutes(assert, version, routes...)
	})
}

func (gateway GatewayContainer) registerRoutes(assert *asrt.Assertions, version string, routes ...dto.RouteV3) {
	routeConfig := dto.RoutingConfigRequestV3{
		Namespace: "",
		Gateways:  []string{gateway.Name},
		VirtualServices: []dto.VirtualService{{
			Name:  gateway.Name,
			Hosts: []string{"*"},
			RouteConfiguration: dto.RouteConfig{
				Version: version,
				Routes:  routes,
			},
		}},
	}
	registerRoutingConfig(assert, &routeConfig)
}

func (gateway GatewayContainer) ApplyConfigAndWait(assert *asrt.Assertions, timeout time.Duration, config string) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		applyConfig(assert, config)
	})
}

func (gateway GatewayContainer) ApplyConfigAndWaitWasmFiltersAppear(assert *asrt.Assertions, timeout time.Duration, config string) {
	gateway.performAndWaitForWasmFiltersAppear(assert, timeout, func() {
		applyConfig(assert, config)
	})
}

func (gateway GatewayContainer) ApplyConfigAndWaitWasmFiltersDisappear(assert *asrt.Assertions, timeout time.Duration, config string) {
	gateway.performAndWaitForWasmFiltersDisappear(assert, timeout, func() {
		applyConfig(assert, config)
	})
}

func (gateway GatewayContainer) ApplyConfigAndWaitLuaFiltersAppear(assert *asrt.Assertions, timeout time.Duration, config string) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		applyConfig(assert, config)
	})
}

func (gateway GatewayContainer) ApplyConfigAndWaitLuaFiltersDisappear(assert *asrt.Assertions, timeout time.Duration, config string) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		applyConfig(assert, config)
	})
}

func applyConfig(assert *asrt.Assertions, config string) {

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v3/config", bytes.NewReader([]byte(config)))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/yaml"}

	log.InfoC(ctx, "Applying config %v", config)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

func (gateway GatewayContainer) CleanupGatewayRoutes(assert *asrt.Assertions, version string, prefixes ...string) {
	routesToDelete := make([]dto.RouteDeleteItem, 0, len(prefixes))
	for _, prefix := range prefixes {
		routesToDelete = append(routesToDelete, dto.RouteDeleteItem{Prefix: prefix})
	}

	gateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{gateway.Name},
		VirtualService: gateway.Name,
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes:  routesToDelete,
			Version: version,
		},
	})
}

func (gateway GatewayContainer) DeleteRoutesAndWait(assert *asrt.Assertions, timeout time.Duration, requests ...dto.RouteDeleteRequestV3) {
	gateway.performAndWaitForRouteConfigUpdate(assert, timeout, func() {
		deleteRoutes(assert, requests...)
	})
}

func deleteRoutes(assert *asrt.Assertions, requests ...dto.RouteDeleteRequestV3) {
	reqJson, err := json.Marshal(requests)
	assert.Nil(err)

	req, err := http.NewRequest(http.MethodDelete, "http://localhost:8080/api/v3/routes", bytes.NewReader(reqJson))
	assert.Nil(err)
	req.Header["Content-Type"] = []string{"application/json"}

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(err)
}

func (gateway GatewayContainer) performAndWaitForCondition(assert *asrt.Assertions, timeout time.Duration, operation func(), condition func(dump *EnvoyConfigDump) bool) {
	operation()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		envoyConfig := gateway.getEnvoyConfig(assert)
		if condition(envoyConfig) {
			return
		}
	}
	assert.Fail("Condition was never true before timeout exceeded")
}

func (gateway GatewayContainer) performAndWaitForRouteConfigUpdate(assert *asrt.Assertions, timeout time.Duration, operation func()) {
	versionBeforeOperation := strconv.FormatInt(time.Now().UnixNano(), 10)
	log.InfoC(ctx, "Envoy config versionBeforeOperation %d", versionBeforeOperation)
	routeConfig := gateway.GetEnvoyRouteConfig(assert)
	if routeConfig != nil && routeConfig.VersionInfo != "" {
		versionBeforeOperation = routeConfig.VersionInfo
		log.InfoC(ctx, "Envoy config versionBeforeOperation changed to %d", versionBeforeOperation)
	}

	operation()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		routeConfig := gateway.GetEnvoyRouteConfig(assert)
		log.InfoC(ctx, "Envoy config routeConfig.VersionInfo check,  %d", routeConfig.VersionInfo)
		if routeConfig != nil && routeConfig.VersionInfo != "" && versionBeforeOperation != routeConfig.VersionInfo {
			log.DebugC(ctx, "Envoy config after route operation %v", *routeConfig)
			return
		}
	}
	assert.Fail("RouteConfig was not updated in envoy before timeout exceeded")
}

func (gateway GatewayContainer) waitAppearInConfig(assert *asrt.Assertions, timeout time.Duration, stringToWait string) (config string) {
	config = gateway.waitFor(assert, timeout, func(config string) bool {
		return strings.Contains(config, stringToWait)
	})
	return
}

func (gateway GatewayContainer) waitDisappearInConfig(assert *asrt.Assertions, timeout time.Duration, stringToWait string) (config string) {
	config = gateway.waitFor(assert, timeout, func(config string) bool {
		return !strings.Contains(config, stringToWait)
	})
	return
}

func (gateway GatewayContainer) waitFor(assert *asrt.Assertions, timeout time.Duration, condition func(string) bool) (config string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		envoyConfig := gateway.GetEnvoyConfigJson(assert)
		if condition(envoyConfig) {
			config = envoyConfig
			return
		}
	}
	assert.Fail("update failed in envoy before timeout exceeded")
	return
}

func (gateway GatewayContainer) performAndWaitForWasmFiltersAppear(assert *asrt.Assertions, timeout time.Duration, operation func()) {
	operation()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if ExtractWasmFilterConfigFromJson(gateway.GetEnvoyConfigJson(assert)) != "" {
			return
		}
	}
	assert.Fail("RouteConfig was not updated in envoy before timeout exceeded")
}

func (gateway GatewayContainer) performAndWaitForWasmFiltersDisappear(assert *asrt.Assertions, timeout time.Duration, operation func()) {
	operation()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if ExtractWasmFilterConfigFromJson(gateway.GetEnvoyConfigJson(assert)) == "" {
			return
		}
	}
	assert.Fail("RouteConfig was not updated in envoy before timeout exceeded")
}

func (gateway GatewayContainer) performAndWaitForLuaFiltersAppear(assert *asrt.Assertions, timeout time.Duration, operation func()) {
	operation()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if ExtractLuaFilterConfigFromJson(gateway.GetEnvoyConfigJson(assert)) != "" {
			return
		}
	}
	assert.Fail("Lua Filter Config was not updated in envoy before timeout exceeded")
}

func (gateway GatewayContainer) performAndWaitForLuaFiltersDisappear(assert *asrt.Assertions, timeout time.Duration, operation func()) {
	operation()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if ExtractLuaFilterConfigFromJson(gateway.GetEnvoyConfigJson(assert)) == "" {
			return
		}
	}
	assert.Fail("Lua Filter Config was not updated in envoy before timeout exceeded")
}

func ExtractHttpFiltersFromJson(json string) string {
	return gjson.Get(json, listenerBasePath+".dynamic_listeners."+
		"0.active_state.listener.filter_chains.0.filters.0.typed_config.http_filters").Raw
}

func ExtractWasmFilterConfigFromJson(json string) string {
	return gjson.Get(ExtractHttpFiltersFromJson(json), "#[name=\"envoy.filters.http.wasm\"].typed_config.config").Raw
}

func ExtractLuaFilterConfigFromJson(json string) string {
	return gjson.Get(ExtractHttpFiltersFromJson(json), "#[name=\"envoy.filters.http.lua\"].typed_config.inline_code").Raw
}

func constructPrefixesFromRoutes(routes []dto.RouteEntry) []string {
	prefixes := make([]string, len(routes))
	for idx, _ := range routes {
		prefixes[idx] = routes[idx].From
	}
	return prefixes
}

type TraceResponse struct {
	ServiceName string `json:"serviceName"`
	FamilyName  string `json:"familyName"`
	Version     string `json:"version"`
	PodID       string `json:"podId"`

	RequestHost string      `json:"requestHost"`
	ServerHost  string      `json:"serverHost"`
	RemoteAddr  string      `json:"remoteAddr"`
	Path        string      `json:"path"`
	Method      string      `json:"method"`
	Headers     http.Header `json:"headers"`
}

func (gateway GatewayContainer) VerifyGatewayRequest(assert *asrt.Assertions, expectedCode int, expectedPath, requestPath string) {
	log.InfoC(ctx, "Trace service request to %v", requestPath)
	commonRouteResponse, statusCode := GetFromTraceService(assert, gateway.Url+requestPath)
	assert.Equal(expectedCode, statusCode)
	if commonRouteResponse == nil {
		log.InfoC(ctx, "Didn't receive TraceResponse; status code: %d", statusCode)
	} else {
		log.InfoC(ctx, "Trace service response: %v", commonRouteResponse)
		assert.Equal(expectedPath, commonRouteResponse.Path)
	}
}

func (gateway GatewayContainer) SendGatewayRequest(assert *asrt.Assertions, method string, path string, body io.Reader, headers ...map[string]string) ([]byte, int) {
	//log.InfoC(ctx, "Trace service %s request to %v", method, path)
	req, err := http.NewRequest(method, gateway.Url+path, body)
	assert.Nil(err)
	if len(headers) > 0 {
		for name, val := range headers[0] {
			req.Header.Add(name, val)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.ErrorC(ctx, "Could not read trace response body bytes: %v", err)
	}
	return bodyBytes, resp.StatusCode
}

func GetFromTraceService(assert *asrt.Assertions, url string) (*TraceResponse, int) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	assert.Nil(err)
	return SendToTraceSrvWithRetry503(assert, req)
}

func GetFromTraceServiceWithHeaders(assert *asrt.Assertions, url string, headers map[string][]string) (*TraceResponse, int) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	assert.Nil(err)
	for key, val := range headers {
		req.Header[key] = val
	}
	return SendToTraceSrvWithRetry503(assert, req)
}

func GetFromTraceServiceWithVersion(assert *asrt.Assertions, url string, version string) (*TraceResponse, int) {
	headers := map[string][]string{"X-Version": {version}}
	return GetFromTraceServiceWithHeaders(assert, url, headers)
}

func SendToTraceSrvWithRetry503(assert *asrt.Assertions, req *http.Request) (*TraceResponse, int) {
	resp := sendWithRetry503(assert, req)
	if resp.StatusCode == http.StatusOK {
		log.Infof("response headers: %+v", resp.Header)
		return ReadTraceServiceResponse(assert, resp), resp.StatusCode
	} else {
		return nil, resp.StatusCode
	}
}

func sendWithRetry503(assert *asrt.Assertions, req *http.Request) *http.Response {
	for attempt := 0; attempt < 5; attempt++ {
		resp, err := http.DefaultClient.Do(req)
		assert.Nil(err)
		if resp.StatusCode == http.StatusOK {
			log.Infof("response headers: %+v", resp.Header)
			return resp
		} else if resp.StatusCode == http.StatusServiceUnavailable {
			log.InfoC(ctx, "Retrying 503 to %s", req.URL.String())
			time.Sleep(500 * time.Millisecond)
			continue
		} else {
			return resp
		}
	}
	assert.Fail("All retries failed with 503 for URL %s", req.URL.String())
	return nil
}

func ReadTraceServiceResponse(assert *asrt.Assertions, resp *http.Response) *TraceResponse {
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	log.Infof("response body: %s", string(bodyBytes))
	var respBody TraceResponse
	assert.Nil(json.Unmarshal(bodyBytes, &respBody))
	return &respBody
}

func (gateway GatewayContainer) GetEnvoyRouteConfig(assert *asrt.Assertions) *EnvoyRouteConfigDump {
	envoyConfig := gateway.getEnvoyConfig(assert)
	var routeConfig EnvoyConfig
	for _, config := range envoyConfig.Configs {
		if config.Type == "type.googleapis.com/envoy.admin.v3.RoutesConfigDump" {
			routeConfig = config
			break
		}
	}
	if len(routeConfig.DynamicRouteConfigs) != 0 {
		return &routeConfig.DynamicRouteConfigs[0]
	}
	return nil
}

func (gateway GatewayContainer) getEnvoyListenerConfigVersion(assert *asrt.Assertions) int64 {
	envoyConfigDump := gateway.GetEnvoyConfigJson(assert)
	version := gjson.Get(envoyConfigDump, "configs.#[@type=\"type.googleapis.com/envoy.admin.v3.ListenersConfigDump\"].version_info")

	return version.Int()
}

func (gateway GatewayContainer) getEnvoyDynamicClusterConfigVersion(assert *asrt.Assertions) int64 {
	envoyConfigDump := gateway.GetEnvoyConfigJson(assert)
	version := gjson.Get(envoyConfigDump, "configs.#[@type=\"type.googleapis.com/envoy.admin.v3.ClustersConfigDump\"].version_info")

	return version.Int()
}

func (gateway GatewayContainer) getEnvoyConfig(assert *asrt.Assertions) *EnvoyConfigDump {
	bodyBytes := gateway.GetEnvoyConfigJson(assert)

	var configDump EnvoyConfigDump
	assert.Nil(json.Unmarshal([]byte(bodyBytes), &configDump))
	return &configDump
}

func (gateway GatewayContainer) GetEnvoyConfigJson(assert *asrt.Assertions) string {
	resp, err := http.DefaultClient.Get(gateway.AdminUrl + "/config_dump")
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)

	return string(bodyBytes)
}

type EnvoyConfigDump struct {
	Configs []EnvoyConfig `json:"configs"`
}

type EnvoyConfig struct {
	Type string `json:"@type"`
	//VersionInfo           string                 `json:"version_info"`
	//LastUpdated           string                 `json:"last_updated"`
	DynamicActiveClusters []DynamicActiveCluster `json:"dynamic_active_clusters"`
	DynamicRouteConfigs   []EnvoyRouteConfigDump `json:"dynamic_route_configs"`
}

type DynamicActiveCluster struct {
	VersionInfo string        `json:"version_info"`
	LastUpdated string        `json:"last_updated"`
	Cluster     *EnvoyCluster `json:"cluster"`
}

type EnvoyCluster struct {
	Name                      string                     `json:"name"`
	Type                      string                     `json:"type"`
	LbPolicy                  string                     `json:"lb_policy"`
	LbSubsetConfig            *LbSubsetConfig            `json:"lb_subset_config"`
	LoadAssignment            *ClusterLoadAssignment     `json:"load_assignment"`
	UpstreamConnectionOptions *UpstreamConnectionOptions `json:"upstream_connection_options"`
}

type UpstreamConnectionOptions struct {
	TcpKeepalive *TcpKeepalive `json:"tcp_keepalive"`
}

type TcpKeepalive struct {
	KeepaliveProbes   *int `json:"keepalive_probes"`
	KeepaliveTime     *int `json:"keepalive_time"`
	KeepaliveInterval *int `json:"keepalive_interval"`
}

type LbSubsetConfig struct {
	FallbackPolicy  string                `json:"fallback_policy"`
	DefaultSubset   map[string]string     `json:"default_subset"`
	SubsetSelectors []map[string][]string `json:"subset_selectors"`
}

type ClusterLoadAssignment struct {
	ClusterName string             `json:"cluster_name"`
	Endpoints   []ClusterEndpoints `json:"endpoints"`
}

type ClusterEndpoints struct {
	LbEndpoints []LbEndpoint `json:"lb_endpoints"`
}

type LbEndpoint struct {
	Endpoint map[string]map[string]EndpointSocketAddr `json:"endpoint"`
	Metadata map[string]map[string]map[string]string  `json:"metadata"`
}

type EndpointSocketAddr struct {
	Address    string `json:"address"`
	PortValue  int    `json:"port_value"`
	IPv4Compat bool   `json:"ipv4_compat"`
}

type EnvoyRouteConfigDump struct {
	VersionInfo string           `json:"version_info"`
	RouteConfig EnvoyRouteConfig `json:"route_config"`
	LastUpdated string           `json:"last_updated"`
}

type EnvoyRouteConfig struct {
	Type         string             `json:"@type"`
	Name         string             `json:"name"`
	VirtualHosts []EnvoyVirtualHost `json:"virtual_hosts"`
}

type EnvoyVirtualHost struct {
	Name    string       `json:"name"`
	Domains []string     `json:"domains"`
	Routes  []EnvoyRoute `json:"routes"`
}

type EnvoyRoute struct {
	Match          EnvoyRouteMatch  `json:"match"`
	Route          EnvoyRouteAction `json:"route"`
	DirectResponse DirectResponse   `json:"direct_response"`
}

type EnvoyRouteMatch struct {
	Prefix  string             `json:"prefix"`
	Regex   string             `json:"regex"`
	Headers []EnvoyHeaderMatch `json:"headers"`
}

type EnvoyHeaderMatch struct {
	Name        string      `json:"name"`
	ExactMatch  string      `json:"exact_match"`
	StringMatch StringMatch `json:"string_match"`
}

type StringMatch struct {
	Exact  string `json:"exact"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
	//SafeRegex  string `json:"safe_regex"`
	Contains   string `json:"contains"`
	IgnoreCase string `json:"ignore_case"`
}

type EnvoyRouteAction struct {
	Cluster       string       `json:"cluster"`
	PrefixRewrite string       `json:"prefix_rewrite"`
	HostRewrite   string       `json:"host_rewrite"`
	RegexRewrite  RegexRewrite `json:"regex_rewrite"`
	Timeout       string       `json:"timeout"`
}

type RegexRewrite struct {
	Pattern      RegexRewritePattern `json:"pattern"`
	Substitution string              `json:"substitution"`
}

type RegexRewritePattern struct {
	GoogleRe2 map[string]interface{} `json:"google_re2"`
	Regex     string                 `json:"regex"`
}

type DirectResponse struct {
	Status int `json:"status"`
}

func (c *EnvoyConfigDump) FindClusterByName(clusterName string) *EnvoyCluster {
	for _, config := range c.Configs {
		for _, cluster := range config.DynamicActiveClusters {
			if cluster.Cluster.Name == clusterName {
				return cluster.Cluster
			}
		}
	}
	return nil
}
