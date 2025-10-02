package entity

import (
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/stretchr/testify/assert"
	"runtime/debug"
	"testing"
)

func TestService_PutVirtualHostWithoutUpdate(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVh1 := domain.NewVirtualHost("test-vh", 1)
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh1))
		return nil
	})
	assert.Nil(t, err)

	expectedVh2 := domain.NewVirtualHost("test-vh1", 2)
	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh2))
		return nil
	})
	assert.Nil(t, err)

	actualVh, err := inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVh)
	assert.Equal(t, 2, len(actualVh))
	assert.Contains(t, actualVh, expectedVh1, expectedVh2)
}

func TestService_PutVirtualHostWithUpdateWithVirtualHostDomain(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVh := domain.NewVirtualHost("test-vh", 1)
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh))
		return nil
	})
	assert.Nil(t, err)

	actualVh, err := inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVh)
	assert.Equal(t, 1, len(actualVh))
	assert.Contains(t, actualVh, expectedVh)

	expectedVhd := domain.NewVirtualHostDomain("*", expectedVh.Id)
	expectedVh.Domains = []*domain.VirtualHostDomain{expectedVhd}
	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh))
		return nil
	})
	assert.Nil(t, err)

	actualVh, err = inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVh)
	assert.Equal(t, 1, len(actualVh))
	assert.Contains(t, actualVh, expectedVh)

	actualVhd, err := inMemDao.FindVirtualHostDomainByVirtualHostId(expectedVh.Id)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVhd)
	assert.Equal(t, 1, len(actualVhd))
	assert.Contains(t, actualVhd, expectedVhd)
}

func TestService_PutVirtualHostWithNewVirtualHostUpdateWithoutVirtualHostDomain(t *testing.T) {
	entityService, inMemDao := getService(t)
	vhToSave := domain.NewVirtualHost("test-vh", 1)
	vhToSave.Domains = []*domain.VirtualHostDomain{{Domain: "*"}}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, dao.SaveVirtualHost(vhToSave))
		vhToSave.Domains[0].VirtualHostId = vhToSave.Id
		assert.Nil(t, dao.SaveVirtualHostDomain(vhToSave.Domains[0]))
		return nil
	})
	assert.Nil(t, err)

	actualVh, err := inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVh)
	assert.Equal(t, 1, len(actualVh))
	expectedVh := domain.NewVirtualHost("test-vh", 1)
	expectedVh.Id = actualVh[0].Id
	expectedVh.Version = actualVh[0].Version
	expectedVh.Domains = []*domain.VirtualHostDomain{{Domain: "*", VirtualHostId: actualVh[0].Id}}
	AssertDeepEqual(t, expectedVh, actualVh[0], domain.VirtualHostTable)

	vhToSave.Domains = []*domain.VirtualHostDomain{}
	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, vhToSave))
		return nil
	})
	assert.Nil(t, err)

	actualVh, err = inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVh)
	assert.Equal(t, 1, len(actualVh))
	AssertDeepEqual(t, expectedVh, actualVh[0], domain.VirtualHostTable)

	actualVhd, err := inMemDao.FindVirtualHostDomainByVirtualHostId(expectedVh.Id)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVhd)
	assert.Equal(t, 1, len(actualVhd))
	AssertDeepEqual(t, expectedVh.Domains[0], actualVhd[0], domain.VirtualHostDomainTable)
}

func TestService_PutVirtualHostWithUpdateVirtualHostDomain(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVh := domain.NewVirtualHost("test-vh", 1)
	vhd := &domain.VirtualHostDomain{Domain: "*"}
	expectedVh.Domains = []*domain.VirtualHostDomain{vhd}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh))
		return nil
	})
	assert.Nil(t, err)

	actualVh, err := inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVh)
	assert.Equal(t, 1, len(actualVh))
	assert.Contains(t, actualVh, expectedVh)

	expectedVhd := &domain.VirtualHostDomain{Domain: "*.qubership.org"}
	expectedVh.Domains = []*domain.VirtualHostDomain{expectedVhd}
	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh))
		return nil
	})
	assert.Nil(t, err)

	actualVh, err = inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVh)
	assert.Equal(t, 1, len(actualVh))
	assert.Contains(t, actualVh, expectedVh)

	actualVhd, err := inMemDao.FindVirtualHostDomainByVirtualHostId(expectedVh.Id)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVhd)
	assert.Equal(t, 2, len(actualVhd))
	assert.Contains(t, actualVhd, expectedVhd)
}

