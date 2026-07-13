package v3

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/loadbalance"
	"github.com/stretchr/testify/assert"
)

const (
	testPostLoadBalance  = "/api/v3/control-plane/load-balance"
	testClusterName      = "test-cluster"
	testEndpoint         = "trace-service-1:8080"
	testEndpointTemplate = "trace-service-%s:8080"
	testHeaderName       = "BID"
	testNodeGroupName    = "test-nodegroup"
)

func TestLoadBalanceController_HandlePostLoadBalance(t *testing.T) {
	lbController, inMemDao := getLbController()
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, inMemDao, v1)
	prepareSingleCluster(t, testClusterName, inMemDao)
	rightVersionRequest := "{\n" +
		"    \"cluster\": \"" + testClusterName + "\",\n" +
		"    \"endpoint\": \"" + testEndpoint + "\",\n" +
		"    \"version\": \"v1\",\n" +
		"    \"policies\": [\n" +
		"        {\n" +
		"            \"header\": {\n" +
		"                \"headerName\": \"" + testHeaderName + "\"\n" +
		"            }\n" +
		"        }\n" +
		"    ]\n" +
		"}"

	response := SendHttpRequestWithBody(t, http.MethodPost, testPostLoadBalance, testPostLoadBalance,
		bytes.NewBufferString(rightVersionRequest), lbController.PostLBUnsecure)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	bodeBytes, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.Equal(t, "null", string(bodeBytes))
}

func TestLoadBalanceController_HandlePostLoadBalanceBadRequestVersionDoesNotExist(t *testing.T) {
	lbController, inMemDao := getLbController()
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, inMemDao, v1)
	prepareSingleCluster(t, testClusterName, inMemDao)
	rightVersionRequest := "{\n" +
		"    \"cluster\": \"" + testClusterName + "\",\n" +
		"    \"endpoint\": \"" + testEndpoint + "\",\n" +
		"    \"version\": \"v6\",\n" +
		"    \"policies\": [\n" +
		"        {\n" +
		"            \"header\": {\n" +
		"                \"headerName\": \"" + testHeaderName + "\"\n" +
		"            }\n" +
		"        }\n" +
		"    ]\n" +
		"}"

	response := SendHttpRequestWithBody(t, http.MethodPost, testPostLoadBalance, testPostLoadBalance,
		bytes.NewBufferString(rightVersionRequest), lbController.PostLBUnsecure)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	bodeBytes, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.NotEmpty(t, bodeBytes)
}

func TestLoadBalanceController_HandlePostLoadBalanceBadRequestWithoutEndpoint(t *testing.T) {
	lbController, inMemDao := getLbController()
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, inMemDao, v1)
	prepareSingleCluster(t, testClusterName, inMemDao)
	rightVersionRequest := "{\n" +
		"    \"cluster\": \"" + testClusterName + "\",\n" +
		"    \"version\": \"v1\",\n" +
		"    \"policies\": [\n" +
		"        {\n" +
		"            \"header\": {\n" +
		"                \"headerName\": \"" + testHeaderName + "\"\n" +
		"            }\n" +
		"        }\n" +
		"    ]\n" +
		"}"

	response := SendHttpRequestWithBody(t, http.MethodPost, testPostLoadBalance, testPostLoadBalance,
		bytes.NewBufferString(rightVersionRequest), lbController.PostLBUnsecure)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	bodeBytes, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.NotEmpty(t, bodeBytes)
}

func TestLoadBalanceController_HandlePostLoadBalanceRequestWithoutVersion(t *testing.T) {
	lbController, inMemDao := getLbController()
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, inMemDao, v1)
	prepareSingleCluster(t, testClusterName, inMemDao)
	rightVersionRequest := "{\n" +
		"    \"cluster\": \"" + testClusterName + "\",\n" +
		"    \"endpoint\": \"" + testEndpoint + "\",\n" +
		"    \"policies\": [\n" +
		"        {\n" +
		"            \"header\": {\n" +
		"                \"headerName\": \"" + testHeaderName + "\"\n" +
		"            }\n" +
		"        }\n" +
		"    ]\n" +
		"}"

	response := SendHttpRequestWithBody(t, http.MethodPost, testPostLoadBalance, testPostLoadBalance,
		bytes.NewBufferString(rightVersionRequest), lbController.PostLBUnsecure)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	bytesBody, _ := io.ReadAll(response.Body)
	assert.Equal(t, "null", string(bytesBody))
}

func TestLoadBalanceController_HandlePostLoadBalanceRequestWithEmptyVersion(t *testing.T) {
	lbController, inMemDao := getLbController()
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, inMemDao, v1)
	prepareSingleCluster(t, testClusterName, inMemDao)
	rightVersionRequest := "{\n" +
		"    \"cluster\": \"" + testClusterName + "\",\n" +
		"    \"endpoint\": \"" + testEndpoint + "\",\n" +
		"    \"version\": \"\",\n" +
		"    \"policies\": [\n" +
		"        {\n" +
		"            \"header\": {\n" +
		"                \"headerName\": \"" + testHeaderName + "\"\n" +
		"            }\n" +
		"        }\n" +
		"    ]\n" +
		"}"

	response := SendHttpRequestWithBody(t, http.MethodPost, testPostLoadBalance, testPostLoadBalance,
		bytes.NewBufferString(rightVersionRequest), lbController.PostLBUnsecure)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	bytesBody, _ := io.ReadAll(response.Body)
	assert.Equal(t, "null", string(bytesBody))
}

