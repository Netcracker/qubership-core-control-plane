package routeconfig

import (
	"errors"
	eroute "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	mock_routeconfig "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/envoy/cache/builder/routeconfig"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestGetDomainsWhenTlsDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_ = os.Setenv("INTERNAL_TLS_ENABLED", "false")
	defer os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	gatewayVirtualHostBuilder := NewMeshVirtualHostBuilder(mockDao, entity.NewService("v1"), mockRouteBuilder)

	domains := []*domain.VirtualHostDomain{
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc.cluster.local:1234",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc.cluster.local:8080",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc:1234",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc:8080",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1:1234",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1:8080",
		},
		{
			Domain: "trace-service:1234",
		},
		{
			Domain: "trace-service:8080",
		},
	}
	mockDao.EXPECT().FindVirtualHostDomainByVirtualHostId(gomock.Any()).Return(domains, nil)

	domainsResult, err := gatewayVirtualHostBuilder.getDomains(0)
	assert.Nil(t, err)
	assert.NotNil(t, domainsResult)
	assert.Equal(t, 8, len(domainsResult))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc.cluster.local:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service:8080"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1:8080"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc:8080"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc.cluster.local:8080"))
	assert.False(t, util.SliceContainsElement(domainsResult, "trace-service:8443"))
	assert.False(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1:8443"))
	assert.False(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc:8443"))
	assert.False(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc.cluster.local:8443"))
}

func TestGetDomainsWhenTlsEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_ = os.Setenv("INTERNAL_TLS_ENABLED", "true")
	defer os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	gatewayVirtualHostBuilder := NewMeshVirtualHostBuilder(mockDao, entity.NewService("v1"), mockRouteBuilder)

	domains := []*domain.VirtualHostDomain{
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc.cluster.local:1234",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc.cluster.local:8080",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc:1234",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1.svc:8080",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1:1234",
		},
		{
			Domain: "trace-service.cloud-core-for-dev-1:8080",
		},
		{
			Domain: "trace-service:1234",
		},
		{
			Domain: "trace-service:8080",
		},
		// method should not duplicate domain
		{
			Domain: "trace-service.cloud-core-for-dev-1:8443",
		},
	}
	mockDao.EXPECT().FindVirtualHostDomainByVirtualHostId(gomock.Any()).Return(domains, nil)

	domainsResult, err := gatewayVirtualHostBuilder.getDomains(0)
	assert.Nil(t, err)
	assert.NotNil(t, domainsResult)
	assert.Equal(t, 12, len(domainsResult))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc.cluster.local:1234"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service:8080"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1:8080"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc:8080"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc.cluster.local:8080"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service:8443"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1:8443"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc:8443"))
	assert.True(t, util.SliceContainsElement(domainsResult, "trace-service.cloud-core-for-dev-1.svc.cluster.local:8443"))
}

func TestGatewayVirtualHostBuilderBuildVirtualHosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	mockProvider := getMockVersionAliasesProvider(ctrl)
	gatewayVirtualHostBuilder := NewGatewayVirtualHostBuilder(mockDao, mockRouteBuilder, mockProvider)

	initRouteConfig, virtualHostsDomain, eRoutes := mockTestData(mockDao, mockRouteBuilder)

	mockProvider.EXPECT().GetVersionAliases().Return("testAliases").AnyTimes()

	resultVirtualHosts, err := gatewayVirtualHostBuilder.BuildVirtualHosts(initRouteConfig)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(resultVirtualHosts))
	for i, resultVirtualHost := range resultVirtualHosts {
		assert.Equal(t, initRouteConfig.VirtualHosts[i].Name, resultVirtualHost.Name)
		assert.Equal(t, virtualHostsDomain[i].Domain, resultVirtualHost.Domains[0])
		assert.Equal(t, []*eroute.Route{eRoutes[i]}, resultVirtualHost.Routes)
		assert.Equal(t, initRouteConfig.VirtualHosts[i].RequestHeadersToAdd[0].Name, resultVirtualHost.RequestHeadersToAdd[0].Header.Key)
		assert.Equal(t, initRouteConfig.VirtualHosts[i].RequestHeadersToAdd[0].Value, resultVirtualHost.RequestHeadersToAdd[0].Header.Value)
		assert.Equal(t, "X-Token-Signature", resultVirtualHost.RequestHeadersToRemove[0])
	}

	assert.Nil(t, resultVirtualHosts[0].TypedPerFilterConfig["envoy.filters.http.local_ratelimit"])
	assert.NotNil(t, resultVirtualHosts[1].TypedPerFilterConfig["envoy.filters.http.local_ratelimit"])
}

