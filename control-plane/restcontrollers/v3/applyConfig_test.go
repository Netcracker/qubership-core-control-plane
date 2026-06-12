package v3

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	v2 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/loadbalance"
	testutils "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/util"
	"github.com/netcracker/qubership-core-lib-go-error-handling/v3/tmf"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	asrt "github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type TestEnvironment struct {
	testutils.TestEnvironment
	ApplyConfigControllerV3 *ApplyConfigController
	LoadBalanceControllerV2 *v2.LoadBalanceController
}

func InitCPTestEnvironment() TestEnvironment {
	testEnv := testutils.InitCPTestEnvironment()
	loadBalanceService := loadbalance.NewLoadBalanceService(testEnv.Dao, testEnv.EntityService, testEnv.EventBus)
	lbRequestValidator := dto.NewLBRequestValidator(testEnv.Dao)
	v2LoadBalanceController := v2.NewLoadBalanceController(loadBalanceService, lbRequestValidator)
	return TestEnvironment{
		TestEnvironment:         testEnv,
		ApplyConfigControllerV3: NewApplyConfigurationController(),
		LoadBalanceControllerV2: v2LoadBalanceController,
	}
}

func TestApplyConfigController_HandlePostConfig(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()
	configresources.RegisterResource(testEnv.RouteServiceV2.GetRegisterRoutesResource())
	configresources.RegisterResources(testEnv.LoadBalanceControllerV2.GetLoadBalanceResources())

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/config", func(ctx *fiber.Ctx) error {
		return testEnv.ApplyConfigControllerV3.HandlePostConfig(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`---
nodeGroup: private-gateway-service
spec:
  - namespace: "default"
    cluster: "trace-service"
    endpoint: http://trace-service-v1:8080
    routes:
      - prefix: "/trace-service/health"
        prefixRewrite: "/health"
    version: "v1"
    allowed: true
  - namespace: "default"
    cluster: "trace-service"
    endpoint: http://trace-service-v1:8080
    routes:
      - prefix: "/trace-service/trace"
        prefixRewrite: "/trace"
    version: "v1"
    allowed: true
  - namespace: "default"
    cluster: "trace-service"
    endpoint: http://trace-service-v1:8080
    routes:
      - prefix: "/trace-service/proxy"
        prefixRewrite: "/proxy"
    version: "v1"
    allowed: true
---
kind: LoadBalance # LoadBalance section is only needed if you require load balance (sticky session) support
spec:
  cluster: "trace-service"
  version: "v1"
  endpoint: http://trace-service-v1:8080
  policies: # list of routes hashing policies for Ring Hash balancing algorithm
    - header: # usually you should configure only one policy (e.g. header)
        headerName: "BID"
    - cookie: # cookie hashing policy is here only as an example of having more than one policy
        name: "JSESSIONID"
---
nodeGroup: private-gateway-service
spec:
  - namespace: "default"
    cluster: "echo-service"
    endpoint: http://echo-service-v1:8080
    routes:
      - prefix: "/echo-service/health"
        prefixRewrite: "/health"
    version: "v1"
    allowed: true
---
APIVersion: nc.core.mesh/v3
kind: LoadBalance # LoadBalance section is only needed if you require load balance (sticky session) support
spec:
  cluster: "echo-service"
  version: "v1"
  endpoint: http://echo-service-v1:8080
  policies: # list of routes hashing policies for Ring Hash balancing algorithm
    - header: # usually you should configure only one policy (e.g. header)
        headerName: "BID"
    - cookie: # cookie hashing policy is here only as an example of having more than one policy
        name: "JSESSIONID"`))
	request.Host = "localhost"

	assert.Nil(err)
	resp, err := app.Test(request)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	bytesBody, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	var result []ApplyResult
	err = json.Unmarshal(bytesBody, &result)
	if err != nil {
		assert.FailNow(err.Error())
	}
	if len(result) < 2 {
		assert.FailNow("Expected two elements in []ApplyResult")
	}
	assert.Equal(200, result[0].Response.Code)

	routes, err := testEnv.Dao.FindAllRoutes()
	if err != nil {
		assert.FailNowf("Finding all routes failed %v", "", err)
	}
	assert.True(containsRouteWithPrefix(routes, "/trace-service/health"))
	assert.True(containsRouteWithPrefix(routes, "/trace-service/trace"))
	assert.True(containsRouteWithPrefix(routes, "/trace-service/proxy"))

	assert.True(containsRouteWithPrefix(routes, "/echo-service/health"))

	assert.Equal(200, result[1].Response.Code)
}

