package itmod

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3"
	asrt "github.com/stretchr/testify/assert"
)

const regProhibitRoutes = `[
   {
       "allowed": false,
       "cluster": "order-backend",
       "endpoint": "http://order-backend-v1:8080",
       "routes": [
           {
               "prefix": "/api/v1/dont-go-there"
           }
       ],
       "version": "v1"
   }
]`

const regRoutesNoVersionJson = `[
   {
       "allowed": true,
       "cluster": "order-backend",
       "endpoint": "http://order-backend-v1:8080",
       "routes": [
           {
               "prefix": "/api/v1/order-backend/info",
               "prefixRewrite": "/api/v1/info"
           },
           {
               "prefix": "/api/v1/order-backend/zzzz",
               "prefixRewrite": "/api/v1/zzzz"
           }
       ],
       "version": null
   }
]`

const regRoutesV1Json = `[
   {
       "allowed": true,
       "cluster": "order-backend",
       "endpoint": "http://order-backend-v1:8080",
       "routes": [
           {
               "prefix": "/api/v1/order-backend/info",
               "prefixRewrite": "/api/v1/info"
           },
           {
               "prefix": "/api/v1/order-backend/zzzz",
               "prefixRewrite": "/api/v1/zzzz"
           }
       ],
       "version": "v1"
   }
]`

const regRoutesV2Json = `[
   {
       "allowed": true,
       "cluster": "another-backend",
       "endpoint": "http://another-backend-v2:8080",
       "routes": [
           {
               "prefix": "/api/v2/another-backend/trace",
               "prefixRewrite": "/api/v2/trace"
           },
           {
               "prefix": "/api/v2/another-backend/hello",
               "prefixRewrite": "/api/v2/hello"
           }
       ],
       "version": "v2"
   }
]`

const v1RegRoutesJson = `{
   "allowed": true,
   "microserviceUrl": "http://order-backend:8080",
   "routes": [
       {
           "from": "/api/v1/order-backend/info",
           "to": "/api/v1/info",
           "type": "PUBLIC"
       },
		{
			"from": "/api/v1/order-backend/zzzz",
			"to": "/api/v1/zzzz",
			"type": "PUBLIC"
		}
   ]
}
`

func TestPromoteAndV2RegisterRoutes(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})
	app.Post("/rollback", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostRollbackVersion(ctx)
	})

	respMock, err := app.Test(makeV2Request(regRoutesV2Json), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	respMock, err = app.Test(makeV2Request(regProhibitRoutes), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	respMock, err = app.Test(makeV2Request(regRoutesV1Json), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	version, err := testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.CandidateStage, version.Stage)

	endpoints, err := testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v1", endpoints[0].DeploymentVersion)
	assert.Equal("order-backend-v1", endpoints[0].Address)

	// promote v2
	respMock, err = app.Test(makePromoteRequest("v2"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, respMock.StatusCode)

	version, err = testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.LegacyStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v2", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend-v1", endpoints[0].Address)

	routes, err := testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	routes, err = testEnv.Dao.FindRoutesByClusterName("another-backend||another-backend||8080")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v2", testRoute.InitialDeploymentVersion)
		assert.Equal(uint32(0), testRoute.DirectResponseCode)
	}

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v1")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.True(strings.HasPrefix(testRoute.Prefix, "/api/v2/another-backend"))
		assert.Equal("v1", testRoute.DeploymentVersion)
		assert.Equal(uint32(404), testRoute.DirectResponseCode)
	}

	// register same routes once again
	respMock, err = app.Test(makeV2Request(regRoutesV1Json), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	respMock, err = app.Test(makeV2Request(regRoutesV2Json), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	version, err = testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.LegacyStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v2", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend-v1", endpoints[0].Address)

	routes, err = testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	routes, err = testEnv.Dao.FindRoutesByClusterName("another-backend||another-backend||8080")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v2", testRoute.InitialDeploymentVersion)
		assert.Equal(uint32(0), testRoute.DirectResponseCode)
	}

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v1")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.True(strings.HasPrefix(testRoute.Prefix, "/api/v2/another-backend"))
		assert.Equal("v1", testRoute.DeploymentVersion)
		assert.Equal(uint32(404), testRoute.DirectResponseCode)
	}

	// register order-backend routes without explicit version
	respMock, err = app.Test(makeV2Request(regRoutesNoVersionJson), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v2", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend-v1", endpoints[0].Address)

	routes, err = testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	routes, err = testEnv.Dao.FindRoutesByClusterName("another-backend||another-backend||8080")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v2", testRoute.InitialDeploymentVersion)
		assert.Equal(uint32(0), testRoute.DirectResponseCode)
	}

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v1")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.True(strings.HasPrefix(testRoute.Prefix, "/api/v2/another-backend"))
		assert.Equal("v1", testRoute.DeploymentVersion)
		assert.Equal(uint32(404), testRoute.DirectResponseCode)
	}

	// register order-backend routes without explicit version
	respMock, err = app.Test(makeV2Request(regRoutesNoVersionJson), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v2", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend-v1", endpoints[0].Address)

	routes, err = testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	// rollback to v1
	respMock, err = app.Test(makeRollbackRequest(), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, respMock.StatusCode)

	version, err = testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.CandidateStage, version.Stage)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v1", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend-v1", endpoints[0].Address)

	routes, err = testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v1", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	routes, err = testEnv.Dao.FindRoutesByClusterName("another-backend||another-backend||8080")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v2", testRoute.InitialDeploymentVersion)
		assert.Equal(uint32(0), testRoute.DirectResponseCode)
	}

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v1")
	assert.Nil(err)
	assert.Equal(0, len(routes))

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v2")
	assert.Nil(err)
	assert.Equal(0, len(routes))
}