func TestFacadeVirtualHostBuilderBuildVirtualHosts_shouldReturnError_whenBuildRoutesFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	facadeVirtualHostBuilder := NewMeshVirtualHostBuilder(mockDao, entity.NewService("v1"), mockRouteBuilder)

	mockDao.EXPECT().FindVirtualHostDomainByVirtualHostId(gomock.Any()).Return([]*domain.VirtualHostDomain{}, nil)

	mockDao.EXPECT().FindRoutesByVirtualHostId(gomock.Any()).Return([]*domain.Route{}, nil)

	testError := errors.New("test")
	mockRouteBuilder.EXPECT().BuildRoutes(gomock.Any()).Return(nil, nil, testError)

	initVirtualHosts := []*domain.VirtualHost{
		{Id: int32(1), Name: "vh1"},
	}
	initRouteConfig := &domain.RouteConfiguration{Id: 1, Name: "test-route-config", VirtualHosts: initVirtualHosts}
	resultVirtualHosts, err := facadeVirtualHostBuilder.BuildVirtualHosts(initRouteConfig)
	assert.Nil(t, resultVirtualHosts)
	assert.NotNil(t, err)
	assert.Equal(t, testError, err)
}

func TestFacadeVirtualHostBuilderBuildVirtualHosts_shouldReturnError_whenFindRoutesFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	facadeVirtualHostBuilder := NewMeshVirtualHostBuilder(mockDao, entity.NewService("v1"), mockRouteBuilder)

	mockDao.EXPECT().FindVirtualHostDomainByVirtualHostId(gomock.Any()).Return([]*domain.VirtualHostDomain{}, nil)

	testError := errors.New("test")
	mockDao.EXPECT().FindRoutesByVirtualHostId(gomock.Any()).Return(nil, testError)

	initVirtualHosts := []*domain.VirtualHost{
		{Id: int32(1), Name: "vh1"},
	}
	initRouteConfig := &domain.RouteConfiguration{Id: 1, Name: "test-route-config", VirtualHosts: initVirtualHosts}
	resultVirtualHosts, err := facadeVirtualHostBuilder.BuildVirtualHosts(initRouteConfig)
	assert.Nil(t, resultVirtualHosts)
	assert.NotNil(t, err)
	assert.Equal(t, testError, err)
}

func TestFacadeVirtualHostBuilderBuildVirtualHosts_shouldReturnError_whenFindDomainFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	facadeVirtualHostBuilder := NewMeshVirtualHostBuilder(mockDao, entity.NewService("v1"), mockRouteBuilder)

	testError := errors.New("test")
	mockDao.EXPECT().FindVirtualHostDomainByVirtualHostId(gomock.Any()).Return(nil, testError)

	initVirtualHosts := []*domain.VirtualHost{
		{Id: int32(1), Name: "vh1"},
	}
	initRouteConfig := &domain.RouteConfiguration{Id: 1, Name: "test-route-config", VirtualHosts: initVirtualHosts}
	resultVirtualHosts, err := facadeVirtualHostBuilder.BuildVirtualHosts(initRouteConfig)
	assert.Nil(t, resultVirtualHosts)
	assert.NotNil(t, err)
	assert.Equal(t, testError, err)
}

func TestFacadeVirtualHostBuilderBuildVirtualHosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	facadeVirtualHostBuilder := NewMeshVirtualHostBuilder(mockDao, entity.NewService("v1"), mockRouteBuilder)

	mockDao.EXPECT().FindListenersByNodeGroupId("test-gw").Times(2).Return([]*domain.Listener{{NodeGroupId: "test-gw"}}, nil)
	initRouteConfig, virtualHostsDomain, eRoutes := mockTestData(mockDao, mockRouteBuilder)
	resultVirtualHosts, err := facadeVirtualHostBuilder.BuildVirtualHosts(initRouteConfig)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(resultVirtualHosts))
	for i, resultVirtualHost := range resultVirtualHosts {
		assert.Equal(t, initRouteConfig.VirtualHosts[i].Name, resultVirtualHost.Name)
		assert.Equal(t, virtualHostsDomain[i].Domain, resultVirtualHost.Domains[0])
		assert.Equal(t, []*eroute.Route{eRoutes[i]}, resultVirtualHost.Routes)
	}
	assert.Nil(t, resultVirtualHosts[0].TypedPerFilterConfig["envoy.filters.http.local_ratelimit"])
	assert.NotNil(t, resultVirtualHosts[1].TypedPerFilterConfig["envoy.filters.http.local_ratelimit"])
}

