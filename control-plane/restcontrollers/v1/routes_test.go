package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/factory"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/routekey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v1"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	security2 "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/security"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var (
	clusterPath            = "/routes/clusters"
	routeConfigsPath       = "/routes/route-configs"
	v1RouteRegPath         = "/api/v1/routes/:nodeGroup"
	v1RouteRegReq          = "/api/v1/routes/%s"
	inMemStorage           = ram.NewStorage()
	genericDao             = dao.NewInMemDao(inMemStorage, &idGeneratorMock{}, []func([]memdb.Change) error{flushChanges})
	internalBus            = bus.GetInternalBusInstance()
	eventBus               = bus.NewEventBusAggregator(genericDao, internalBus, internalBus, nil, nil)
	flusher                = constancy.Flusher{BatchTm: &batchTransactionManagerMock{}}
	entityService          = entity.NewService("v1")
	routeModeService       = routingmode.NewService(genericDao, "v1")
	routeComponentsFactory = factory.NewComponentsFactory(entityService)
	registrationService    = route.NewRegistrationService(routeComponentsFactory, entityService, genericDao, eventBus, routeModeService)
	//registrationSrv        = v2.NewV2Service(routeComponentsFactory, entityService, genericDao, eventBus, routeModeService, registrationService)
	v1RouteProcessor = v1.NewV1Service(entityService, genericDao, eventBus, routeModeService, registrationService)
	v1controller     = NewController(v1RouteProcessor, dto.RoutingV1RequestValidator{})
	expectedClusters = []*domain.Cluster{
		domain.NewCluster("testCluster1", false),
		domain.NewCluster("testCluster2", false),
		domain.NewCluster("testCluster3", false),
	}

	createdRouteUuids []string
)

func flushChanges(changes []memdb.Change) error {
	return flusher.Flush(changes)
}

type idGeneratorMock struct {
	seq int32
}

func (generator *idGeneratorMock) Generate(uniqEntity domain.Unique) error {
	if uniqEntity.GetId() == 0 {
		uniqEntity.SetId(atomic.AddInt32(&generator.seq, 1))
	}
	return nil
}

type batchTransactionManagerMock struct{}

func (tm *batchTransactionManagerMock) WithTxBatch(_ func(tx constancy.BatchStorage) error) error {
	return nil
}

func (tm *batchTransactionManagerMock) IsCurrentPodDefinedAsMaster() (bool, error) {
	return true, nil
}

func createDeploymentVersion(ver, stage string) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		return dao.SaveDeploymentVersion(&domain.DeploymentVersion{
			Version:     ver,
			Stage:       stage,
			CreatedWhen: time.Now(),
			UpdatedWhen: time.Now(),
		})
	})
}

func createRouteConfiguration() int32 {
	id, _, _ := genericDao.WithWTxVal(func(dao dao.Repository) (interface{}, error) {
		rconfig := &domain.RouteConfiguration{
			Id:          1,
			Name:        domain.PublicGateway + "-routes",
			NodeGroupId: domain.PublicGateway,
		}
		dao.SaveRouteConfig(rconfig)
		return rconfig.Id, nil
	})

	return id.(int32)
}

func clearRoutes() {
	genericDao.WithWTx(func(repo dao.Repository) error {
		for _, u := range createdRouteUuids {
			err := repo.DeleteRouteByUUID(u)
			if err != nil {
				panic("can not delete route")
			}
		}
		return nil
	})
}

func createNodeGroups() {
	genericDao.WithWTx(func(dao dao.Repository) error {
		dao.SaveNodeGroup(domain.NewNodeGroup(domain.PrivateGateway))
		dao.SaveNodeGroup(domain.NewNodeGroup(domain.PublicGateway))
		dao.SaveNodeGroup(domain.NewNodeGroup(domain.InternalGateway))
		return nil
	})
}

func createVirtualHost(rconfig int32) int32 {
	vhostId, _, _ := genericDao.WithWTxVal(func(dao dao.Repository) (interface{}, error) {
		vhost := &domain.VirtualHost{
			Name:                 "testVHost",
			Version:              0,
			RouteConfigurationId: rconfig,
		}
		dao.SaveVirtualHost(vhost)

		return vhost.Id, nil
	})

	return vhostId.(int32)
}

func saveExpectedClusters(t *testing.T) {
	_, err := genericDao.WithWTx(func(dao dao.Repository) error {
		for _, cluster := range expectedClusters {
			assert.Nil(t, dao.SaveCluster(cluster))
		}
		return nil
	})
	assert.Nil(t, err)
}

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security2.DummyFiberServerSecurityMiddleware{})

	createDeploymentVersion("v1", "ACTIVE")
	createDeploymentVersion("v2", "CANDIDATE")
	configloader.Init(configloader.EnvPropertySource())

	os.Exit(m.Run())
}

func TestV1RouteController_GetClustersPrepareCluster(t *testing.T) {
	defer clearRoutes()
	saveExpectedClusters(t)
	actualClusters, err := genericDao.FindAllClusters()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClusters)
	assert.ObjectsAreEqualValues(expectedClusters, actualClusters)
}