func TestPromoteAndRegisterRoutesV1(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/v1/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV1.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})
	app.Post("/rollback", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostRollbackVersion(ctx)
	})

	respMock, err := app.Test(makeV2Request(regRoutesV2Json), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	respMock, err = app.Test(makeV2Request(regProhibitRoutes), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	respMock, err = app.Test(makeV1Request(v1RegRoutesJson), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	version, err := testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.CandidateStage, version.Stage)

	endpoints, err := testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v1", endpoints[0].DeploymentVersion)
	assert.Equal("order-backend", endpoints[0].Address)

	// promote v2
	respMock, err = app.Test(makePromoteRequest("v2"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, respMock.StatusCode)

	version, err = testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.LegacyStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v2", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend", endpoints[0].Address)

	routes, err := testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	routes, err = testEnv.Dao.FindRoutesByClusterName("another-backend||another-backend||8080")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v2", testRoute.InitialDeploymentVersion)
		assert.Equal(uint32(0), testRoute.DirectResponseCode)
	}

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v1")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.True(strings.HasPrefix(testRoute.Prefix, "/api/v2/another-backend"))
		assert.Equal("v1", testRoute.DeploymentVersion)
		assert.Equal(uint32(404), testRoute.DirectResponseCode)
	}

	// register same routes once again
	respMock, err = app.Test(makeV1Request(v1RegRoutesJson), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	respMock, err = app.Test(makeV2Request(regRoutesV2Json), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, respMock.StatusCode)

	version, err = testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.LegacyStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v2", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend", endpoints[0].Address)

	routes, err = testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	// rollback to v1
	respMock, err = app.Test(makeRollbackRequest(), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, respMock.StatusCode)

	version, err = testEnv.Dao.FindDeploymentVersion("v1")
	assert.Nil(err)
	assert.Equal(domain.ActiveStage, version.Stage)
	version, err = testEnv.Dao.FindDeploymentVersion("v2")
	assert.Nil(err)
	assert.Equal(domain.CandidateStage, version.Stage)

	endpoints, err = testEnv.Dao.FindEndpointsByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(1, len(endpoints))
	assert.Equal("v1", endpoints[0].DeploymentVersion)
	assert.Equal("v1", endpoints[0].InitialDeploymentVersion)
	assert.Equal("order-backend", endpoints[0].Address)

	routes, err = testEnv.Dao.FindRoutesByClusterName("order-backend||order-backend||8080")
	assert.Nil(err)
	assert.Equal(3, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v1", testRoute.DeploymentVersion)
		assert.Equal("v1", testRoute.InitialDeploymentVersion)
	}

	routes, err = testEnv.Dao.FindRoutesByClusterName("another-backend||another-backend||8080")
	assert.Nil(err)
	assert.Equal(2, len(routes))
	for _, testRoute := range routes {
		assert.Equal("v2", testRoute.DeploymentVersion)
		assert.Equal("v2", testRoute.InitialDeploymentVersion)
		assert.Equal(uint32(0), testRoute.DirectResponseCode)
	}

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v1")
	assert.Nil(err)
	assert.Equal(0, len(routes))

	routes, err = testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v2")
	assert.Nil(err)
	assert.Equal(0, len(routes))
}

