package config

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	test_mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	test_mock_event_bus "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
)

type GeneratorMock struct {
	counter int32
}

func (g *GeneratorMock) Generate(uniqEntity domain.Unique) error {
	if uniqEntity.GetId() == 0 {
		uniqEntity.SetId(atomic.AddInt32(&g.counter, 1))
	}
	return nil
}

var (
	enableErrorOnAccessToPersistentStorageMode = false
)

func TestCommonConfiguration_CreateCommonConfiguration(t *testing.T) {
	_ = os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	testable := dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, []func([]memdb.Change) error{flushChangesToPersistenceStorage})
	entityService := entity.NewService("v1")
	commonConfig := NewCommonConfiguration(testable, entityService, false)
	nodeGroups := []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}
	deploymentVersion := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	enableErrorOnAccessToPersistentStorageMode = false

	insertNodeGroups(t, testable, nodeGroups)

	saveActiveDeploymentVersion(t, testable, deploymentVersion)

	assert.NotPanics(t, func() {
		commonConfig.CreateCommonConfiguration()
	})

	verifyClusters(t, testable, false)
	verifyClusterWithNodeGroup(t, testable, false)
	verifyEndpoints(t, testable, false)
	verifyListeners(t, testable)
	verifyVirtualHosts(t, testable)
	verifyVirtualHostDomain(t, testable)
	verifyRouteConfiguration(t, testable)
	verifyRoutes(t, testable)
	verifyChangesMaps(t, commonConfig, false)
}

func TestServiceMeshDetector_WrapRouteCreator(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		gatewayName   string
		shouldSkip    bool
		creatorCalled bool
	}{
		{
			name:          "Istio enabled - skip internal gateway",
			envValue:      "istio",
			gatewayName:   "internal",
			shouldSkip:    true,
			creatorCalled: false,
		},
		{
			name:          "Istio enabled - skip private gateway",
			envValue:      "istio",
			gatewayName:   "private",
			shouldSkip:    true,
			creatorCalled: false,
		},
		{
			name:          "No Istio - execute",
			envValue:      "",
			gatewayName:   "public",
			shouldSkip:    false,
			creatorCalled: true,
		},
		{
			name:          "Other mesh type - execute",
			envValue:      "Core",
			gatewayName:   "internal",
			shouldSkip:    false,
			creatorCalled: true,
		},
	}

	testable := dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, []func([]memdb.Change) error{flushChangesToPersistenceStorage})
	entityService := entity.NewService("v1")
	deploymentVersion := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	vHostId := int32(0)
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("SERVICE_MESH_TYPE", tt.envValue)
				defer os.Unsetenv("SERVICE_MESH_TYPE")
			} else {
				os.Unsetenv("SERVICE_MESH_TYPE")
			}
			
			creatorCalled := false
			creator := func(storage dao.Repository, entityService entity.ServiceInterface, virtualHostId int32, currentDeploymentVersion string,
	initialDeploymentVersion string) error  {
				creatorCalled = true
				return nil
			}

			detector := NewServiceMeshDetector(logger)
			wrappedCreator := detector.WrapRouteCreator(creator, tt.gatewayName)
			
			err := wrappedCreator(testable, entityService, vHostId, "", deploymentVersion.Version)
			assert.NoError(t, err)
			assert.Equal(t, tt.creatorCalled, creatorCalled)
		})
	}
}

func TestCommonConfiguration_CreateSecuredCommonConfiguration(t *testing.T) {
	_ = os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	testable := dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, []func([]memdb.Change) error{flushChangesToPersistenceStorage})
	entityService := entity.NewService("v1")
	commonConfig := NewCommonConfiguration(testable, entityService, true)
	nodeGroups := []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}
	deploymentVersion := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	enableErrorOnAccessToPersistentStorageMode = false

	insertNodeGroups(t, testable, nodeGroups)

	saveActiveDeploymentVersion(t, testable, deploymentVersion)

	assert.NotPanics(t, func() {
		commonConfig.CreateCommonConfiguration()
	})

	verifyClusters(t, testable, true)
	verifyClusterWithNodeGroup(t, testable, true)
	verifyEndpoints(t, testable, true)
	verifyListeners(t, testable)
	verifyVirtualHosts(t, testable)
	verifyVirtualHostDomain(t, testable)
	verifyRouteConfiguration(t, testable)
	verifyRoutes(t, testable)
	verifyChangesMaps(t, commonConfig, true)
}

func flushChangesToPersistenceStorage(changes []memdb.Change) error {
	if enableErrorOnAccessToPersistentStorageMode {
		return fmt.Errorf("error of access to storage")
	}
	return nil
}