func TestApplyConfigController_HandlePostConfigWithEmptyVersion(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()
	configresources.RegisterResource(testEnv.RouteServiceV2.GetRegisterRoutesResource())
	configresources.RegisterResources(testEnv.LoadBalanceControllerV2.GetLoadBalanceResources())

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/config", func(ctx *fiber.Ctx) error {
		return testEnv.ApplyConfigControllerV3.HandlePostConfig(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`---
nodeGroup: private-gateway-service
spec:
 - namespace: "default"
   cluster: "trace-service"
   endpoint: http://trace-service-v1:8080
   routes:
     - prefix: "/trace-service/test"
       prefixRewrite: "/test"
   version: ""
   allowed: true
---
APIVersion: nc.core.mesh/v3
kind: LoadBalance
spec:
 cluster: "trace-service"
 version: ""
 endpoint: http://trace-service-v1:8080
 policies:
   - header:
       headerName: "BID"
   - cookie:
       name: "JSESSIONID"`))
	request.Host = "localhost"

	resp, err := app.Test(request)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	bytesBody, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	var result []ApplyResult
	err = json.Unmarshal(bytesBody, &result)
	if err != nil {
		assert.FailNow(err.Error())
	}
	if len(result) < 2 {
		assert.FailNow("Expected two elements in []ApplyResult")
	}
	assert.Equal(http.StatusOK, result[0].Response.Code)

	routes, err := testEnv.Dao.FindAllRoutes()
	if err != nil {
		assert.FailNowf("Finding all routes failed %v", "", err)
	}
	assert.True(containsRouteWithPrefixAndDeploymentVersion(routes, "/trace-service/test", "v1"))

	assert.Equal(http.StatusOK, result[1].Response.Code)
}

func TestApplyConfigController_HandlePostConfigWithoutVersion(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()
	configresources.RegisterResource(testEnv.RouteServiceV2.GetRegisterRoutesResource())
	configresources.RegisterResources(testEnv.LoadBalanceControllerV2.GetLoadBalanceResources())

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/config", func(ctx *fiber.Ctx) error {
		return testEnv.ApplyConfigControllerV3.HandlePostConfig(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`---
nodeGroup: private-gateway-service
spec:
 - namespace: "default"
   cluster: "trace-service"
   endpoint: http://trace-service-v1:8080
   routes:
     - prefix: "/trace-service/test"
       prefixRewrite: "/test"
   allowed: true
---
APIVersion: nc.core.mesh/v3
kind: LoadBalance
spec:
 cluster: "trace-service"
 endpoint: http://trace-service-v1:8080
 policies:
   - header:
       headerName: "BID"
   - cookie:
       name: "JSESSIONID"`))
	request.Host = "localhost"

	resp, err := app.Test(request)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	bytesBody, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	var result []ApplyResult
	err = json.Unmarshal(bytesBody, &result)
	if err != nil {
		assert.FailNow(err.Error())
	}
	if len(result) < 2 {
		assert.FailNow("Expected two elements in []ApplyResult")
	}
	assert.Equal(http.StatusOK, result[0].Response.Code)

	routes, err := testEnv.Dao.FindAllRoutes()
	if err != nil {
		assert.FailNowf("Finding all routes failed %v", "", err)
	}
	assert.True(containsRouteWithPrefixAndDeploymentVersion(routes, "/trace-service/test", "v1"))

	assert.Equal(http.StatusOK, result[1].Response.Code)
}