func TestPromoteAndRegisterExistV2Route(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/routes/v1/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV1.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})

	rr, err := app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service2",
      "endpoint": "http://some-micro-service2-v1:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-micro-service2/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v2"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makePromoteRequest("v2"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service1",
      "endpoint": "http://some-micro-service1-v2:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-micro-service1/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v2"
  },{
      "allowed": true,
      "cluster": "some-micro-service2",
      "endpoint": "http://some-micro-service2-v3:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-micro-service2/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v3"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makePromoteRequest("v3"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service1",
      "endpoint": "http://some-micro-service1-v2:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-micro-service1/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v2"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)
}

func TestPromoteAndRegisterNewV2LegacyRoutesWithoutFoundEndpoints(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/routes/v1/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV1.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})

	rr, err := app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service",
      "endpoint": "http://some-micro-service-v1:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-micro-service/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v1"
  },
	{
      "allowed": true,
      "cluster": "some-micro-service",
      "endpoint": "http://some-micro-service-v2:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-micro-service/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v2"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makePromoteRequest("v2"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service",
      "endpoint": "http://some-micro-service-v3:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-micro-service/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v3"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makePromoteRequest("v3"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-new-micro-service",
      "endpoint": "http://some-new-micro-service-v3:8080",
      "routes": [
          {
              "prefix": "/api/v1/some-new-micro-service/action",
              "prefixRewrite": "/api/v1/action"
          }
      ],
      "version": "v2"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, rr.StatusCode)
}

func TestPromoteAndRegisterNewV2LegacyRoutes(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/routes/v1/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV1.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})

	rr, err := app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service",
      "endpoint": "http://some-micro-service-v1:8080",
      "routes": [
          {
              "prefix": "/api/v2/some-micro-service/action",
              "prefixRewrite": "/api/v2/action"
          }
      ],
      "version": "v1"
  },
	{
      "allowed": true,
      "cluster": "some-micro-service",
      "endpoint": "http://some-micro-service-v1:8080",
      "routes": [
          {
              "prefix": "/api/v2/some-micro-service/action",
              "prefixRewrite": "/api/v2/action"
          }
      ],
      "version": "v2"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
	  "allowed": true,
	  "cluster": "some-micro-service",
	  "endpoint": "http://some-micro-service-v1:8080",
	  "routes": [
	      {
	          "prefix": "/api/v2/some-micro-service/v2-before-promote",
	          "prefixRewrite": "/api/v2/v2-before-promote"
	      }
	  ],
	  "version": "v2"
	}]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makePromoteRequest("v2"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service",
      "endpoint": "http://some-micro-service-v1:8080",
      "routes": [
          {
              "prefix": "/api/v2/some-micro-service/action",
              "prefixRewrite": "/api/v2/action"
          }
      ],
      "version": "v1"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
      "allowed": true,
      "cluster": "some-micro-service",
      "endpoint": "http://some-micro-service-v1:8080",
      "routes": [
          {
              "prefix": "/api/v2/some-micro-service/new-action",
              "prefixRewrite": "/api/v2/action"
          }
      ],
      "version": "v1"
  }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, rr.StatusCode)

	rr, err = app.Test(makeV1Request(`
	{
      "allowed": true,
      "microserviceUrl": "http://some-micro-service-v1:8080",
      "routes": [
          {
              "from": "/api/v2/some-micro-service/new-action",
              "to": "/api/v2/action"
          }
      ]
  }`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makeV2Request(`
	[{
	  "allowed": true,
	  "cluster": "some-micro-service",
	  "endpoint": "http://some-micro-service-v1:8080",
	  "routes": [
	      {
	          "prefix": "/api/v2/some-micro-service/v2-before-promote",
	          "prefixRewrite": "/api/v2/v2-before-promote"
	      }
	  ],
	  "version": "v1"
	}]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, rr.StatusCode)
}

func Test_AfterBlueGreenCycle_NewRequestForExistingEndpointAndOldRoute_ThenNoNewRoute(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/routes/v1/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV1.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})
	app.Delete("/versions/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandleDeleteDeploymentVersionWithID(ctx)
	})
	app.Get("/routes/:nodeGroup/:virtualServiceName", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV3.HandleGetVirtualService(ctx)
	})

	// registering two routes:
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action
	// and http://some-micro-service-2-v1:8080/api/v2/some-micro-service-2/action
	// deploymentVersion=v1
	rr, err := app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
       "allowed": true,
       "cluster": "some-micro-service-1",
       "endpoint": "http://some-micro-service-1-v1:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-1/action",
               "prefixRewrite": "/api/v2/s1/action"
           }
       ],
       "version": "v1"
   },
	{
       "allowed": true,
       "cluster": "some-micro-service-2",
       "endpoint": "http://some-micro-service-2-v1:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-2/action",
               "prefixRewrite": "/api/v2/s1/action"
           }
       ],
       "version": "v1"
   }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	// installing candidate for some-micro-service-2:
	//     http://some-micro-service-2-v2:8080/api/v2/some-micro-service-2/action
	// deploymentVersion=v2
	rr, err = app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
       "allowed": true,
       "cluster": "some-micro-service-2",
       "endpoint": "http://some-micro-service-2-v2:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-2/action",
               "prefixRewrite": "/api/v2/s2/action"
           }
       ],
       "version": "v2"
   }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	// promoting v2
	rr, err = app.Test(makePromoteRequest("v2"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, rr.StatusCode)

	// switching to rolling mode. delete v1
	rr, err = app.Test(deleteVersion("v1"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusOK, rr.StatusCode)

	// updating some-micro-service-1
	// new request with old host some-micro-service-1-v1 but with new deploymentVersion=v2 after promotion
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action
	rr, err = app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
       "allowed": true,
       "cluster": "some-micro-service-1",
       "endpoint": "http://some-micro-service-1-v1:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-1/action",
               "prefixRewrite": "/api/v2/s1/action"
           }
       ],
       "version": "v2"
   }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	// getting routes for node group bg-test-ng-1
	// there expects two routes:
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action  - initialDeploymentVersion=v1
	// and http://some-micro-service-2-v2:8080/api/v2/some-micro-service-2/action  - initialDeploymentVersion=v2
	// deploymentVersion=v2 for both
	rr, err = app.Test(getRoutesWithNodeGroupAndVirtualServiceName("bg-test-ng-1", "bg-test-ng-1"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusOK, rr.StatusCode)

	res, err := io.ReadAll(rr.Body)
	assert.Nil(err)
	assert.NotNil(res)

	var response dto.VirtualServiceResponse
	err = json.Unmarshal(res, &response)
	assert.Nil(err)

	assert.Equal(2, len(response.VirtualHost.Routes))
}

func Test_AfterBlueGreenCycle_NewRequestForExistingEndpointAndNewRoute_ThenNewRoute(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/routes/v1/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV1.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})
	app.Delete("/versions/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandleDeleteDeploymentVersionWithID(ctx)
	})
	app.Get("/routes/:nodeGroup/:virtualServiceName", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV3.HandleGetVirtualService(ctx)
	})

	// registering two routes:
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action
	// and http://some-micro-service-2-v1:8080/api/v2/some-micro-service-2/action
	// deploymentVersion=v1
	rr, err := app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
       "allowed": true,
       "cluster": "some-micro-service-1",
       "endpoint": "http://some-micro-service-1-v1:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-1/action",
               "prefixRewrite": "/api/v2/s1/action"
           }
       ],
       "version": "v1"
   },
	{
       "allowed": true,
       "cluster": "some-micro-service-2",
       "endpoint": "http://some-micro-service-2-v1:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-2/action",
               "prefixRewrite": "/api/v2/s2/action"
           }
       ],
       "version": "v1"
   }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	// installing candidate for some-micro-service-2:
	//     http://some-micro-service-2-v2:8080/api/v2/some-micro-service-2/action
	// deploymentVersion=v2
	rr, err = app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
       "allowed": true,
       "cluster": "some-micro-service-2",
       "endpoint": "http://some-micro-service-2-v2:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-2/action",
               "prefixRewrite": "/api/v2/s2/action"
           }
       ],
       "version": "v2"
   }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	// promoting v2
	rr, err = app.Test(makePromoteRequest("v2"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusAccepted, rr.StatusCode)

	rr, err = app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
	  "allowed": true,
	  "cluster": "some-micro-service-1",
	  "endpoint": "http://some-micro-service-1-v1:8080",
	  "routes": [
	      {
	          "prefix": "/api/v2/some-micro-service-1/action/new",
	          "prefixRewrite": "/api/v2/s1/action/new"
	      }
	  ],
	  "version": "v1"
	}]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	// switching to rolling mode. delete v1
	rr, err = app.Test(deleteVersion("v1"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusOK, rr.StatusCode)

	// updating some-micro-service-1
	// new request with old host some-micro-service-1-v1 but with new deploymentVersion=v2 after promotion
	// and new route
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action
	rr, err = app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
       "allowed": true,
       "cluster": "some-micro-service-1",
       "endpoint": "http://some-micro-service-1-v1:8080",
       "routes": [
           {
               "prefix": "/api/v2/some-micro-service-1/action/new",
               "prefixRewrite": "/api/v2/s1/action/new"
           }
       ],
       "version": "v2"
   }]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	rr, err = app.Test(makeV2RequestWithNodeGroup("bg-test-ng-1", `
	[{
	  "allowed": true,
	  "cluster": "some-micro-service-1",
	  "endpoint": "http://some-micro-service-1-v1:8080",
	  "routes": [
	      {
	          "prefix": "/api/v2/some-micro-service-1/action/new1",
	          "prefixRewrite": "/api/v2/s1/action/new1"
	      }
	  ],
	  "version": "v1"
	}]`), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	// getting routes for node group bg-test-ng-1
	// there expects three routes:
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action  		- initialDeploymentVersion=v1
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action/new  	- initialDeploymentVersion=v1
	//     http://some-micro-service-1-v1:8080/api/v2/some-micro-service-1/action/new1  - initialDeploymentVersion=v1
	//     http://some-micro-service-2-v2:8080/api/v2/some-micro-service-2/action  		- initialDeploymentVersion=v2
	// deploymentVersion=v2 for all
	rr, err = app.Test(getRoutesWithNodeGroupAndVirtualServiceName("bg-test-ng-1", "bg-test-ng-1"), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusOK, rr.StatusCode)

	res, err := io.ReadAll(rr.Body)
	assert.Nil(err)
	assert.NotNil(res)

	var response dto.VirtualServiceResponse
	err = json.Unmarshal(res, &response)
	assert.Nil(err)

	assert.Equal(4, len(response.VirtualHost.Routes))
}

func Test_ProhibitRouteInActiveVersion(t *testing.T) {
	assert := asrt.New(t)
	testEnv := InitCPTestEnvironment()

	app, err := fiberserver.New().Process()
	assert.Nil(err)
	app.Post("/routes/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV2.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/routes/v1/:nodeGroup", func(ctx fiber.Ctx) error {
		return testEnv.RouteControllerV1.HandlePostRoutesWithNodeGroup(ctx)
	})
	app.Post("/promote/:version", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostPromoteVersion(ctx)
	})
	app.Post("/rollback", func(ctx fiber.Ctx) error {
		return testEnv.BlueGreenControllerV2.HandlePostRollbackVersion(ctx)
	})

	request := httptest.NewRequest(http.MethodPost, "/routes/private-gateway-service", strings.NewReader(
		`[
		{
			"allowed": true,
			"cluster": "trace-service",
			"endpoint": "http://trace-service-v1:8080",
			"routes": [
				{
					"prefix": "/trace-service/health",
					"prefixRewrite": "/health"
				}
			],
			"version": "v1"
		},
		{
			"allowed": true,
			"cluster": "trace-service",
			"endpoint": "http://trace-service-v1:8080",
			"routes": [
				{
					"prefix": "/trace-service/trace",
					"prefixRewrite": "/trace"
				}
			],
			"version": "v1"
		},
		{
			"allowed": true,
			"cluster": "trace-service",
			"endpoint": "http://trace-service-v1:8080",
			"routes": [
				{
					"prefix": "/trace-service/proxy",
					"prefixRewrite": "/proxy"
				}
			],
			"version": "v1"
		}
	]`))

	rr, err := app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	request = httptest.NewRequest(http.MethodPost, "/routes/private-gateway-service", strings.NewReader(
		`[
		{
			"allowed": true,
			"cluster": "trace-service",
			"endpoint": "http://trace-service-v2:8080",
			"routes": [
				{
					"prefix": "/trace-service/health",
					"prefixRewrite": "/health"
				}
			],
			"version": "v2"
		},
		{
			"allowed": true,
			"cluster": "trace-service",
			"endpoint": "http://trace-service-v2:8080",
			"routes": [
				{
					"prefix": "/trace-service/v2/trace",
					"prefixRewrite": "/trace"
				}
			],
			"version": "v2"
		},
		{
			"allowed": true,
			"cluster": "trace-service",
			"endpoint": "http://trace-service-v2:8080",
			"routes": [
				{
					"prefix": "/trace-service/proxy",
					"prefixRewrite": "/proxy"
				}
			],
			"version": "v2"
		}
	]`))

	rr, err = app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(err)
	assert.Equal(http.StatusCreated, rr.StatusCode)

	routes, err := testEnv.Dao.FindRoutesByAutoGeneratedAndDeploymentVersion(true, "v2")
	assert.Nil(err)
	var v2TraceRoute *domain.Route
	for _, route := range routes {
		if route.Prefix == "/trace-service/trace" {
			v2TraceRoute = route
			break
		}
	}
	assert.NotNil(v2TraceRoute)
	if v2TraceRoute != nil {
		assert.True(v2TraceRoute.IsProhibit())
	}
}