func insertNodeGroups(t *testing.T, testable *dao.InMemDao, nodeGroups []*domain.NodeGroup) {
	_, err := testable.WithWTx(func(dao dao.Repository) error {
		for _, nodeGroup := range nodeGroups {
			assert.Nil(t, dao.SaveNodeGroup(nodeGroup))
		}
		return nil
	})
	assert.Nil(t, err)
	// check node groups inserted
	selectedNodeGroups, err := testable.FindAllNodeGroups()
	assert.Nil(t, err)
	assert.NotNil(t, selectedNodeGroups)
	assert.Equal(t, len(nodeGroups), len(selectedNodeGroups))
	assert.ObjectsAreEqualValues(nodeGroups, selectedNodeGroups)
}

func saveActiveDeploymentVersion(t *testing.T, testable *dao.InMemDao, deploymentVersion *domain.DeploymentVersion) {
	_, err := testable.WithWTx(func(dao dao.Repository) error {
		assert.Nil(t, dao.SaveDeploymentVersion(deploymentVersion))
		return nil
	})
	assert.Nil(t, err)
}

func TestCreateCommonConfigurationWithCatchingAccessToDbError(t *testing.T) {
	_ = os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	testable := dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, []func([]memdb.Change) error{flushChangesToPersistenceStorage})
	entityService := entity.NewService("v1")
	commonConfig := NewCommonConfiguration(testable, entityService, false)
	nodeGroups := []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}
	deploymentVersion := domain.NewDeploymentVersion("v1", domain.ActiveStage)

	insertNodeGroups(t, testable, nodeGroups)

	saveActiveDeploymentVersion(t, testable, deploymentVersion)

	enableErrorOnAccessToPersistentStorageMode = true
	var err error
	assert.NotPanics(t, func() {
		err = commonConfig.CreateCommonConfiguration()
	})
	assert.NotNil(t, err)
	assert.Containsf(t, err.Error(), "error of access to storage", "wrong error message")
	enableErrorOnAccessToPersistentStorageMode = false
}

func TestCreateCommonConfiguration(t *testing.T) {
	_ = os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	testable := dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, []func([]memdb.Change) error{flushChangesToPersistenceStorage})
	entityService := entity.NewService("v1")
	commonConfig := NewCommonConfiguration(testable, entityService, false)
	nodeGroups := []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}
	deploymentVersion := domain.NewDeploymentVersion("v1", domain.ActiveStage)

	insertNodeGroups(t, testable, nodeGroups)

	saveActiveDeploymentVersion(t, testable, deploymentVersion)

	enableErrorOnAccessToPersistentStorageMode = false

	err := commonConfig.CreateCommonConfiguration()
	assert.Nil(t, err)
}

func verifyClusters(t *testing.T, testable *dao.InMemDao, secured bool) {
	selectedClusters, err := testable.FindAllClusters()
	assert.Nil(t, err)
	assert.NotNil(t, selectedClusters)
	if secured {
		assert.Equal(t, 3, len(selectedClusters))
	} else {
		assert.Equal(t, 2, len(selectedClusters))
	}

	csClusterVerified := false
	cpClusterVerified := false
	extAuthzClusterVerified := false
	for _, cluster := range selectedClusters {
		if domain.ExtAuthClusterName == cluster.Name {
			extAuthzClusterVerified = true
		} else {
			if "config-server||config-server||8080" == cluster.Name {
				csClusterVerified = true
			} else if "control-plane||control-plane||8080" == cluster.Name {
				cpClusterVerified = true
			}
		}
	}
	assert.True(t, csClusterVerified)
	assert.True(t, cpClusterVerified)
	if secured {
		assert.True(t, extAuthzClusterVerified)
	}
}

func verifyTlsConfig(t *testing.T, testable *dao.InMemDao, tlsConfigId int32) {
	tlsConfig, err := testable.FindTlsConfigById(tlsConfigId)
	assert.Nil(t, err)
	assert.NotNil(t, tlsConfig)
	assert.True(t, tlsConfig.Enabled)
	assert.Empty(t, tlsConfig.TrustedCA)
	assert.Empty(t, tlsConfig.ClientCert)
	assert.Empty(t, tlsConfig.PrivateKey)
}

func verifyClusterWithNodeGroup(t *testing.T, testable *dao.InMemDao, secured bool) {
	selected, err := testable.FindAllClusterWithNodeGroup()
	assert.Nil(t, err)
	assert.NotNil(t, selected)
	if secured {
		assert.Equal(t, 8, len(selected))
	} else {
		assert.Equal(t, 5, len(selected))
	}
}