func TestApplyConfigController_HandlePostConfigCookieWithPath(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()
	configresources.RegisterResource(testEnv.RouteServiceV2.GetRegisterRoutesResource())
	configresources.RegisterResources(testEnv.LoadBalanceControllerV2.GetLoadBalanceResources())

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/config", func(ctx *fiber.Ctx) error {
		return testEnv.ApplyConfigControllerV3.HandlePostConfig(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`---
nodeGroup: private-gateway-service
spec:
 - namespace: "default"
   cluster: "trace-service"
   endpoint: http://trace-service-v1:8080
   routes:
     - prefix: "/trace-service/health"
       prefixRewrite: "/health"
   version: "v1"
   allowed: true
 - namespace: "default"
   cluster: "trace-service"
   endpoint: http://trace-service-v1:8080
   routes:
     - prefix: "/trace-service/trace"
       prefixRewrite: "/trace"
   version: "v1"
   allowed: true
 - namespace: "default"
   cluster: "trace-service"
   endpoint: http://trace-service-v1:8080
   routes:
     - prefix: "/trace-service/proxy"
       prefixRewrite: "/proxy"
   version: "v1"
   allowed: true
---
APIVersion: nc.core.mesh/v3
kind: LoadBalance # LoadBalance section is only needed if you require load balance (sticky session) support
spec:
 cluster: "trace-service"
 version: "v1"
 endpoint: http://trace-service-v1:8080
 policies: # list of routes hashing policies for Ring Hash balancing algorithm
   - header: # usually you should configure only one policy (e.g. header)
       headerName: "BID"
   - cookie: # cookie hashing policy is here only as an example of having more than one policy
       name: "JSESSIONID"
       path: "/"`))
	request.Host = "localhost"

	resp, err := app.Test(request)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	bytesBody, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	var result []ApplyResult
	err = json.Unmarshal(bytesBody, &result)
	if err != nil {
		assert.FailNow(err.Error())
	}
	if len(result) < 2 {
		assert.FailNow("Expected two elements in []ApplyResult")
	}
	assert.Equal(200, result[0].Response.Code)

	routes, err := testEnv.Dao.FindAllRoutes()
	if err != nil {
		assert.FailNowf("Finding all routes failed %v", "", err)
	}
	assert.True(containsRouteWithPrefix(routes, "/trace-service/health"))
	assert.True(containsRouteWithPrefix(routes, "/trace-service/trace"))
	assert.True(containsRouteWithPrefix(routes, "/trace-service/proxy"))

	assert.Equal(200, result[1].Response.Code)
}

func TestApplyConfigController_HandlePostConfigWithError_WrongYamlIndent(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()
	configresources.RegisterResource(testEnv.RouteServiceV2.GetRegisterRoutesResource())

	fiberConfig := fiber.Config{
		ErrorHandler: errorcodes.DefaultErrorHandlerWrapper(errorcodes.UnknownErrorCode),
	}
	app, err := fiberserver.New(fiberConfig).Process()
	assert.Nil(err)
	app.Post("/config", func(ctx *fiber.Ctx) error {
		return testEnv.ApplyConfigControllerV3.HandlePostConfig(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`---
nodeGroup: private-gateway-service
spec:
  - namespace: "default"
    cluster: "trace-service"
    endpoint: http://trace-service-v1:8080
    routes:
      -prefix: "/trace-service/health"
      -prefixRewrite: "/health"
    version: "v1"
    allowed: true
  - namespace: "default"
    cluster: "trace-service"
    endpoint: http://trace-service-v1:8080
    routes:
      - prefix: "/trace-service/trace"
        prefixRewrite: "/trace"
    version: "v1"
    allowed: true
  - namespace: "default"
    cluster: "trace-service"
    endpoint: http://trace-service-v1:8080
    routes:
      - prefix: "/trace-service/proxy"
        prefixRewrite: "/proxy"
    version: "v1"
    allowed: true`))
	request.Host = "localhost"

	resp, err := app.Test(request)
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)

	bytesBody, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	var result tmf.Response
	err = json.Unmarshal(bytesBody, &result)
	if err != nil {
		assert.FailNow(err.Error())
	}
	assert.NotNil(result)
	if len(*result.Errors) != 1 {
		assert.FailNow("Expected one elements in []ApplyResult")
	}
	resultErrors := *result.Errors
	assert.Equal(errorcodes.ValidationRequestError.Code, resultErrors[0].Code)
}

