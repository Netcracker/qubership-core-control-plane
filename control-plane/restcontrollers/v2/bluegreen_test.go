package v2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/bluegreen"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/loadbalance"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

const (
	versionPath           = "/control-plane/versions"
	deleteVersionPath     = "/control-plane/versions/:version"
	reqDeleteVersionPath  = "/control-plane/versions/%s"
	promoteVersionPath    = "/control-plane/promote/:version"
	reqPromoteVersionPath = "/control-plane/promote/%s"
	rollbackVersionPath   = "/control-plane/rollback"
	expectedVersion       = "v6"
	notFoundVersion       = "v4012"
)

var (
	expectedNodeGroups = []domain.NodeGroup{
		{Name: domain.PublicGateway},
		{Name: domain.PrivateGateway},
		{Name: domain.InternalGateway},
	}
	expectedRouteConfigs = []domain.RouteConfiguration{
		{
			Id:          1,
			Name:        domain.PublicGateway + "-routes",
			NodeGroupId: domain.PublicGateway,
		},
		{
			Id:          2,
			Name:        domain.PrivateGateway + "-routes",
			NodeGroupId: domain.PrivateGateway,
		},
		{
			Id:          3,
			Name:        domain.InternalGateway + "-routes",
			NodeGroupId: domain.InternalGateway,
		},
	}
	expectedClusters = []domain.Cluster{
		{
			Id:   1,
			Name: "testCluster1",
		},
		{
			Id:   2,
			Name: "testCluster2",
		},
		{
			Id:   3,
			Name: "testCluster3",
		},
		{
			Id:   4,
			Name: "testCluster4",
		},
		{
			Id:   5,
			Name: "ext-authz",
		},
	}
	expectedDeploymentVersions = []domain.DeploymentVersion{
		{Version: "v1", Stage: domain.ArchivedStage},
		{Version: "v2", Stage: domain.ArchivedStage},
		{Version: "v3", Stage: domain.ArchivedStage},
		{Version: "v4", Stage: domain.ActiveStage},
		{Version: "v5", Stage: domain.CandidateStage},
		{Version: "v6", Stage: domain.CandidateStage},
	}
	expectedEndpoints = []domain.Endpoint{
		{
			Id:                1,
			Address:           "http://address1",
			Port:              8080,
			DeploymentVersion: "v2",
			ClusterId:         expectedClusters[0].Id,
		},
		{
			Id:                2,
			Address:           "http://address1",
			Port:              8080,
			DeploymentVersion: "v3",
			ClusterId:         expectedClusters[1].Id,
		},
		{
			Id:                3,
			Address:           "http://address2",
			Port:              8080,
			DeploymentVersion: "v4",
			ClusterId:         expectedClusters[1].Id,
		},
		{
			Id:                4,
			Address:           "http://address3",
			Port:              8080,
			DeploymentVersion: "v5",
			ClusterId:         expectedClusters[1].Id,
		},
		{
			Id:                5,
			Address:           "http://address4",
			Port:              8080,
			DeploymentVersion: "v5",
			ClusterId:         expectedClusters[2].Id,
		},
		{
			Id:                6,
			Address:           "http://address4",
			Port:              8080,
			DeploymentVersion: "v6",
			ClusterId:         expectedClusters[2].Id,
		},
		{
			Id:                7,
			Address:           "http://address5",
			Port:              8080,
			DeploymentVersion: "v6",
			ClusterId:         expectedClusters[3].Id,
		},
		{
			Id:                8,
			Address:           "http://address6",
			Port:              8080,
			DeploymentVersion: "v6",
			ClusterId:         expectedClusters[3].Id,
		},
		{
			Id:                9,
			Address:           "http://address6",
			Port:              8080,
			DeploymentVersion: "v6",
			ClusterId:         expectedClusters[1].Id,
		},
		{
			Id:                10,
			Address:           "http://address6",
			Port:              8080,
			DeploymentVersion: "v4",
			ClusterId:         expectedClusters[4].Id,
		},
	}
	expectedVirtualHosts = []domain.VirtualHost{
		{
			Id:                   1,
			Name:                 domain.PublicGateway,
			RouteConfigurationId: expectedRouteConfigs[0].Id,
		},
		{
			Id:                   2,
			Name:                 domain.PrivateGateway,
			RouteConfigurationId: expectedRouteConfigs[1].Id,
		},
		{
			Id:                   3,
			Name:                 domain.InternalGateway,
			RouteConfigurationId: expectedRouteConfigs[2].Id,
		},
	}
	uuid1, _       = uuid.GenerateUUID()
	uuid2, _       = uuid.GenerateUUID()
	uuid3, _       = uuid.GenerateUUID()
	uuid4, _       = uuid.GenerateUUID()
	uuid5, _       = uuid.GenerateUUID()
	uuid6, _       = uuid.GenerateUUID()
	uuid7, _       = uuid.GenerateUUID()
	uuid8, _       = uuid.GenerateUUID()
	uuid9, _       = uuid.GenerateUUID()
	expectedRoutes = []domain.Route{
		{
			Id:                       1,
			Uuid:                     uuid1,
			VirtualHostId:            3,
			RouteKey:                 "||/api/v2/test||v2",
			Prefix:                   "/api/v2/test",
			PrefixRewrite:            "/api/v2/test",
			DeploymentVersion:        "v2",
			InitialDeploymentVersion: "v2",
			ClusterName:              expectedClusters[0].Name,
		},
		{
			Id:                       2,
			Uuid:                     uuid2,
			VirtualHostId:            1,
			RouteKey:                 "||/api/v3/test||v3",
			Prefix:                   "/api/v3/test",
			PrefixRewrite:            "/api/v3/test",
			DeploymentVersion:        "v3",
			InitialDeploymentVersion: "v3",
			ClusterName:              expectedClusters[1].Name,
		},
		{
			Id:                       3,
			Uuid:                     uuid3,
			VirtualHostId:            2,
			RouteKey:                 "||/api/v5/test||v4",
			Prefix:                   "/api/v5/test",
			PrefixRewrite:            "/api/v5/test",
			DeploymentVersion:        "v4",
			InitialDeploymentVersion: "v4",
			ClusterName:              expectedClusters[2].Name,
		},
		{
			Id:                       4,
			Uuid:                     uuid4,
			VirtualHostId:            1,
			RouteKey:                 "||/api/v2/asd||v4",
			Prefix:                   "/api/v2/asd",
			PrefixRewrite:            "/api/v2/asd",
			DeploymentVersion:        "v4",
			InitialDeploymentVersion: "v4",
			ClusterName:              expectedClusters[1].Name,
		},
		{
			Id:                       5,
			Uuid:                     uuid5,
			VirtualHostId:            1,
			RouteKey:                 "||/api/v1/test||v5",
			Prefix:                   "/api/v1/test",
			PrefixRewrite:            "/api/v1/test",
			DeploymentVersion:        "v5",
			InitialDeploymentVersion: "v5",
			ClusterName:              expectedClusters[2].Name,
		},
		{
			Id:                       6,
			Uuid:                     uuid6,
			VirtualHostId:            1,
			RouteKey:                 "||/api/v5/asd||v6",
			Prefix:                   "/api/v5/asd",
			PrefixRewrite:            "/api/v5/asd",
			DeploymentVersion:        "v6",
			InitialDeploymentVersion: "v6",
			ClusterName:              expectedClusters[1].Name,
		},
		{
			Id:                       7,
			Uuid:                     uuid7,
			VirtualHostId:            2,
			RouteKey:                 "||/api/v2/test||v6",
			Prefix:                   "/api/v2/test",
			PrefixRewrite:            "/api/v2/test/test",
			DeploymentVersion:        "v6",
			InitialDeploymentVersion: "v6",
			ClusterName:              expectedClusters[3].Name,
		},
		{
			Id:                       8,
			Uuid:                     uuid8,
			VirtualHostId:            2,
			RouteKey:                 "||/api/v5/test||v6",
			Prefix:                   "/api/v5/test",
			PrefixRewrite:            "/api/v5/test",
			DeploymentVersion:        "v6",
			InitialDeploymentVersion: "v6",
			ClusterName:              expectedClusters[2].Name,
		},
		{
			Id:                       9,
			Uuid:                     uuid9,
			VirtualHostId:            2,
			RouteKey:                 "||/api/v2/control-plane/routing/details||v6",
			Prefix:                   "/api/v2/control-plane/routing/details",
			PrefixRewrite:            "/api/v2/control-plane/routing/details",
			DeploymentVersion:        "v6",
			InitialDeploymentVersion: "v6",
			ClusterName:              expectedClusters[1].Name,
		},
	}
)

