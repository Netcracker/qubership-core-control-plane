package v3

import (
	"strings"
	"testing"

	v3route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	envoy "github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
)

const (
	quotePrefixPath = "/test-service/quoteManagement/v6/quote"
	quoteRegexpPath = "/test-service/quoteManagement/v6/quote/([^/]+)(/.*)?"
)

func TestBuildRouteConfig_DenyRoutesOrder(t *testing.T) {
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	v3Service, inMemDao := getV3Service()
	assert.Nil(t, v3Service.RegisterRoutingConfig(nil, createV3RequestWithDenyRoutes()))

	routeConfigs, err := inMemDao.FindAllRouteConfigs()
	assert.Nil(t, err)
	var routeConfig *domain.RouteConfiguration
	for _, rc := range routeConfigs {
		if rc.NodeGroupId == domain.PublicGateway {
			routeConfig = rc
		}
	}
	if !assert.NotNil(t, routeConfig) {
		return
	}

	envoyConfigBuilder := envoy.DefaultEnvoyConfigurationBuilder(inMemDao, entity.NewService("v1"), v3Service)

	// route input order differs between builds because RouteMultiVersionPreparer groups routes in a map
	var firstBuildOrder []string
	for build := 0; build < 50; build++ {
		envoyRouteConfig, err := envoyConfigBuilder.BuildRouteConfig(routeConfig)
		assert.Nil(t, err)
		assert.Len(t, envoyRouteConfig.VirtualHosts, 1)
		envoyRoutes := envoyRouteConfig.VirtualHosts[0].Routes

		order := make([]string, len(envoyRoutes))
		lastDenyIdx, firstAllowIdx := -1, -1
		for i, envoyRoute := range envoyRoutes {
			order[i] = routeMatchString(envoyRoute)
			if envoyRoute.GetDirectResponse() != nil {
				lastDenyIdx = i
			} else if firstAllowIdx < 0 {
				firstAllowIdx = i
			}
		}
		assert.GreaterOrEqual(t, lastDenyIdx, 0, "deny routes not found: %v", order)
		assert.Less(t, lastDenyIdx, firstAllowIdx, "deny routes must go before allowed routes, build %d: %v", build, order)
		quoteRegexpIdx := indexOf(order, quoteRegexpPath)
		assert.GreaterOrEqual(t, quoteRegexpIdx, 0, "%s not found: %v", quoteRegexpPath, order)
		for _, prefix := range []string{quotePrefixPath, quotePrefixPath + "/"} {
			assert.Less(t, quoteRegexpIdx, indexOf(order, prefix), "%s must go before %s, build %d: %v", quoteRegexpPath, prefix, build, order)
		}

		if firstBuildOrder == nil {
			firstBuildOrder = order
		} else {
			assert.Equal(t, firstBuildOrder, order, "route order must not depend on build, build %d", build)
		}
	}
}

func routeMatchString(envoyRoute *v3route.Route) string {
	if regex := envoyRoute.GetMatch().GetSafeRegex(); regex != nil {
		return regex.GetRegex()
	}
	return envoyRoute.GetMatch().GetPrefix()
}

func indexOf(values []string, value string) int {
	for i, v := range values {
		if v == value {
			return i
		}
	}
	return -1
}

func createV3RequestWithDenyRoutes() dto.RoutingConfigRequestV3 {
	deny := true
	rules := []dto.Rule{
		{Match: dto.RouteMatch{Prefix: "/test-service/quoteManagement/v6/quote"}},
		{Match: dto.RouteMatch{Prefix: "/test-service/quoteManagement/v6/quote/{quoteId}"}},
		{Match: dto.RouteMatch{Prefix: "/test-service/quoteManagement/v6/hidden/details"}},
		{Deny: &deny, Match: dto.RouteMatch{Prefix: "/test-service/quoteManagement/v6/hidden"}},
		{Deny: &deny, Match: dto.RouteMatch{Prefix: "/test-service/internal"}},
	}
	// filler routes have distinct lengths shorter than the quote routes, so the expected order is unambiguous
	for i := 0; i < 15; i++ {
		rules = append(rules, dto.Rule{Match: dto.RouteMatch{Prefix: "/f/" + strings.Repeat("x", 2*i+1)}})
	}
	return dto.RoutingConfigRequestV3{
		Gateways: []string{domain.PublicGateway},
		VirtualServices: []dto.VirtualService{
			{
				Name:  "quote-vs",
				Hosts: []string{"*"},
				RouteConfiguration: dto.RouteConfig{
					Routes: []dto.RouteV3{
						{
							Rules: rules,
							Destination: dto.RouteDestination{
								Cluster:  "test-service",
								Endpoint: "http://test-service-v1:8080",
							},
						},
					},
				},
			},
		},
	}
}
