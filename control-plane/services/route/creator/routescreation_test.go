package creator

// TODO rewrite test and check methods:
// CreateRoute, ConfigureAllow, ConfigureProhibit

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

var (
	expectedRegexpRouteKey     = ""
	expectedRouteConfiguration *domain.RouteConfiguration
	expectedVirtualHost        *domain.VirtualHost
	expectedVirtualHostDomain  *domain.VirtualHostDomain
	expectedListener           *domain.Listener
	nodeGroups                 = []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}
	deploymentVersion = domain.NewDeploymentVersion("v1", domain.ActiveStage)
	testRoutes        = map[string]string{
		"/api/v1/config-server/**":     "http://config-server:8080",
		"/api/v1/identity-provider/**": "http://identity-provider:8080",
		"/auth/**":                     "http://identity-provider:8080",
		"/asd/{some}/something/**":     "http://identity-provider:8080",
	}
	testGeneratedClusterKeys = make(map[string]string)
	testGeneratedRouteKeys   = make(map[string]string)
	yamlConfig               = `
blue-green:
  versions:
    default-version: v1
`
)

func init() {
	fd, _ := os.Create("application.yaml")
	defer fd.Close()
	fd.Write([]byte(yamlConfig))
}

func TestPrepareEnv(t *testing.T) {
	testable := getDao()
	saveAll(t, testable)
	selectedRouteConfigs, err := testable.FindAllRouteConfigs()
	assert.Nil(t, err)
	assert.NotNil(t, selectedRouteConfigs)
	assert.NotNil(t, selectedRouteConfigs[0])
	assert.ObjectsAreEqualValues(expectedRouteConfiguration, selectedRouteConfigs[0])

	selectedListeners, err := testable.FindAllListeners()
	assert.Nil(t, err)
	assert.NotNil(t, selectedListeners)
	assert.NotNil(t, selectedListeners[0])
	assert.ObjectsAreEqualValues(expectedListener, selectedListeners[0])

	selectedVirtualHosts, err := testable.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotNil(t, selectedVirtualHosts)
	assert.NotNil(t, selectedVirtualHosts[0])
	assert.ObjectsAreEqualValues(expectedVirtualHost, selectedVirtualHosts)

	selectedVirtualHostsDomain, err := testable.FindAllVirtualHostsDomain()
	assert.Nil(t, err)
	assert.NotNil(t, selectedVirtualHostsDomain)
	assert.NotNil(t, selectedVirtualHostsDomain[0])
	assert.ObjectsAreEqualValues(expectedVirtualHostDomain, selectedVirtualHostsDomain)
}

func TestGenerateKeys(t *testing.T) {
	for _, value := range testRoutes {
		msAddress := msaddr.NewMicroserviceAddress(value, msaddr.DefaultNamespace)
		clusterKey := clusterkey.DefaultClusterKeyGenerator.GenerateKey("", msAddress)
		assert.NotEmpty(t, clusterKey)
		testGeneratedClusterKeys[clusterKey] = "found"

		/*
			routeFromAddr := format.NewRouteFromAddress(key)
			routeKey := routekey.DefaultRouteKeyGenerator.GenerateKeyFromAddr(msaddr.NewNamespace(msaddr.DefaultNamespace),
				routeFromAddr, deploymentVersion.Version)
			assert.NotEmpty(t, routeKey)
			testGeneratedRouteKeys[routeKey] = "found"*/
	}
}

func TestGetGroups(t *testing.T) {
	testable := getDao()
	saveAll(t, testable)
	selectedNodeGroups, err := testable.FindAllNodeGroups()
	assert.Nil(t, err)
	assert.NotNil(t, selectedNodeGroups)
	assert.Equal(t, len(nodeGroups), len(selectedNodeGroups))
	assert.ObjectsAreEqualValues(nodeGroups, selectedNodeGroups)
}

func TestGetRoutes(t *testing.T) {
	testable := getDao()
	saveAll(t, testable)
	routes, err := testable.FindAllRoutes()
	assert.Nil(t, err)
	assert.NotNil(t, routes)
	// there are no route, that is why test are passed
	for _, route := range routes {
		assert.Equal(t, expectedVirtualHost.Id, route.VirtualHostId)
		assert.Equal(t, deploymentVersion.Version, route.DeploymentVersion)
		assert.Equal(t, deploymentVersion.Version, route.InitialDeploymentVersion)
		assert.Equal(t, "found", testGeneratedClusterKeys[route.ClusterName])
		assert.Equal(t, "found", testGeneratedRouteKeys[route.RouteKey])
		assert.False(t, strings.Contains(route.Prefix, "*"))
		assert.False(t, strings.Contains(route.PrefixRewrite, "*"))
	}
}