func verifyEndpoints(t *testing.T, testable *dao.InMemDao, secured bool) {
	selectedEndpoints, err := testable.FindAllEndpoints()
	assert.Nil(t, err)
	assert.NotNil(t, selectedEndpoints)
	if secured {
		assert.Equal(t, 3, len(selectedEndpoints))
	} else {
		assert.Equal(t, 2, len(selectedEndpoints))
	}
}

func verifyRouteConfiguration(t *testing.T, testable *dao.InMemDao) {
	selectedRouteConfigs, err := testable.FindAllRouteConfigs()
	assert.Nil(t, err)
	assert.NotNil(t, selectedRouteConfigs)
	assert.Equal(t, 3, len(selectedRouteConfigs))
}

func verifyListeners(t *testing.T, testable *dao.InMemDao) {
	selectedListeners, err := testable.FindAllListeners()
	assert.Nil(t, err)
	assert.NotNil(t, selectedListeners)
	assert.Equal(t, 3, len(selectedListeners))
}

func verifyVirtualHosts(t *testing.T, testable *dao.InMemDao) {
	selectedVirtualHosts, err := testable.FindAllVirtualHosts()
	assert.Nil(t, err)
	assert.NotNil(t, selectedVirtualHosts)
	assert.Equal(t, 3, len(selectedVirtualHosts))
}

func verifyVirtualHostDomain(t *testing.T, testable *dao.InMemDao) {
	selectedVirtualHostsDomain, err := testable.FindAllVirtualHostsDomain()
	assert.Nil(t, err)
	assert.NotNil(t, selectedVirtualHostsDomain)
	assert.Equal(t, 3, len(selectedVirtualHostsDomain))
}

func verifyRoutes(t *testing.T, testable *dao.InMemDao) {
	selectedRotes, err := testable.FindAllRoutes()
	assert.Nil(t, err)
	assert.NotNil(t, selectedRotes)
	assert.Equal(t, 45, len(selectedRotes))
}

func verifyChangesMaps(t *testing.T, commonConfig *CommonConfiguration, secured bool) {
	assert.NotNil(t, commonConfig.changes)
	assert.Equal(t, 3, len(commonConfig.changes))
	assert.NotNil(t, commonConfig.changes[domain.PublicGateway])
	assert.NotNil(t, commonConfig.changes[domain.PrivateGateway])
	assert.NotNil(t, commonConfig.changes[domain.InternalGateway])

	assert.Equal(t, 3, len(commonConfig.envoyChangesMap))
	assert.NotNil(t, commonConfig.envoyChangesMap[domain.PublicGateway])
	assert.NotNil(t, commonConfig.envoyChangesMap[domain.PrivateGateway])
	assert.NotNil(t, commonConfig.envoyChangesMap[domain.InternalGateway])

	if secured {
		assert.Equal(t, 26, len(commonConfig.changes[domain.InternalGateway]))
		assert.Equal(t, 14, len(commonConfig.changes[domain.PublicGateway]))
		assert.Equal(t, 44, len(commonConfig.changes[domain.PrivateGateway]))
		assert.Equal(t, 4, len(commonConfig.envoyChangesMap[domain.PublicGateway]))
		assert.Equal(t, 5, len(commonConfig.envoyChangesMap[domain.PrivateGateway]))
		assert.Equal(t, 5, len(commonConfig.envoyChangesMap[domain.InternalGateway]))
	} else {
		assert.Equal(t, 22, len(commonConfig.changes[domain.InternalGateway]))
		assert.Equal(t, 10, len(commonConfig.changes[domain.PublicGateway]))
		assert.Equal(t, 40, len(commonConfig.changes[domain.PrivateGateway]))
		assert.Equal(t, 3, len(commonConfig.envoyChangesMap[domain.PublicGateway]))
		assert.Equal(t, 4, len(commonConfig.envoyChangesMap[domain.PrivateGateway]))
		assert.Equal(t, 4, len(commonConfig.envoyChangesMap[domain.InternalGateway]))
	}
}

func Test_CreateCommonConfigurationReturnsError_onFindAllNodeGroupsFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	testableMock := test_mock_dao.NewMockDao(ctrl)

	entityService := entity.NewService("v1")
	commonConfigMock := NewCommonConfiguration(testableMock, entityService, false)

	testableMock.EXPECT().FindAllNodeGroups().Return(nil, fmt.Errorf("FindAllNodeGroups has returned error"))

	err := commonConfigMock.CreateCommonConfiguration()
	assert.Error(t, err)
}