func mockTestData(mockDao *mock_dao.MockDao, mockRouteBuilder *mock_routeconfig.MockRouteBuilder) (*domain.RouteConfiguration, []*domain.VirtualHostDomain, []*eroute.Route) {
	initVirtualHosts := []*domain.VirtualHost{
		{Id: int32(1), Name: "vh1", RateLimitId: "missing-ratelimit", RequestHeadersToAdd: []domain.Header{{Name: "name1", Value: "value1"}}},
		{Id: int32(2), Name: "vh2", RateLimitId: "ratelimit", RequestHeadersToAdd: []domain.Header{{Name: "name2", Value: "value2"}}},
	}
	initRouteConfig := &domain.RouteConfiguration{Id: 1, Name: "test-rc", NodeGroupId: "test-gw", VirtualHosts: initVirtualHosts}

	virtualHostsDomain := []*domain.VirtualHostDomain{
		{Domain: "domain1", Version: int32(0)},
		{Domain: "domain2", Version: int32(0)},
	}
	mockDao.EXPECT().FindVirtualHostDomainByVirtualHostId(initVirtualHosts[0].Id).Return([]*domain.VirtualHostDomain{virtualHostsDomain[0]}, nil)
	mockDao.EXPECT().FindVirtualHostDomainByVirtualHostId(initVirtualHosts[1].Id).Return([]*domain.VirtualHostDomain{virtualHostsDomain[1]}, nil)

	routes := []*domain.Route{
		{Uuid: "d64a9674-96ae-4a1a-b168-9a55afe6d6c8"},
		{Uuid: "268ede47-c0ce-4d59-beba-d8275c558650"},
	}
	mockDao.EXPECT().FindRoutesByVirtualHostId(initVirtualHosts[0].Id).Return([]*domain.Route{routes[0]}, nil)
	mockDao.EXPECT().FindRoutesByVirtualHostId(initVirtualHosts[1].Id).Return([]*domain.Route{routes[1]}, nil)
	eRoutes := []*eroute.Route{
		{Name: routes[0].Uuid},
		{Name: routes[1].Uuid},
	}
	mockRouteBuilder.EXPECT().BuildRoutes([]*domain.Route{routes[0]}).Return([]*eroute.Route{eRoutes[0]}, nil, nil)
	mockRouteBuilder.EXPECT().BuildRoutes([]*domain.Route{routes[1]}).Return([]*eroute.Route{eRoutes[1]}, nil, nil)

	mockDao.EXPECT().FindRateLimitByNameWithHighestPriority("missing-ratelimit").Return(nil, nil)
	mockDao.EXPECT().FindRateLimitByNameWithHighestPriority("ratelimit").Return(&domain.RateLimit{Name: "ratelimit", LimitRequestsPerSecond: 10}, nil)

	return initRouteConfig, virtualHostsDomain, eRoutes
}

func TestGatewayVirtualHostBuilderBuildEgressVirtualHosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockRouteBuilder := getMockRouteBuilder(ctrl)
	mockProvider := getMockVersionAliasesProvider(ctrl)
	mockDao.EXPECT().FindListenersByNodeGroupId("test-gw").Times(2).Return([]*domain.Listener{{NodeGroupId: "test-gw"}}, nil)
	gatewayVirtualHostBuilder := NewEgressVirtualHostBuilder(mockDao, entity.NewService("v1"), mockRouteBuilder)

	initRouteConfig, virtualHostsDomain, eRoutes := mockTestData(mockDao, mockRouteBuilder)

	mockProvider.EXPECT().GetVersionAliases().Return("testAliases").AnyTimes()

	resultVirtualHosts, err := gatewayVirtualHostBuilder.BuildVirtualHosts(initRouteConfig)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(resultVirtualHosts))
	for i, resultVirtualHost := range resultVirtualHosts {
		assert.Equal(t, initRouteConfig.VirtualHosts[i].Name, resultVirtualHost.Name)
		assert.Equal(t, virtualHostsDomain[i].Domain, resultVirtualHost.Domains[0])
		assert.Equal(t, []*eroute.Route{eRoutes[i]}, resultVirtualHost.Routes)
		assert.Equal(t, initRouteConfig.VirtualHosts[i].RequestHeadersToAdd[0].Name, resultVirtualHost.RequestHeadersToAdd[0].Header.Key)
		assert.Equal(t, initRouteConfig.VirtualHosts[i].RequestHeadersToAdd[0].Value, resultVirtualHost.RequestHeadersToAdd[0].Header.Value)
		assert.Equal(t, "X-Token-Signature", resultVirtualHost.RequestHeadersToRemove[0])
		assert.Equal(t, "X-Forwarded-For", resultVirtualHost.ResponseHeadersToRemove[1])
		assert.Equal(t, "X-Forwarded-For", resultVirtualHost.RequestHeadersToRemove[1])
	}

	assert.Nil(t, resultVirtualHosts[0].TypedPerFilterConfig["envoy.filters.http.local_ratelimit"])
	assert.NotNil(t, resultVirtualHosts[1].TypedPerFilterConfig["envoy.filters.http.local_ratelimit"])
}