func TestV1RouteController_GetClusters(t *testing.T) {
	defer clearRoutes()
	response := sendHttpRequest(t, http.MethodGet, clusterPath, clusterPath,
		bytes.NewBufferString(""), v1controller.GetClustersUnsecure)
	bytesBody, _ := io.ReadAll(response.Body)
	var actualClusters []dto.ClusterResponse
	err := json.Unmarshal(bytesBody, &actualClusters)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClusters)
	assert.Equal(t, len(expectedClusters), len(actualClusters))
	assert.ObjectsAreEqualValues(expectedClusters, actualClusters)
}

func TestV1RouteController_GetRouteConfigs(t *testing.T) {
	defer clearRoutes()
	createRouteConfiguration()
	response := sendHttpRequest(t, http.MethodGet, routeConfigsPath, routeConfigsPath,
		bytes.NewBufferString(""), v1controller.GetRouteConfigsUnsecure)
	bytesBody, _ := io.ReadAll(response.Body)
	var actualRouteConfiguration []dto.RouteConfigurationResponse
	err := json.Unmarshal(bytesBody, &actualRouteConfiguration)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualRouteConfiguration)
	assert.Equal(t, 1, len(actualRouteConfiguration))
}

func TestV1RouteController_CreateRoutes(t *testing.T) {
	createNodeGroups()
	createRouteConfiguration()
	testNamespace := "some-namespace"
	msUrl := "http://dbaas-agent:8080"
	firstRoute := dto.RouteEntry{
		From:      "/api/v3/dbaas/postgresql/physical_databases",
		To:        "/api/v3/dbaas/postgresql/physical_databases",
		Type:      domain.ProfileInternal,
		Namespace: testNamespace,
	}
	secondRoute := dto.RouteEntry{
		From:      "/postgresql/physical_databases",
		To:        "/postgresql/physical_databases",
		Type:      domain.ProfileInternal,
		Namespace: testNamespace,
	}
	routeEntityRequest := dto.RouteEntityRequest{
		MicroserviceUrl: msUrl,
		Routes:          &[]dto.RouteEntry{firstRoute, secondRoute},
		Allowed:         true,
	}
	requestByte, err := json.Marshal(routeEntityRequest)
	os.Setenv(msaddr.CloudNamespace, testNamespace)
	assert.Nil(t, err)
	responseRecorder := sendHttpRequest(t, http.MethodPost, v1RouteRegPath,
		fmt.Sprintf(v1RouteRegReq, domain.PublicGateway), bytes.NewReader(requestByte), v1controller.CreateRoutesUnsecure)
	assert.NotNil(t, responseRecorder)
	assert.Equal(t, http.StatusCreated, responseRecorder.StatusCode)
	actualRoutes, err := genericDao.FindAllRoutes()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualRoutes)
	assert.Equal(t, 2, len(actualRoutes))
	vHosts, err := genericDao.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotEmpty(t, vHosts)
	assert.Equal(t, 1, len(vHosts))
	for _, actualRoute := range actualRoutes {
		assert.Equal(t, vHosts[0].Id, actualRoute.VirtualHostId)
		//routeFrom := actualRoute.Prefix
		//routeFromAddr := format.NewRouteFromAddress(routeFrom)
		routeKey := routekey.GenerateKey(*actualRoute)
		clusterKey := clusterkey.DefaultClusterKeyGenerator.GenerateKey("", msaddr.NewMicroserviceAddress(msUrl, testNamespace))
		assert.Equal(t, routeKey, actualRoute.RouteKey)
		assert.Equal(t, clusterKey, actualRoute.ClusterName)
		assert.False(t, strings.Contains(actualRoute.ClusterName, testNamespace))
		assert.False(t, strings.Contains(actualRoute.RouteKey, testNamespace))
	}
	actualClusters, err := genericDao.FindAllClusters()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClusters)
	assert.Equal(t, len(expectedClusters)+1, len(actualClusters))
	os.Unsetenv(msaddr.CloudNamespace)
}

func checkResponseCode(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Errorf("handler returned wrong status code: got %v want %v",
			actual, expected)
	}
}

func sendHttpRequest(t *testing.T, httpMethod, endpoint, reqUrl string, body io.Reader, f func(c *fiber.Ctx) error) *http.Response {
	app, err := fiberserver.New().Process()
	assert.Nil(t, err)
	app.Add(httpMethod, endpoint, f)
	req, err := http.NewRequest(httpMethod,
		reqUrl,
		body,
	)
	req.Host = "localhost"
	assert.Nil(t, err)
	resp, err := app.Test(req, -1)
	return resp
}

func (c *Controller) CreateRoutesUnsecure(ctx *fiber.Ctx) error {
	return c.HandlePostRoutesWithNodeGroup(ctx)
}

func (c *Controller) GetRouteConfigsUnsecure(ctx *fiber.Ctx) error {
	return c.HandleGetRouteConfigs(ctx)
}

func (c *Controller) GetClustersUnsecure(ctx *fiber.Ctx) error {
	return c.HandleGetClusters(ctx)
}