func TestV2Controller_GetAllDeploymentVersions(t *testing.T) {
	_, v2controller := prepare()
	responseRecorder := sendHttpRequest(t, http.MethodGet, versionPath, versionPath, v2controller.GetAllDeploymentVersionsUnsecure)
	assert.Equal(t, http.StatusOK, responseRecorder.StatusCode)
	defer responseRecorder.Body.Close()
	bodyBytes, _ := io.ReadAll(responseRecorder.Body)
	var actualDeploymentVersions []domain.DeploymentVersion
	err := json.Unmarshal(bodyBytes, &actualDeploymentVersions)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualDeploymentVersions)
	assert.Equal(t, len(expectedDeploymentVersions), len(actualDeploymentVersions))
}

func TestV2Controller_DeleteDeploymentVersion(t *testing.T) {
	genericDao, v2controller := prepare()
	response := sendHttpRequest(t, http.MethodDelete, deleteVersionPath,
		fmt.Sprintf(reqDeleteVersionPath, expectedVersion), v2controller.DeleteDeploymentVersionUnsecure)
	assert.NotNil(t, response)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	actualNodeGroups, err := genericDao.FindAllNodeGroups()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualNodeGroups)
	assert.Equal(t, len(expectedNodeGroups), len(actualNodeGroups))
	actualDeploymentVersions, err := genericDao.FindAllDeploymentVersions()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualDeploymentVersions)
	assert.Equal(t, len(expectedDeploymentVersions)-1, len(actualDeploymentVersions))
	actualEndpoints, err := genericDao.FindAllEndpoints()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualEndpoints)
	assert.Equal(t, len(expectedEndpoints)-4, len(actualEndpoints))
	actualClusters, err := genericDao.FindAllClusters()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClusters)
	assert.Equal(t, len(expectedClusters)-1, len(actualClusters))
	findDeletedCluster, err := genericDao.FindClusterByName("testCluster4")
	assert.Nil(t, err)
	assert.Empty(t, findDeletedCluster)
	actualRoutes, err := genericDao.FindAllRoutes()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualRoutes)
	assert.Equal(t, len(expectedRoutes)-4, len(actualRoutes))
	actualRoutesByV6, err := genericDao.FindRoutesByDeploymentVersion(expectedVersion)
	assert.Nil(t, err)
	assert.Empty(t, actualRoutesByV6)
}

