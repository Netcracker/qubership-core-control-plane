package routes

import (
	"time"

	routeregistration "github.com/netcracker/qubership-core-lib-go-rest-utils/v2/route-registration"
)

// Fixture for httproute-from-code — see README.md next to this file.
func RegisterRoutes() {
	routeregistration.NewRegistrar().WithRoutes(
		routeregistration.Route{
			From:      "/api/v1/svc/{id}",
			To:        "/svc/{id}",
			RouteType: routeregistration.Public,
		},
		routeregistration.Route{
			From:      "/api/v1/svc/{id}/admin",
			To:        "/svc/{id}/admin",
			RouteType: routeregistration.Internal,
		},
		routeregistration.Route{
			From:      "/api/v1/svc/orders/{id}/items",
			To:        "/svc/orders/{id}/items",
			RouteType: routeregistration.Private,
			Timeout:   30 * time.Second,
		},
		routeregistration.Route{
			From:      "/api/v1/svc/debug",
			To:        "/svc/debug",
			RouteType: routeregistration.Public,
			Forbidden: true,
		},
	).Register()
}