func TestService_PutVirtualHostWithUpdateWithoutVirtualHostDomain(t *testing.T) {
	entityService, inMemDao := getService(t)
	vhToSave := domain.NewVirtualHost("test-vh", 1)
	vhToSave.Domains = []*domain.VirtualHostDomain{{Domain: "*"}}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, vhToSave))
		return nil
	})
	assert.Nil(t, err)

	loadedVirtualHosts, err := inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, loadedVirtualHosts)
	assert.Equal(t, 1, len(loadedVirtualHosts))

	expectedVh := domain.NewVirtualHost("test-vh", 1)
	expectedVh.Id = vhToSave.Id
	expectedVh.Domains = []*domain.VirtualHostDomain{domain.NewVirtualHostDomain("*", vhToSave.Id)}
	actualVh := loadedVirtualHosts[0]
	actualVh, err = entityService.LoadVirtualHostRelations(inMemDao, actualVh)
	assert.Nil(t, err)
	AssertDeepEqual(t, expectedVh, actualVh, domain.VirtualHostTable)

	vhUpdateReq := domain.NewVirtualHost("test-vh", 1)
	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, vhUpdateReq))
		return nil
	})
	assert.Nil(t, err)

	loadedVirtualHosts, err = inMemDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, loadedVirtualHosts)
	assert.Equal(t, 1, len(loadedVirtualHosts))
	actualVh = loadedVirtualHosts[0]
	actualVh, err = entityService.LoadVirtualHostRelations(inMemDao, actualVh)
	assert.Nil(t, err)
	AssertDeepEqual(t, expectedVh, actualVh, domain.VirtualHostTable)

	actualVhd, err := inMemDao.FindVirtualHostDomainByVirtualHostId(expectedVh.Id)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVhd)
	assert.Equal(t, 1, len(actualVhd))
	assert.True(t, expectedVh.Domains[0].Equals(actualVhd[0]))
}

func TestService_SaveVirtualHostDomains(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVhd := &domain.VirtualHostDomain{Domain: "*", VirtualHostId: 1}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.SaveVirtualHostDomains(dao, []*domain.VirtualHostDomain{expectedVhd}, expectedVhd.VirtualHostId))
		return nil
	})
	assert.Nil(t, err)

	actualVhd, err := inMemDao.FindAllVirtualHostsDomain()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualVhd)
	assert.Equal(t, 1, len(actualVhd))
	assert.Contains(t, actualVhd, expectedVhd)
}

func TestService_DeleteVirtualHostDomains(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVhd := &domain.VirtualHostDomain{Domain: "*", VirtualHostId: 1}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.SaveVirtualHostDomains(dao, []*domain.VirtualHostDomain{expectedVhd}, expectedVhd.VirtualHostId))
		return nil
	})
	assert.Nil(t, err)

	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.DeleteVirtualHostDomains(dao, []*domain.VirtualHostDomain{expectedVhd}))
		return nil
	})
	assert.Nil(t, err)

	actualVhd, err := inMemDao.FindAllVirtualHostsDomain()
	assert.Nil(t, err)
	assert.Empty(t, actualVhd)
}

func TestService_FindVirtualHostsByRouteConfigWithoutData(t *testing.T) {
	entityService, inMemDao := getService(t)
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		vh, err := entityService.FindVirtualHostsByRouteConfig(dao, 1)
		assert.Nil(t, err)
		assert.Empty(t, vh)
		return nil
	})
	assert.Nil(t, err)
}

func TestService_FindVirtualHostsByRouteConfigWithVirtualHostDomainRelation(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVh := domain.NewVirtualHost("test-vh", 1)
	vhd := &domain.VirtualHostDomain{Domain: "*"}
	expectedVh.Domains = []*domain.VirtualHostDomain{vhd}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh))
		return nil
	})
	assert.Nil(t, err)

	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		vh, err := entityService.FindVirtualHostsByRouteConfig(dao, 1)
		assert.Nil(t, err)
		assert.NotEmpty(t, vh)
		assert.Contains(t, vh, expectedVh)
		return nil
	})
	assert.Nil(t, err)
}

func TestService_FindVirtualHostsByRouteConfigWithVirtualWithRouteRelation(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVh := domain.NewVirtualHost("test-vh", 1)
	vhd := &domain.VirtualHostDomain{Domain: "*"}
	expectedVh.Domains = []*domain.VirtualHostDomain{vhd}
	route := &domain.Route{Uuid: uuid.New().String(), RouteKey: "1", VirtualHostId: 1, ClusterName: "test-cluster", DeploymentVersion: "v1"}
	expectedVh.Routes = []*domain.Route{route}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh))
		route.VirtualHostId = expectedVh.Id
		assert.Nil(t, dao.SaveRoute(route))
		return nil
	})
	assert.Nil(t, err)

	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		vh, err := entityService.FindVirtualHostsByRouteConfig(dao, 1)
		assert.Nil(t, err)
		assert.NotEmpty(t, vh)
		assert.Contains(t, vh, expectedVh)
		return nil
	})
	assert.Nil(t, err)
}

func TestService_LoadVirtualHostRelations(t *testing.T) {
	entityService, inMemDao := getService(t)
	expectedVh := domain.NewVirtualHost("test-vh", 1)
	vhd := &domain.VirtualHostDomain{Domain: "*"}
	expectedVh.Domains = []*domain.VirtualHostDomain{vhd}
	route := &domain.Route{Uuid: uuid.New().String(), RouteKey: "1", VirtualHostId: 1, ClusterName: "test-cluster", DeploymentVersion: "v1"}
	expectedVh.Routes = []*domain.Route{route}
	_, err := inMemDao.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, entityService.PutVirtualHost(dao, expectedVh))
		route.VirtualHostId = expectedVh.Id
		assert.Nil(t, dao.SaveRoute(route))
		return nil
	})
	assert.Nil(t, err)

	_, err = inMemDao.WithWTx(func(dao dao.Repository) error {
		vhToLoad := domain.NewVirtualHost(expectedVh.Name, expectedVh.RouteConfigurationId)
		vhToLoad.Id = expectedVh.Id
		vh, err := entityService.LoadVirtualHostRelations(dao, vhToLoad)
		assert.Nil(t, err)
		assert.NotEmpty(t, vh)
		AssertDeepEqual(t, expectedVh, vh, domain.VirtualHostTable)
		return nil
	})
	assert.Nil(t, err)
}