func TestV2Controller_DeleteDeploymentVersion_NotFound(t *testing.T) {
	_, v2controller := prepare()
	response := sendHttpRequest(t, http.MethodDelete, deleteVersionPath,
		fmt.Sprintf(reqDeleteVersionPath, expectedVersion), v2controller.DeleteDeploymentVersionUnsecure)
	assert.NotNil(t, response)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	responseNotFound := sendHttpRequest(t, http.MethodDelete, deleteVersionPath,
		fmt.Sprintf(reqDeleteVersionPath, expectedVersion), v2controller.DeleteDeploymentVersionUnsecure)
	assert.NotNil(t, responseNotFound)
	assert.Equal(t, http.StatusNotFound, responseNotFound.StatusCode)
	defer responseNotFound.Body.Close()
}

func TestV2Controller_DeleteDeploymentVersion_Active(t *testing.T) {
	_, v2controller := prepare()
	response := sendHttpRequest(t, http.MethodDelete, deleteVersionPath,
		fmt.Sprintf(reqDeleteVersionPath, "v4"), v2controller.DeleteDeploymentVersionUnsecure)
	assert.NotNil(t, response)
	defer response.Body.Close()
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
}

/* TODO Rewrite
func TestV2Controller_Promote(t *testing.T) {
	genericDao, v2controller := prepare()
	responseRecorder := sendHttpRequest(t, http.MethodPost, promoteVersionPath,
		fmt.Sprintf(reqPromoteVersionPath, expectedVersion), v2controller.PromoteHandlerUnsecure)
	assert.NotNil(t, responseRecorder)
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	var actualDeploymentVersions []*domain.DeploymentVersion
	err := json.Unmarshal([]byte(responseRecorder.Body.String()), &actualDeploymentVersions)
	assert.Nil(t, err)

	//deployments
	foundDeploymentVersion, err := genericDao.FindAllDeploymentVersions()
	assert.Nil(t, err)
	assert.NotEmpty(t, foundDeploymentVersion)
	assert.Equal(t, len(foundDeploymentVersion), len(actualDeploymentVersions))
	assert.Equal(t, 3, len(actualDeploymentVersions))
	assert.True(t, reflect.DeepEqual(actualDeploymentVersions, foundDeploymentVersion))

	dVersions := make(map[string]int)
	for _, dVersion := range foundDeploymentVersion {
		assert.NotEqual(t, domain.CandidateStage, dVersion.Stage)
		assert.NotEqual(t, "v1", dVersion.Version)
		assert.NotEqual(t, "v2", dVersion.Version)
		assert.NotEqual(t, "v5", dVersion.Version)
		dVersions[dVersion.Version] += 1
		switch dVersion.Version {
		case "v3":
			assert.Equal(t, domain.ArchivedStage, dVersion.Stage)
			break
		case "v4":
			assert.Equal(t, domain.LegacyStage, dVersion.Stage)
			break
		case "v6":
			assert.Equal(t, domain.ActiveStage, dVersion.Stage)
			break
		}
	}
	assert.Equal(t, 3, len(dVersions))
	assert.Equal(t, 1, dVersions["v3"])
	assert.Equal(t, 1, dVersions["v4"])
	assert.Equal(t, 1, dVersions["v6"])

	//routes
	actualRoutes, err := v2controller.dao.FindAllRoutes()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualRoutes)
	assert.Equal(t, len(expectedRoutes)-2, len(actualRoutes))
	routeResponse404 := 0
	routesByVersion := make(map[string]int)
	for _, actualRoute := range actualRoutes {
		assert.NotEqual(t, "v2", actualRoute.DeploymentVersion)
		assert.NotEqual(t, "v5", actualRoute.DeploymentVersion)
		routesByVersion[actualRoute.DeploymentVersion] += 1
		if actualRoute.DirectResponseCode == 404 {
			routeResponse404++
		}
	}
	assert.Equal(t, 1, routesByVersion["v3"])
	assert.Equal(t, 4, routesByVersion["v4"])
	assert.Equal(t, 2, routesByVersion["v6"])
	assert.Equal(t, 3, routeResponse404)

	//endpoints
	actualEndpoints, err := genericDao.FindAllEndpoints()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualEndpoints)
	assert.Equal(t, len(expectedEndpoints)-3, len(actualEndpoints))
	endpointsByCluster := make(map[int32]int)
	endpointsByVersion := make(map[string]int)
	for _, actualEndpoint := range actualEndpoints {
		assert.NotEqual(t, "v2", actualEndpoint.DeploymentVersion)
		assert.NotEqual(t, "v5", actualEndpoint.DeploymentVersion)
		assert.NotEmpty(t, actualEndpoint.ClusterId)
		assert.NotEmpty(t, actualEndpoint.DeploymentVersion)
		endpointsByCluster[actualEndpoint.ClusterId] += 1
		endpointsByVersion[actualEndpoint.DeploymentVersion] += 1
	}
	assert.Equal(t, 0, endpointsByCluster[1])
	assert.Equal(t, 3, endpointsByCluster[2])
	assert.Equal(t, 1, endpointsByCluster[3])
	assert.Equal(t, 2, endpointsByCluster[4])
	assert.Equal(t, 1, endpointsByCluster[5])
	assert.Equal(t, 1, endpointsByVersion["v3"])
	assert.Equal(t, 0, endpointsByVersion["v4"])
	assert.Equal(t, 6, endpointsByVersion["v6"])

	//check clusters
	actualClusters, err := genericDao.FindAllClusters()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClusters)
	assert.Equal(t, len(expectedClusters)-1, len(actualClusters))
	for _, actualCluster := range actualClusters {
		assert.NotEqual(t, expectedClusters[0].Name, actualCluster.Name)
	}

	actualClustersNodeGroups, err := genericDao.FindAllClusterWithNodeGroup()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClustersNodeGroups)
	assert.Equal(t, 12, len(actualClustersNodeGroups))
	for _, actualClusterNodeGroup := range actualClustersNodeGroups {
		assert.NotEqual(t, expectedClusters[0].Id, actualClusterNodeGroup.ClustersId)
	}

}
*/