func TestLoadBalanceController_HandlePostLoadBalanceBadRequestWithoutPolicies(t *testing.T) {
	lbController, inMemDao := getLbController()
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, inMemDao, v1)
	prepareSingleCluster(t, testClusterName, inMemDao)
	rightVersionRequest := "{\n" +
		"    \"cluster\": \"" + testClusterName + "\",\n" +
		"    \"endpoint\": \"" + testEndpoint + "\",\n" +
		"    \"version\": \"v1\"\n" +
		"}"

	response := SendHttpRequestWithBody(t, http.MethodPost, testPostLoadBalance, testPostLoadBalance,
		bytes.NewBufferString(rightVersionRequest), lbController.PostLBUnsecure)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	bodeBytes, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.NotEmpty(t, bodeBytes)
}

func TestLoadBalanceController_HandlePostLoadBalanceBadRequestWithEmptyPolicies(t *testing.T) {
	lbController, inMemDao := getLbController()
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, inMemDao, v1)
	prepareSingleCluster(t, testClusterName, inMemDao)
	rightVersionRequest := "{\n" +
		"    \"cluster\": \"" + testClusterName + "\",\n" +
		"    \"endpoint\": \"" + testEndpoint + "\",\n" +
		"    \"version\": \"v1\",\n" +
		"    \"policies\": []\n" +
		"}"

	response := SendHttpRequestWithBody(t, http.MethodPost, testPostLoadBalance, testPostLoadBalance,
		bytes.NewBufferString(rightVersionRequest), lbController.PostLBUnsecure)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	bodeBytes, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.Equal(t, "null", string(bodeBytes))
}

func prepareSingleCluster(t *testing.T, clusterName string, memDao *dao.InMemDao) {
	_, err := memDao.WithWTx(func(dao dao.Repository) error {
		prepareClusterWithEndpoints(t, clusterName, dao)
		return nil
	})
	assert.Nil(t, err)
}

func prepareClusterWithEndpoints(t *testing.T, clusterName string, dao dao.Repository) int32 {
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	v2 := domain.NewDeploymentVersion("v2", domain.CandidateStage)
	assert.Nil(t, dao.SaveDeploymentVersion(v1))
	assert.Nil(t, dao.SaveDeploymentVersion(v2))
	cluster := domain.NewCluster(fmt.Sprintf("%s||%s||%s", clusterName, clusterName, "8080"), false)
	assert.Nil(t, dao.SaveCluster(cluster))
	nodeGroup := &domain.NodeGroup{Name: testNodeGroupName}
	assert.Nil(t, dao.SaveNodeGroup(nodeGroup))
	clusterNodeGroup := domain.NewClusterNodeGroups(cluster.Id, nodeGroup.Name)
	assert.Nil(t, dao.SaveClustersNodeGroup(clusterNodeGroup))
	endpoint1 := domain.NewEndpoint(fmt.Sprintf(testEndpointTemplate, "1"), 8080, "v1", "v1", cluster.Id)
	assert.Nil(t, dao.SaveEndpoint(endpoint1))
	return endpoint1.Id
}

func (c *LoadBalanceController) PostLBUnsecure(ctx fiber.Ctx) error {
	return c.HandlePostLoadBalance(ctx)
}

func getLbController() (*LoadBalanceController, *dao.InMemDao) {
	inMemStorage := ram.NewStorage()
	internalBus := bus.GetInternalBusInstance()
	eventBus := bus.NewEventBusAggregator(inMemStorage, internalBus, internalBus, nil, nil)
	genericDao := dao.NewInMemDao(inMemStorage, &idGeneratorMock{}, []func([]memdb.Change) error{flushChanges})
	entityService := entity.NewService("v1")
	lbService := loadbalance.NewLoadBalanceService(genericDao, entityService, eventBus)
	lbRequestValidator := dto.NewLBRequestValidator(genericDao)
	lbController := NewLoadBalanceController(lbService, lbRequestValidator)
	return lbController, genericDao
}

func TestLoadBalanceController_buildDomainHashPolicyFromDto(t *testing.T) {
	policy := dto.HashPolicy{Cookie: &dto.Cookie{}}
	loadBalanceController := LoadBalanceController{}
	result := loadBalanceController.buildDomainHashPolicyFromDto(&policy)
	assert.False(t, result.CookieTTL.Valid)

	ttl := int64(0)
	policy = dto.HashPolicy{Cookie: &dto.Cookie{Ttl: &ttl}}
	loadBalanceController = LoadBalanceController{}
	result = loadBalanceController.buildDomainHashPolicyFromDto(&policy)
	assert.True(t, result.CookieTTL.Valid)
	assert.Equal(t, int64(0), result.CookieTTL.Int64)

	ttl = int64(10)
	policy = dto.HashPolicy{Cookie: &dto.Cookie{Ttl: &ttl}}
	loadBalanceController = LoadBalanceController{}
	result = loadBalanceController.buildDomainHashPolicyFromDto(&policy)
	assert.True(t, result.CookieTTL.Valid)
	assert.Equal(t, int64(10), result.CookieTTL.Int64)
}
