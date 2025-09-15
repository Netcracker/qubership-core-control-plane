package routeconfig

import (
	"os"
	"strings"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

func NewEgressVirtualHostBuilder(dao dao.Repository, routeBuilder RouteBuilder, provider VersionAliasesProvider) *GatewayVirtualHostBuilder {
		origins := "*"
		if value, exists := os.LookupEnv("GATEWAYS_ALLOWED_ORIGIN"); exists {
			origins = value
		}
		allowedHeaders := "*"
		if value, exists := os.LookupEnv("GATEWAYS_ALLOWED_HEADERS"); exists {
			allowedHeaders = value
		}
		maxAge := "-1"
		if value, exists := os.LookupEnv("GATEWAYS_ACCESS_CONTROL_MAX_AGE"); exists {
			maxAge = value
		}

		responseHeadersToRemove := []string{"server",
											"X-Forwarded-For",
											"X-Forwarded-Proto",
											"X-Forwarded-Host",
											"X-Forwarded-Port",
											"Via",
											"X-Real-IP"}
		if value, exists := os.LookupEnv("EGRESS_RESPONSE_HEADERS_TO_REMOVE"); exists {
			responseHeadersToRemove = strings.Split(value, ", ")
			responseHeadersToRemove = append(responseHeadersToRemove, "server")
		}
		compositePlatformEnv, exists := os.LookupEnv("COMPOSITE_PLATFORM")
		compositePlatform := exists && strings.EqualFold(strings.TrimSpace(compositePlatformEnv), "true")
		namespace := configloader.GetOrDefaultString("microservice.namespace", "")
	
		return &GatewayVirtualHostBuilder{dao: dao, routeBuilder: routeBuilder, allowedHeaders: allowedHeaders,
			responseHeadersToRemove: responseHeadersToRemove, maxAge: maxAge, originStringMatchers: convertOrigins(origins),
			builderExt: &gatewayVhBuilderExt{
				origins:            origins,
				allowedHeaders:     allowedHeaders,
				maxAge:             maxAge,
				aliasProvider:      provider,
				compositeSatellite: compositePlatform,
			},
			namespace: namespace,
		}
	}

type egressVirtualHostBuilderExt struct {
	dao           dao.Repository
	entityService entity.ServiceInterface
}

func (i *egressVirtualHostBuilderExt) BuildExtAuthzPerRoute(virtualHost *domain.VirtualHost) (*any.Any, error) {
	return buildCustomExtAuthzPerRoute(i.dao, i.entityService, virtualHost)
}

func (i *egressVirtualHostBuilderExt) EnrichHeadersToRemove(headersToRemove []string) []string {
	return headersToRemove
}