func TestV2Controller_Promote_NotFound(t *testing.T) {
	_, v2controller := prepare()
	response := sendHttpRequest(t, http.MethodPost, promoteVersionPath,
		fmt.Sprintf(reqPromoteVersionPath, notFoundVersion), v2controller.PromoteHandlerUnsecure)
	defer response.Body.Close()
	assert.NotNil(t, response)
	assert.Equal(t, http.StatusNotFound, response.StatusCode)
}

func TestV2Controller_Promote(t *testing.T) {
	tmpExpectedDeploymentVersions := expectedDeploymentVersions
	defer func() {
		expectedDeploymentVersions = tmpExpectedDeploymentVersions
	}()
	expectedDeploymentVersions = []domain.DeploymentVersion{
		{Version: "v1", Stage: domain.ArchivedStage},
		{Version: "v2", Stage: domain.ArchivedStage},
		{Version: "v3", Stage: domain.LegacyStage},
		{Version: "v4", Stage: domain.ActiveStage},
		{Version: "v5", Stage: domain.CandidateStage},
		{Version: "v6", Stage: domain.CandidateStage},
	}
	_, v2controller := prepare()

	for i := 1; i <= 4; i++ {
		response := sendHttpRequest(t, http.MethodPost, promoteVersionPath,
			fmt.Sprintf(reqPromoteVersionPath, fmt.Sprintf("v%d", i)), v2controller.PromoteHandlerUnsecure)
		defer response.Body.Close()
		assert.NotNil(t, response)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	}

	response := sendHttpRequest(t, http.MethodPost, promoteVersionPath,
		fmt.Sprintf(reqPromoteVersionPath, "v5"), v2controller.PromoteHandlerUnsecure)
	defer response.Body.Close()
	assert.NotNil(t, response)
	assert.Equal(t, http.StatusAccepted, response.StatusCode)
}