func Test_CreateCommonConfigurationReturnsError_OnGenerateAndSaveEnvoyConfigVersionFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	testableMock := test_mock_dao.NewMockDao(ctrl)
	entityService := entity.NewService("v1")
	commonConfigMock := NewCommonConfiguration(testableMock, entityService, false)

	nodeGroups := []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}

	testableMock.EXPECT().FindAllNodeGroups().Return(nodeGroups, nil)
	testableMock.EXPECT().WithWTx(gomock.Any()).Return(nil, nil)
	testableMock.EXPECT().WithWTx(gomock.Any()).Return(nil, fmt.Errorf("generateAndSaveEnvoyConfigVersion has failed with error"))

	err := commonConfigMock.CreateCommonConfiguration()
	assert.Error(t, err)
}

func Test_CreateCommonConfigurationReturnsError_OnSaveDefaultGatewayConfigFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	testableMock := test_mock_dao.NewMockDao(ctrl)
	entityService := entity.NewService("v1")
	commonConfigMock := NewCommonConfiguration(testableMock, entityService, false)

	nodeGroups := []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}

	testableMock.EXPECT().FindAllNodeGroups().Return(nodeGroups, nil)
	testableMock.EXPECT().WithWTx(gomock.Any()).Times(16).Return(nil, nil)
	testableMock.EXPECT().WithWTx(gomock.Any()).Return(nil, fmt.Errorf("saveDefaultGatewayConfig has failed with error"))

	err := commonConfigMock.CreateCommonConfiguration()
	assert.Error(t, err)
}

func Test_CreateCommonConfigurationReturnsError_OnSaveEnvoyConfigVersionsForGatewayFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	testableMock := test_mock_dao.NewMockDao(ctrl)
	entityService := entity.NewService("v1")
	commonConfigMock := NewCommonConfiguration(testableMock, entityService, false)

	nodeGroups := []*domain.NodeGroup{
		domain.NewNodeGroup(domain.PublicGateway),
		domain.NewNodeGroup(domain.PrivateGateway),
		domain.NewNodeGroup(domain.InternalGateway),
	}

	testableMock.EXPECT().FindAllNodeGroups().Return(nodeGroups, nil)
	testableMock.EXPECT().WithWTx(gomock.Any()).Times(16).Return(nil, nil)
	testableMock.EXPECT().WithWTx(gomock.Any()).Return(nil, nil)
	testableMock.EXPECT().WithWTx(gomock.Any()).Return(nil, fmt.Errorf("saveEnvoyConfigVersionsForGateway has failed with error"))

	err := commonConfigMock.CreateCommonConfiguration()
	assert.Error(t, err)
}

func Test_PublishChanges_SuccessPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	publ := test_mock_event_bus.NewMockBusPublisher(ctrl)

	_ = os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	testable := dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, []func([]memdb.Change) error{flushChangesToPersistenceStorage})
	entityService := entity.NewService("v1")
	commonConfig := NewCommonConfiguration(testable, entityService, false)

	commonConfig.changes = map[string][]memdb.Change{
		"int": {memdb.Change{Table: "table1"}},
	}
	commonConfig.envoyChangesMap = map[string][]memdb.Change{
		"int": {memdb.Change{Table: "table2"}},
	}

	publ.EXPECT().Publish(gomock.Eq(bus.TopicChanges), gomock.Any()).Do(func(topic, data interface{}) {
		event := data.(*events.ChangeEvent)
		assert.Equal(t, "int", event.NodeGroup)
		assert.Equal(t, 2, len(event.Changes))
	})
	err := commonConfig.PublishChanges(publ)
	assert.Nil(t, err)

	commonConfig.changes = map[string][]memdb.Change{}
	commonConfig.envoyChangesMap = map[string][]memdb.Change{}
}

func Test_PublishChanges_PublishFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	publ := test_mock_event_bus.NewMockBusPublisher(ctrl)

	_ = os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()

	testable := dao.NewInMemDao(ram.NewStorage(), &GeneratorMock{}, []func([]memdb.Change) error{flushChangesToPersistenceStorage})
	entityService := entity.NewService("v1")
	commonConfig := NewCommonConfiguration(testable, entityService, false)

	commonConfig.changes = map[string][]memdb.Change{
		"int": {memdb.Change{Table: "table1"}},
	}

	publ.EXPECT().Publish(gomock.Eq(bus.TopicChanges), gomock.Any()).
		Return(fmt.Errorf("publish has failed with error"))
	err := commonConfig.PublishChanges(publ)
	assert.Error(t, err)

	commonConfig.changes = map[string][]memdb.Change{}
}