func AssertDeepEqual(t *testing.T, expected, actual interface{}, tableName string) {
	equal := false
	switch tableName {
	case domain.VirtualHostTable:
		equal = VirtualHostsEqual(expected.(*domain.VirtualHost), actual.(*domain.VirtualHost))
		break
	case domain.VirtualHostDomainTable:
		equal = expected.(*domain.VirtualHostDomain).Equals(actual.(*domain.VirtualHostDomain))
		break
	case domain.RouteConfigurationTable:
		equal = RouteConfigsEqual(expected.(*domain.RouteConfiguration), actual.(*domain.RouteConfiguration))
		break
	case domain.RouteTable:
		equal = RoutesEqual(expected.(*domain.Route), actual.(*domain.Route))
		break
	case domain.ClusterTable:
		equal = ClustersEqual(expected.(*domain.Cluster), actual.(*domain.Cluster))
		break
	case domain.EndpointTable:
		equal = EndpointsEqual(expected.(*domain.Endpoint), actual.(*domain.Endpoint))
		break
	case domain.HashPolicyTable:
		equal = expected.(*domain.HashPolicy).Equals(actual.(*domain.HashPolicy))
		break
	default:
		debug.PrintStack()
		t.Fatalf("%s type is not supported by AssertDeepEqual method", tableName)
	}
	if !equal {
		debug.PrintStack()
		t.Fatalf("%ss are not equal!\n Expected: %v\n Actual: %v", tableName, expected, actual)
	}
}

func VirtualHostsEqual(expected, actual *domain.VirtualHost) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}
	if expected.Id != actual.Id ||
		expected.Name != actual.Name ||
		expected.Version != actual.Version ||
		expected.RouteConfigurationId != actual.RouteConfigurationId {
		return false
	}

	// compare routes
	if len(expected.Domains) != len(actual.Domains) {
		return false
	}
	for _, expectedRoute := range expected.Routes {
		presentsInBothLists := false
		for _, actualRoute := range actual.Routes {
			if RoutesEqual(expectedRoute, actualRoute) {
				presentsInBothLists = true
				break
			}
		}
		if !presentsInBothLists {
			return false
		}
	}
	for _, actualRoute := range actual.Routes {
		presentsInBothLists := false
		for _, expectedRoute := range expected.Routes {
			if RoutesEqual(expectedRoute, actualRoute) {
				presentsInBothLists = true
				break
			}
		}
		if !presentsInBothLists {
			return false
		}
	}

	// compare domains
	if len(expected.Domains) != len(actual.Domains) {
		return false
	}
	for _, expectedDomain := range expected.Domains {
		presentsInBothLists := false
		for _, actualDomain := range actual.Domains {
			if expectedDomain.Equals(actualDomain) {
				presentsInBothLists = true
				break
			}
		}
		if !presentsInBothLists {
			return false
		}
	}
	for _, actualDomain := range actual.Domains {
		presentsInBothLists := false
		for _, expectedDomain := range expected.Domains {
			if expectedDomain.Equals(actualDomain) {
				presentsInBothLists = true
				break
			}
		}
		if !presentsInBothLists {
			return false
		}
	}

	// compare RequestHeadersToAdd
	if len(expected.RequestHeadersToAdd) != len(actual.RequestHeadersToAdd) {
		return false
	}
	for _, expectedHeader := range expected.RequestHeadersToAdd {
		presentsInBothLists := false
		for _, actualHeader := range actual.RequestHeadersToAdd {
			if expectedHeader.Equals(actualHeader) {
				presentsInBothLists = true
				break
			}
		}
		if !presentsInBothLists {
			return false
		}
	}
	for _, actualHeader := range actual.RequestHeadersToAdd {
		presentsInBothLists := false
		for _, expectedHeader := range expected.RequestHeadersToAdd {
			if expectedHeader.Equals(actualHeader) {
				presentsInBothLists = true
				break
			}
		}
		if !presentsInBothLists {
			return false
		}
	}

	// compare RequestHeadersToRemove
	if len(expected.RequestHeadersToRemove) != len(actual.RequestHeadersToRemove) {
		return false
	}
	for _, expectedHeader := range expected.RequestHeadersToRemove {
		if !util.SliceContainsElement(actual.RequestHeadersToRemove, expectedHeader) {
			return false
		}
	}
	for _, actualHeader := range actual.RequestHeadersToRemove {
		if !util.SliceContainsElement(actual.RequestHeadersToRemove, actualHeader) {
			return false
		}
	}
	return true
}