/* TODO Rewrite
func TestV2Controller_Rollback(t *testing.T) {
	genericDao, v2controller := prepare()
	responseRecorder := sendHttpRequest(t, http.MethodPost, promoteVersionPath,
		fmt.Sprintf(reqPromoteVersionPath, expectedVersion), v2controller.PromoteHandlerUnsecure)
	assert.NotNil(t, responseRecorder)
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	var actualDeploymentVersions []*domain.DeploymentVersion
	err := json.Unmarshal([]byte(responseRecorder.Body.String()), &actualDeploymentVersions)
	assert.Nil(t, err)

	//deployments
	foundDeploymentVersion, err := genericDao.FindAllDeploymentVersions()
	assert.Nil(t, err)
	assert.NotEmpty(t, foundDeploymentVersion)
	assert.Equal(t, 3, len(actualDeploymentVersions))
	assert.Equal(t, 3, len(foundDeploymentVersion))
	assert.Equal(t, len(foundDeploymentVersion), len(actualDeploymentVersions))
	assert.True(t, reflect.DeepEqual(actualDeploymentVersions, foundDeploymentVersion))

	rollbackResponse := sendHttpRequest(t, http.MethodPost, rollbackVersionPath,
		rollbackVersionPath, v2controller.RollbackHandlerUnsecure)
	assert.NotNil(t, rollbackResponse)
	assert.Equal(t, http.StatusAccepted, rollbackResponse.Code)

	//deployments
	var actualDeploymentVersionsRollback []*domain.DeploymentVersion
	err = json.Unmarshal([]byte(rollbackResponse.Body.String()), &actualDeploymentVersionsRollback)
	assert.Nil(t, err)
	assert.NotEmpty(t, actualDeploymentVersionsRollback)
	assert.Equal(t, 3, len(actualDeploymentVersionsRollback))

	expectedAllDeploymentVersions, err := genericDao.FindAllDeploymentVersions()
	assert.Nil(t, err)
	assert.NotEmpty(t, expectedAllDeploymentVersions)
	assert.Equal(t, 3, len(expectedAllDeploymentVersions))
	assert.True(t, reflect.DeepEqual(actualDeploymentVersionsRollback, expectedAllDeploymentVersions))

	for _, actualDVersion := range actualDeploymentVersionsRollback {
		switch actualDVersion.Version {
		case "v3":
			assert.Equal(t, domain.ArchivedStage, actualDVersion.Stage)
			break
		case "v4":
			assert.Equal(t, domain.ActiveStage, actualDVersion.Stage)
			break
		case "v6":
			assert.Equal(t, domain.CandidateStage, actualDVersion.Stage)
			break
		}
	}

	//clusters
	//check clusters
	actualClusters, err := genericDao.FindAllClusters()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClusters)
	assert.Equal(t, len(expectedClusters)-1, len(actualClusters))
	for _, actualCluster := range actualClusters {
		assert.NotEqual(t, expectedClusters[0].Name, actualCluster.Name)
	}

	//endpoints
	endpointsByCluster := make(map[int32]int)
	endpointsByVersion := make(map[string]int)
	endpoints, err := genericDao.FindAllEndpoints()
	assert.Nil(t, err)
	assert.NotEmpty(t, endpoints)
	for _, endpoint := range endpoints {
		assert.NotEqual(t, "v2", endpoint.DeploymentVersion)
		assert.NotEqual(t, "v5", endpoint.DeploymentVersion)
		endpointsByCluster[endpoint.ClusterId] += 1
		endpointsByVersion[endpoint.DeploymentVersion] += 1
	}
	assert.Equal(t, 0, endpointsByCluster[1])
	assert.Equal(t, 3, endpointsByCluster[2])
	assert.Equal(t, 1, endpointsByCluster[3])
	assert.Equal(t, 2, endpointsByCluster[4])
	assert.Equal(t, 1, endpointsByCluster[5])
	assert.Equal(t, 1, endpointsByVersion["v3"])
	assert.Equal(t, 1, endpointsByVersion["v4"])
	assert.Equal(t, 5, endpointsByVersion["v6"])

	//routes
	actualRoutes, err := genericDao.FindAllRoutes()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualRoutes)
	assert.Equal(t, len(expectedRoutes)-5, len(actualRoutes))
	routeResponse404 := 0
	routesByVersion := make(map[string]int)
	for _, actualRoute := range actualRoutes {
		assert.NotEqual(t, "v2", actualRoute.DeploymentVersion)
		assert.NotEqual(t, "v5", actualRoute.DeploymentVersion)
		routesByVersion[actualRoute.DeploymentVersion] += 1
		if actualRoute.DirectResponseCode == 404 {
			routeResponse404++
		}
	}
	assert.Equal(t, 1, routesByVersion["v3"])
	assert.Equal(t, 1, routesByVersion["v4"])
	assert.Equal(t, 2, routesByVersion["v6"])
	assert.Equal(t, 1, routeResponse404)

	//clusters node group
	actualClustersNodeGroups, err := genericDao.FindAllClusterWithNodeGroup()
	assert.Nil(t, err)
	assert.NotEmpty(t, actualClustersNodeGroups)
	assert.Equal(t, 12, len(actualClustersNodeGroups))
	for _, actualClusterNodeGroup := range actualClustersNodeGroups {
		assert.NotEqual(t, expectedClusters[0].Id, actualClusterNodeGroup.ClustersId)
	}
}
*/

