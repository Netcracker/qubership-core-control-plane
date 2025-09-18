package routeconfig

import (
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	ext_authz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	envoy_type_matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/bg"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"os"
	"strings"
)

type VirtualHostBuilderExt interface {
	BuildExtAuthzPerRoute(virtualHost *domain.VirtualHost) (*any.Any, error)
	EnrichHeadersToRemove(headersToRemove []string) []string
}

type GatewayVirtualHostBuilder struct {
	dao                     dao.Repository
	routeBuilder            RouteBuilder
	allowedHeaders          string
	maxAge                  string
	originStringMatchers    []*envoy_type_matcher.StringMatcher
	builderExt              VirtualHostBuilderExt
	namespace               string
	responseHeadersToRemove []string
}

func NewGatewayVirtualHostBuilder(dao dao.Repository, routeBuilder RouteBuilder, provider VersionAliasesProvider) *GatewayVirtualHostBuilder {
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

	responseHeadersToRemove := []string{"server"}

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

func (vhBuilder *GatewayVirtualHostBuilder) BuildVirtualHosts(routeConfig *domain.RouteConfiguration) ([]*route.VirtualHost, error) {
	result := make([]*route.VirtualHost, 0, len(routeConfig.VirtualHosts))
	for _, virtualHost := range routeConfig.VirtualHosts {
		virtualHost.RouteConfiguration = routeConfig
		domains, err := vhBuilder.dao.FindVirtualHostDomainByVirtualHostId(virtualHost.Id)
		if err != nil {
			logger.Errorf("Failed to load virtual host domains for virtual host %v using DAO: %v", virtualHost.Id, err)
			return nil, err
		}
		domainStrings := make([]string, len(domains))
		for idx, vhDomain := range domains {
			domainStrings[idx] = vhDomain.Domain
		}

		if virtualHost.RateLimitId != "" {
			virtualHost.RateLimit, err = vhBuilder.dao.FindRateLimitByNameWithHighestPriority(virtualHost.RateLimitId)
			if err != nil {
				logger.Errorf("Failed to load virtual host rateLimit using DAO:\n %v", err)
				return nil, err
			}
		}

		routes, err := vhBuilder.dao.FindRoutesByVirtualHostId(virtualHost.Id)
		if err != nil {
			logger.Errorf("Failed to load routes by virtual host id %v using DAO: %v", virtualHost.Id, err)
			return nil, err
		}
		envoyRoutes, routeRateLimits, err := vhBuilder.routeBuilder.BuildRoutes(routes)
		if err != nil {
			return nil, err
		}

		if routeConfig.NodeGroupId == "private-gateway-service" {
			namespaceRoute := &route.Route{
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/api/v3/control-plane/namespace"},
				},
				Action: &route.Route_DirectResponse{
					DirectResponse: &route.DirectResponseAction{
						Status: 200,
						Body: &core.DataSource{
							Specifier: &core.DataSource_InlineString{InlineString: vhBuilder.namespace},
						},
					},
				},
			}
			envoyRoutes = append(envoyRoutes, namespaceRoute)
		}

		responseHeadersToAdd := make([]*core.HeaderValueOption, 3)
		responseHeadersToAdd[0] = &core.HeaderValueOption{
			Header: &core.HeaderValue{
				Key:   "X-Content-Type-Options",
				Value: "nosniff",
			},
		}
		responseHeadersToAdd[1] = &core.HeaderValueOption{
			Header: &core.HeaderValue{
				Key:   "Access-Control-Allow-Origin",
				Value: "%DYNAMIC_METADATA(envoy.filters.http.ext_authz:access.control.allow.origin)%",
			},
		}
		responseHeadersToAdd[2] = &core.HeaderValueOption{
			Header: &core.HeaderValue{
				Key:   "X-XSS-Protection",
				Value: "0",
			},
		}

		corsPolicy, err := buildCorsPolicy(virtualHost, vhBuilder)
		if err != nil {
			return nil, err
		}

		typedPerFilterConfig, err := buildTypedPerFilterConfig(virtualHost, routeRateLimits, vhBuilder.builderExt.BuildExtAuthzPerRoute, corsPolicy)
		if err != nil {
			return nil, err
		}

		envoyVirtualHost := &route.VirtualHost{
			Name:                    virtualHost.Name,
			Domains:                 domainStrings,
			Routes:                  envoyRoutes,
			RequestHeadersToAdd:     buildHeaderOptions(virtualHost.RequestHeadersToAdd),
			RequestHeadersToRemove:  vhBuilder.builderExt.EnrichHeadersToRemove(virtualHost.RequestHeadersToRemove),
			TypedPerFilterConfig:    typedPerFilterConfig,
			ResponseHeadersToAdd:    responseHeadersToAdd,
			ResponseHeadersToRemove: vhBuilder.responseHeadersToRemove,
		}
		result = append(result, envoyVirtualHost)
	}
	return result, nil
}

type gatewayVhBuilderExt struct {
	origins                 string
	allowedHeaders          string
	maxAge                  string
	aliasProvider           VersionAliasesProvider
	compositeSatellite      bool
}

func (vhBuilder *gatewayVhBuilderExt) EnrichHeadersToRemove(headersToRemove []string) []string {
	if !vhBuilder.compositeSatellite {
		return append(headersToRemove, "X-Token-Signature")
	}
	return headersToRemove
}

func (vhBuilder *gatewayVhBuilderExt) BuildExtAuthzPerRoute(virtualHost *domain.VirtualHost) (*any.Any, error) {
	//todo refactor role creation, all virtualHosts must have role which is valid to auth_ext.
	//But some virtualHosts (i.e. active-active ones) have names, that do not have corresponding roles in auth_ext
	//For example we can set 'role' property on domain.VirtualHost and get role value from it, instead of getting it from the name
	var roleName string
	if strings.HasPrefix(virtualHost.Name, "public-gateway-service") {
		roleName = "public-gateway-service"
	} else if strings.HasPrefix(virtualHost.Name, "private-gateway-service") {
		roleName = "private-gateway-service"
	} else {
		roleName = virtualHost.Name
	}
	extAuthzPerRoute := &ext_authz.ExtAuthzPerRoute{
		Override: &ext_authz.ExtAuthzPerRoute_CheckSettings{
			CheckSettings: &ext_authz.CheckSettings{
				ContextExtensions: map[string]string{
					"role":                 roleName,
					"cors.allowed_origins": vhBuilder.origins,
					"cors.allowed_headers": vhBuilder.allowedHeaders,
					"cors.max_age":         vhBuilder.maxAge,
				},
			},
		},
	}
	if bg.GetMode() == bg.BlueGreen1 {
		extAuthzPerRoute.Override.(*ext_authz.ExtAuthzPerRoute_CheckSettings).CheckSettings.ContextExtensions["aliases"] = vhBuilder.aliasProvider.GetVersionAliases()
	}
	marshalledExtAuthz, err := ptypes.MarshalAny(extAuthzPerRoute)
	if err != nil {
		logger.Errorf("routeconfig: failed to marshal ExtAuthzPerRoute config to protobuf Any")
		return nil, err
	}
	return marshalledExtAuthz, nil
}