func TestApplyConfigController_HandlePostConfigWithComment(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()
	configresources.RegisterResource(testEnv.RouteServiceV2.GetRegisterRoutesResource())

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/config", func(ctx *fiber.Ctx) error {
		return testEnv.ApplyConfigControllerV3.HandlePostConfig(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`# Some comment
#Some second comment
---
nodeGroup: private-gateway-service
spec:
  - namespace: "default"
    cluster: "trace-service"
    endpoint: http://trace-service-v1:8080
    routes:
      - prefix: "/trace-service/health"
        prefixRewrite: "/health"
    version: "v1"
    allowed: true`))
	request.Host = "localhost"

	resp, err := app.Test(request)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	bytesBody, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	var result []ApplyResult
	err = json.Unmarshal(bytesBody, &result)
	if err != nil {
		assert.FailNow(err.Error())
	}
	if len(result) < 1 {
		assert.FailNow("Expected two elements in []ApplyResult")
	}
	assert.Equal(200, result[0].Response.Code)
}

func TestApplyConfigController_HandlePostConfigWithHeaderMatcher(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()
	configresources.RegisterResource(testEnv.RouteServiceV2.GetRegisterRoutesResource())

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/config", func(ctx *fiber.Ctx) error {
		return testEnv.ApplyConfigControllerV3.HandlePostConfig(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`nodeGroup: anonymous-graphql-service
spec:
  - namespace: "cloudsspbe-kube-portal-dev-ci"
    cluster: "anonymous-graphql-service"
    endpoint: http://anonymous-graphql-service-v1:8080
    routes:
      - prefix: "/api/graphql-server"
      #graphql playground/voyager prefix
      - prefix: "/vendor"
    version: "v1"
    allowed: true
---

nodeGroup: public-gateway-service
spec:
  - namespace: "cloudsspbe-kube-portal-dev-ci"
    cluster: "anonymous-graphql-service"
    endpoint: http://anonymous-graphql-service-v1:8080
    routes:
      - prefix: "/api/graphql-server"
        headerMatchers:
          - name: "Authorization"
            presentMatch: true
      #graphql playground/voyager prefix
      - prefix: "/vendor"
        headerMatchers:
          - name: "Authorization"
            presentMatch: true
    version: "v1"
    allowed: true

`))
	request.Host = "localhost"

	resp, err := app.Test(request)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	bytesBody, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	var result []ApplyResult
	err = json.Unmarshal(bytesBody, &result)
	if err != nil {
		assert.FailNow(err.Error())
	}
	if len(result) != 2 {
		assert.FailNow("Expected two elements in []ApplyResult")
	}
	assert.Equal(http.StatusOK, result[0].Response.Code)
	assert.Equal(http.StatusOK, result[1].Response.Code)
}

func containsRouteWithPrefix(routes []*domain.Route, prefix string) bool {
	for _, route := range routes {
		if route.Prefix == prefix {
			return true
		}
	}
	return false
}

func containsRouteWithPrefixAndDeploymentVersion(routes []*domain.Route, prefix, deploymentVersion string) bool {
	for _, route := range routes {
		if route.Prefix == prefix && route.DeploymentVersion == deploymentVersion {
			return true
		}
	}
	return false
}

func TestOrderConfigs(t *testing.T) {
	assert := asrt.New(t)
	configs := []configresources.ConfigResource{
		{Kind: "RouteConfiguration"},
		{Kind: "StatefulSession"},
		{Kind: "LoadBalance"},
		{Kind: "TlsDef"},
		{Kind: "LoadBalance"},
		{Kind: "RouteConfiguration"},
	}
	result := orderConfigs(configs)
	assert.Equal(3, len(result))

	assert.Equal(1, len(result[0]))
	for _, config := range result[0] {
		assert.Equal("TlsDef", config.Kind)
	}

	assert.Equal(2, len(result[1]))
	for _, config := range result[1] {
		assert.Equal("RouteConfiguration", config.Kind)
	}

	assert.Equal(3, len(result[2]))
	for _, config := range result[2] {
		if "LoadBalance" != config.Kind && "StatefulSession" != config.Kind {
			t.Fail()
		}
	}
}