func TestRouteWithRegexp(t *testing.T) {
	testable := getDao()
	saveAll(t, testable)
	routes, err := testable.FindRoutesByVirtualHostIdAndRouteKey(expectedVirtualHost.Id, expectedRegexpRouteKey)
	assert.Nil(t, err)
	assert.NotNil(t, routes)
	for _, route := range routes {
		logger.Infof("%s", route.RouteKey)
		assert.NotEmpty(t, route.Regexp)
		assert.Empty(t, route.Prefix)
		assert.True(t, strings.Contains(route.Regexp, "*"))
	}
}

func TestRouteEntry(t *testing.T) {
	idpRouteEntry := &RouteEntry{
		"/api/v1/identity-provider/**",
		"/",
		"default",
		0,
		0,
		[]*domain.HeaderMatcher{},
	}
	idpRoute := idpRouteEntry.CreateRoute(1, "/asd/{some}/something/**", "http://some-addr:8080", "", 0, 0, "v1", "v1", []*domain.HeaderMatcher{}, []domain.Header{}, []string{})
	assert.True(t, idpRouteEntry.IsValidRoute())
	assert.False(t, idpRouteEntry.IsProhibited())
	assert.Equal(t, "", idpRoute.Prefix)
	assert.Equal(t, "/asd/([^/]+)/something(/.*)?", idpRoute.Regexp)
	assert.Equal(t, "", idpRoute.Path)
	assert.Equal(t, "http://some-addr:8080", idpRoute.HostRewrite)
	allowedRoute := idpRouteEntry.ConfigureAllowedRoute(idpRoute)
	assert.ObjectsAreEqualValues(idpRoute, allowedRoute)
}

func TestRouteEntry_WithBrackets(t *testing.T) {
	prohibitedRouteEntry := &RouteEntry{
		"/cip-routes/orderManagement/v1/listDocument/#{microserviceName}?fields=[]",
		"/", "default", 0, 0, []*domain.HeaderMatcher{},
	}
	assert.False(t, prohibitedRouteEntry.IsValidRoute())
	assert.False(t, prohibitedRouteEntry.IsProhibited())

	prohibitedRouteEntry = &RouteEntry{
		"/cip-routes/orderManagement/v1/listDocument/brackets[abcde]",
		"/", "default", 0, 0, []*domain.HeaderMatcher{},
	}
	assert.False(t, prohibitedRouteEntry.IsValidRoute())
	assert.False(t, prohibitedRouteEntry.IsProhibited())
}

func saveAll(t *testing.T, testable *dao.InMemDao) {
	_, err := testable.WithWTx(func(dao dao.Repository) error {
		for _, nodeGroup := range nodeGroups {
			assert.Nil(t, dao.SaveNodeGroup(nodeGroup))
		}
		assert.Nil(t, dao.SaveDeploymentVersion(deploymentVersion))
		expectedRouteConfiguration = domain.NewRouteConfiguration(domain.PublicGateway+"-routes", domain.PublicGateway)
		assert.Nil(t, dao.SaveRouteConfig(expectedRouteConfiguration))
		expectedListener = domain.NewListener(domain.PublicGateway+"-listener", "::",
			"8080", domain.PublicGateway, domain.PublicGateway+"-routes")
		assert.Nil(t, dao.SaveListener(expectedListener))
		expectedVirtualHost = domain.NewVirtualHost(domain.PublicGateway, expectedRouteConfiguration.Id)
		assert.Nil(t, dao.SaveVirtualHost(expectedVirtualHost))
		expectedVirtualHostDomain = domain.NewVirtualHostDomain("*", expectedVirtualHost.Id)
		assert.Nil(t, dao.SaveVirtualHostDomain(expectedVirtualHostDomain))
		return nil
	})
	assert.Nil(t, err)
}

func TestCleanUpFile(t *testing.T) {
	assert.Nil(t, os.Remove("application.yaml"))
}

func getDao() *dao.InMemDao {
	return dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, nil)
}

type GeneratorMock struct {
	counter int32
}

func (g *GeneratorMock) Generate(uniqEntity domain.Unique) error {
	if uniqEntity.GetId() == 0 {
		uniqEntity.SetId(atomic.AddInt32(&g.counter, 1))
	}
	return nil
}