func TestV2Controller_Rollback_409(t *testing.T) {
	_, v2controller := prepare()
	response := sendHttpRequest(t, http.MethodPost, rollbackVersionPath,
		rollbackVersionPath, v2controller.RollbackHandlerUnsecure)
	assert.NotNil(t, response)
	assert.Equal(t, http.StatusConflict, response.StatusCode)
}

// private
func (v2 *BlueGreenController) RollbackHandlerUnsecure(ctx *fiber.Ctx) error {
	return v2.HandlePostRollbackVersion(ctx)
}

func (v2 *BlueGreenController) PromoteHandlerUnsecure(ctx *fiber.Ctx) error {
	return v2.HandlePostPromoteVersion(ctx)
}

func (v2 *BlueGreenController) DeleteDeploymentVersionUnsecure(ctx *fiber.Ctx) error {
	return v2.HandleDeleteDeploymentVersionWithID(ctx)
}

func (v2 *BlueGreenController) GetAllDeploymentVersionsUnsecure(ctx *fiber.Ctx) error {
	return v2.HandleGetDeploymentVersions(ctx)
}

func prepare() (*dao.InMemDao, *BlueGreenController) {
	inMemStorage := ram.NewStorage()
	genericDao := dao.NewInMemDao(inMemStorage, &IdGeneratorMock{}, []func([]memdb.Change) error{FlushChanges})
	internalBus := bus.GetInternalBusInstance()
	eventBus := bus.NewEventBusAggregator(genericDao, internalBus, internalBus, nil, nil)
	entityService := entity.NewService("v1")
	lbService := loadbalance.NewLoadBalanceService(genericDao, entityService, eventBus)
	bgRegistry := bluegreen.NewVersionsRegistry(genericDao, entityService, eventBus)
	blueGreenService := bluegreen.NewService(entityService, lbService, genericDao, eventBus, bgRegistry)
	v2BlueGreenController := NewBlueGreenController(blueGreenService, genericDao)
	prepareDeploymentVersions(genericDao)
	prepareNodeGroups(genericDao)
	prepareRouteConfigs(genericDao)
	prepareVirtualHosts(genericDao)
	prepareCreateClusters(genericDao)
	prepareClusterNodeGroups(genericDao)
	prepareEndpoints(genericDao)
	prepareRoutes(genericDao)
	return genericDao, v2BlueGreenController
}

