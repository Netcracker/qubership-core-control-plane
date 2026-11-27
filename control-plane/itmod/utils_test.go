package itmod

import (
	"github.com/hashicorp/go-memdb"
	cpCfg "github.com/netcracker/qubership-core-control-plane/control-plane/v2/configuration"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	v1 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v1"
	v2 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v2"
	v3 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/bluegreen"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/loadbalance"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/factory"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/registration"
	v12 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v1"
	v22 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v2"
	srv3 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	security2 "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type TestEnvironment struct {
	Dao *dao.InMemDao

	RouteControllerV1     *v1.Controller
	RouteControllerV2     *v2.RoutesController
	RouteControllerV3     *v3.RoutingConfigController
	BlueGreenControllerV2 *v2.BlueGreenController
}

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security2.DummyFiberServerSecurityMiddleware{})
	os.Exit(m.Run())
}

func InitCPTestEnvironment() TestEnvironment {
	os.Setenv("policy.file.name", "./testdata/test-policies.conf")
	defer os.Unsetenv("policy.file.name")
	configloader.Init(configloader.EnvPropertySource())
	defaultVersion := "v1"
	entityService := entity.NewService(defaultVersion)
	inMemStorage := ram.NewStorage()
	genericDao := dao.NewInMemDao(inMemStorage, &generatorMock{}, []func([]memdb.Change) error{})
	commonConfiguration := cpCfg.NewCommonConfiguration(genericDao, entityService, false)
	_, err := genericDao.WithWTx(func(dao dao.Repository) error {
		err := dao.SaveNodeGroup(&domain.NodeGroup{Name: "private-gateway-service"})
		if err != nil {
			return err
		}
		err = dao.SaveNodeGroup(&domain.NodeGroup{Name: "public-gateway-service"})
		if err != nil {
			return err
		}
		err = dao.SaveNodeGroup(&domain.NodeGroup{Name: "internal-gateway-service"})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	commonConfiguration.CreateCommonConfiguration()
	internalBus := bus.GetInternalBusInstance()
	eventBus := bus.NewEventBusAggregator(genericDao, internalBus, internalBus, nil, nil)
	routingModeService := routingmode.NewService(genericDao, defaultVersion)
	routeComponentsFactory := factory.NewComponentsFactory(entityService)
	registrationService := route.NewRegistrationService(routeComponentsFactory, entityService, genericDao, eventBus, routingModeService)
	v2RouteService := v22.NewV2Service(routeComponentsFactory, entityService, genericDao, eventBus, routingModeService, registrationService)
	v2RouteController := v2.NewRoutesController(v2RouteService, dto.RoutingV2RequestValidator{})
	v3RequestProcessor := registration.NewV3RequestProcessor(genericDao)
	v3RouteService := srv3.NewV3Service(genericDao, eventBus, routingModeService, registrationService, entityService, v3RequestProcessor)
	v3RouteController := v3.NewRoutingConfigController(v3RouteService, dto.RoutingV3RequestValidator{})
	v1RouteService := v12.NewV1Service(entityService, genericDao, eventBus, routingModeService, registrationService)
	v1RouteController := v1.NewController(v1RouteService, dto.RoutingV1RequestValidator{})
	loadBalanceService := loadbalance.NewLoadBalanceService(genericDao, entityService, eventBus)
	bgRegistry := bluegreen.NewVersionsRegistry(genericDao, entityService, eventBus)
	blueGreenService := bluegreen.NewService(entityService, loadBalanceService, genericDao, eventBus, bgRegistry)
	v2BlueGreenController := v2.NewBlueGreenController(blueGreenService, genericDao)

	return TestEnvironment{
		Dao:                   genericDao,
		RouteControllerV1:     v1RouteController,
		RouteControllerV2:     v2RouteController,
		RouteControllerV3:     v3RouteController,
		BlueGreenControllerV2: v2BlueGreenController,
	}
}

func makePromoteRequest(version string) *http.Request {
	reqUrl := &url.URL{
		Scheme: "http",
		Host:   "control-plane",
		Path:   "/promote/" + version,
	}
	req, _ := http.NewRequest(http.MethodPost, reqUrl.String(), strings.NewReader(""))
	return req
}

func deleteVersion(version string) *http.Request {
	reqUrl := &url.URL{
		Scheme: "http",
		Host:   "control-plane",
		Path:   "/versions/" + version,
	}
	req, _ := http.NewRequest(http.MethodDelete, reqUrl.String(), strings.NewReader(""))
	return req
}

func makeRollbackRequest() *http.Request {
	reqUrl := &url.URL{
		Scheme: "http",
		Host:   "control-plane",
		Path:   "/rollback",
	}
	req, _ := http.NewRequest(http.MethodPost, reqUrl.String(), strings.NewReader(""))
	return req
}

func makeV1Request(reqBody string) *http.Request {
	reqUrl := &url.URL{
		Scheme: "http",
		Host:   "control-plane",
		Path:   "/routes/v1/private-gateway-service",
	}
	req, _ := http.NewRequest(http.MethodPost, reqUrl.String(), strings.NewReader(reqBody))
	return req
}

type generatorMock struct {
	inc int32
}

func (g *generatorMock) Generate(uniqEntity domain.Unique) error {
	if uniqEntity.GetId() == 0 {
		g.inc++
		uniqEntity.SetId(g.inc)
	}
	return nil
}
func makeV2Request(reqBody string) *http.Request {
	reqUrl := &url.URL{
		Scheme: "http",
		Host:   "control-plane",
		Path:   "/routes/private-gateway-service",
	}
	req, _ := http.NewRequest(http.MethodPost, reqUrl.String(), strings.NewReader(reqBody))
	return req
}

func makeV2RequestWithNodeGroup(nodeGroup, reqBody string) *http.Request {
	reqUrl := &url.URL{
		Scheme: "http",
		Host:   "control-plane",
		Path:   "/routes/" + nodeGroup,
	}
	req, _ := http.NewRequest(http.MethodPost, reqUrl.String(), strings.NewReader(reqBody))
	return req
}

func getRoutesWithNodeGroupAndVirtualServiceName(nodeGroup, virtualServiceName string) *http.Request {
	reqUrl := &url.URL{
		Scheme: "http",
		Host:   "control-plane",
		Path:   "/routes/" + nodeGroup + "/" + virtualServiceName,
	}
	req, _ := http.NewRequest(http.MethodGet, reqUrl.String(), strings.NewReader(""))
	return req
}
