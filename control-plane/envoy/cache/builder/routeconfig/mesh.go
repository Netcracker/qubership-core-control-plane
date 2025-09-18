package routeconfig

import (
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	protoAny "github.com/golang/protobuf/ptypes/any"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"strings"
)

type MeshVirtualHostBuilder struct {
	dao           dao.Repository
	entityService entity.ServiceInterface
	routeBuilder  RouteBuilder
}

func NewMeshVirtualHostBuilder(dao dao.Repository, entityService entity.ServiceInterface, routeBuilder RouteBuilder) *MeshVirtualHostBuilder {
	return &MeshVirtualHostBuilder{dao: dao, entityService: entityService, routeBuilder: routeBuilder}
}

func (b *MeshVirtualHostBuilder) BuildVirtualHosts(routeConfig *domain.RouteConfiguration) ([]*route.VirtualHost, error) {
	result := make([]*route.VirtualHost, 0, len(routeConfig.VirtualHosts))
	for _, virtualHost := range routeConfig.VirtualHosts {
		virtualHost.RouteConfiguration = routeConfig
		domainStrings, err := b.getDomains(virtualHost.Id)
		if err != nil {
			logger.Errorf("Failed to load virtual host domains for virtual host %v using DAO: %v", virtualHost.Id, err)
			return nil, err
		}
		routes, err := b.dao.FindRoutesByVirtualHostId(virtualHost.Id)
		if err != nil {
			logger.Errorf("Failed to load routes by virtual host id %v using DAO: %v", virtualHost.Id, err)
			return nil, err
		}
		envoyRoutes, routeRateLimits, err := b.routeBuilder.BuildRoutes(routes)
		if err != nil {
			return nil, err
		}
		if virtualHost.RateLimitId != "" {
			virtualHost.RateLimit, err = b.dao.FindRateLimitByNameWithHighestPriority(virtualHost.RateLimitId)
			if err != nil {
				logger.Errorf("Failed to load virtual host rateLimit using DAO:\n %v", err)
				return nil, err
			}
		}
		typedPerFilterConfig, err := buildTypedPerFilterConfig(virtualHost, routeRateLimits, b.buildCustomExtAuthzPerRoute, nil)
		if err != nil {
			return nil, err
		}
		envoyVirtualHost := &route.VirtualHost{
			Name:                   virtualHost.Name,
			Domains:                domainStrings,
			TypedPerFilterConfig:   typedPerFilterConfig,
			RequestHeadersToAdd:    buildHeaderOptions(virtualHost.RequestHeadersToAdd),
			RequestHeadersToRemove: virtualHost.RequestHeadersToRemove,
			ResponseHeadersToAdd: buildHeaderOptions([]domain.Header{{
				Name:  "Access-Control-Allow-Origin",
				Value: "%DYNAMIC_METADATA(envoy.filters.http.ext_authz:access.control.allow.origin)%",
			}}),
			ResponseHeadersToRemove: []string{"server"},
			Routes: envoyRoutes,
		}
		result = append(result, envoyVirtualHost)
	}
	return result, nil
}

func (b *MeshVirtualHostBuilder) getDomains(virtualHostId int32) ([]string, error) {
	domains, err := b.dao.FindVirtualHostDomainByVirtualHostId(virtualHostId)
	if err != nil {
		return nil, err
	}
	domainMap := make(map[string]bool)
	for _, vhDomain := range domains {
		domainMap[vhDomain.Domain] = true
	}

	if tlsmode.GetMode() == tlsmode.Preferred {
		for key, _ := range domainMap {
			if strings.Contains(key, ":8080") {
				newDomainString := strings.Replace(key, ":8080", ":8443", 1)
				domainMap[newDomainString] = true
			}
		}
	}

	return util.MapKeysToSlice(domainMap), nil
}

func (b *MeshVirtualHostBuilder) buildCustomExtAuthzPerRoute(virtualHost *domain.VirtualHost) (*protoAny.Any, error) {
	return buildCustomExtAuthzPerRoute(b.dao, b.entityService, virtualHost)
}