func FlushChanges(changes []memdb.Change) error {
	flusher := &constancy.Flusher{BatchTm: &batchTransactionManagerMock{}}
	return flusher.Flush(changes)
}

func sendHttpRequest(t *testing.T, httpMethod, endpoint, reqUrl string, f func(fiberCtx *fiber.Ctx) error) *http.Response {
	fiberConfig := fiber.Config{
		ErrorHandler: errorcodes.DefaultErrorHandlerWrapper(errorcodes.UnknownErrorCode),
	}
	app, err := fiberserver.New(fiberConfig).Process()
	assert.Nil(t, err)
	app.Add(httpMethod, endpoint, f)
	req, err := http.NewRequest(httpMethod,
		reqUrl,
		bytes.NewBufferString(""),
	)
	req.Host = "localhost"
	assert.Nil(t, err)
	resp, err := app.Test(req, -1)
	return resp
}

type IdGeneratorMock struct {
	seq int32
}

type batchTransactionManagerMock struct{}

func (generator *IdGeneratorMock) Generate(uniqEntity domain.Unique) error {
	if uniqEntity.GetId() == 0 {
		uniqEntity.SetId(atomic.AddInt32(&generator.seq, 1))
	}
	return nil
}

func (tm *batchTransactionManagerMock) WithTxBatch(_ func(tx constancy.BatchStorage) error) error {
	return nil
}

func (tm *batchTransactionManagerMock) IsCurrentPodDefinedAsMaster() (bool, error) {
	return true, nil
}

func prepareDeploymentVersions(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, deploymentVersion := range expectedDeploymentVersions {
			newDv := deploymentVersion
			dao.SaveDeploymentVersion(&newDv)
		}
		return nil
	})
}

func prepareNodeGroups(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, nodeGroup := range expectedNodeGroups {
			newNodeGroup := nodeGroup
			dao.SaveNodeGroup(&newNodeGroup)
		}
		return nil
	})
}

func prepareRouteConfigs(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, routeConfig := range expectedRouteConfigs {
			newRouteConfig := routeConfig
			dao.SaveRouteConfig(&newRouteConfig)
		}
		return nil
	})
}

func prepareVirtualHosts(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, virtualHost := range expectedVirtualHosts {
			newVirtualHost := virtualHost
			dao.SaveVirtualHost(&newVirtualHost)
		}
		return nil
	})
}

func prepareCreateClusters(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, cluster := range expectedClusters {
			newCluster := cluster
			dao.SaveCluster(&newCluster)
		}
		return nil
	})
}

func prepareClusterNodeGroups(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, nodeGroup := range expectedNodeGroups {
			for _, cluster := range expectedClusters {
				clusterNodeGroup := domain.NewClusterNodeGroups(cluster.Id, nodeGroup.Name)
				dao.SaveClustersNodeGroup(clusterNodeGroup)
			}
		}
		return nil
	})
}

func prepareRoutes(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, route := range expectedRoutes {
			newRoute := route
			dao.SaveRoute(&newRoute)
		}
		return nil
	})
}

func prepareEndpoints(genericDao *dao.InMemDao) {
	genericDao.WithWTx(func(dao dao.Repository) error {
		for _, endpoint := range expectedEndpoints {
			newEndpoint := endpoint
			dao.SaveEndpoint(&newEndpoint)
		}
		return nil
	})
}
