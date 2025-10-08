package routeconfig

import (
	"os"
	"slices"
	"strings"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

var (
	responseHeadersToRemove = []string{"server"}
	requestHeadersToRemove = []string{"X-Token-Signature"}
	commonHeadersToRemove = []string{"X-Forwarded-For",
									"X-Forwarded-Proto",
									"X-Forwarded-Host",
									"X-Forwarded-Port",
									"Via",
									"X-Real-IP"}
)

func NewEgressVirtualHostBuilder(dao dao.Repository, entityService entity.ServiceInterface, routeBuilder RouteBuilder) *GatewayVirtualHostBuilder {
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

		mergedHeadersToRemove := slices.Concat(responseHeadersToRemove, commonHeadersToRemove)
		if value, exists := os.LookupEnv("EGRESS_RESPONSE_HEADERS_TO_REMOVE"); exists {
			envHeadersToRemove := strings.Split(value, ", ")
			mergedHeadersToRemove = slices.Concat(responseHeadersToRemove, envHeadersToRemove)
		}
		namespace := configloader.GetOrDefaultString("microservice.namespace", "")
	
		return &GatewayVirtualHostBuilder{dao: dao, routeBuilder: routeBuilder, allowedHeaders: allowedHeaders,
			responseHeadersToRemove: mergedHeadersToRemove, maxAge: maxAge, originStringMatchers: convertOrigins(origins),
			builderExt: &egressVirtualHostBuilderExt{
				dao:                dao, 
				entityService:      entityService,
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
	mergedHeadersToRemove := slices.Concat(requestHeadersToRemove, commonHeadersToRemove, headersToRemove)
	if value, exists := os.LookupEnv("EGRESS_REQUEST_HEADERS_TO_REMOVE"); exists {
		envHeadersToRemove := strings.Split(value, ", ")
		mergedHeadersToRemove = slices.Concat(requestHeadersToRemove, envHeadersToRemove, headersToRemove)
	}
	return mergedHeadersToRemove
}
