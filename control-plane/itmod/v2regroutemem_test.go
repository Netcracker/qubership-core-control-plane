package itmod

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

var (
	regRouteRequestRaw = `[
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
                "prefix": "/api/v1/order-backend/debug",
                "prefixRewrite": "/api/v1/debug"
            },
            {
                "prefix": "/api/v1/order-backend/trace",
                "prefixRewrite": "/api/v1/trace"
            },
            {
                "prefix": "/api/v1/order-backend/customer",
                "prefixRewrite": "/api/v1/customer"
            },
            {
                "prefix": "/api/v1/order-backend/double",
                "prefixRewrite": "/api/v1/double"
            },
            {
                "prefix": "/api/v1/order-backend/zzzz",
                "prefixRewrite": "/api/v1/zzzz"
            }
        ],
        "version": "v1"
    }
]`
)

func BenchmarkRegRouteV2(b *testing.B) {
	configloader.Init(configloader.EnvPropertySource())
	app, _ := fiberserver.New().Process()

	for i := 0; i < b.N; i++ {
		app.Test(makeV2Request(regRouteRequestRaw), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	}
}
