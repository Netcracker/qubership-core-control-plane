package constancy

import (
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	mock_constancy "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/constancy"
	mock_db "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/db"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"testing"
)


func TestFindLuaFilterById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.LuaFilter{}
	storage := getAndMockStorage(ctrl, &domain.LuaFilter{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindLuaFilterById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}
func TestFindWasmFilterById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.WasmFilter{}
	storage := getAndMockStorage(ctrl, &domain.WasmFilter{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindWasmFilterById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestCircuitBreakerById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.CircuitBreaker{}
	storage := getAndMockStorage(ctrl, &domain.CircuitBreaker{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindCircuitBreakerById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFinThresholdById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.Threshold{}
	storage := getAndMockStorage(ctrl, &domain.Threshold{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindThresholdById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindVirtualHostById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.VirtualHost{}
	storage := getAndMockStorage(ctrl, &domain.VirtualHost{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindVirtualHostById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindTlsConfigById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.TlsConfig{}
	storage := getAndMockStorage(ctrl, &domain.TlsConfig{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindTlsConfigById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindRouteById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.Route{}
	storage := getAndMockStorage(ctrl, &domain.Route{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindRouteById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindRouteConfigById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.RouteConfiguration{}
	storage := getAndMockStorage(ctrl, &domain.RouteConfiguration{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindRouteConfigById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindRetryPolicyById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.RetryPolicy{}
	storage := getAndMockStorage(ctrl, &domain.RetryPolicy{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindRetryPolicyById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindListenerById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.Listener{}
	storage := getAndMockStorage(ctrl, &domain.Listener{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindListenerById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindHealthCheckById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.HealthCheck{}
	storage := getAndMockStorage(ctrl, &domain.HealthCheck{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindHealthCheckById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindHeaderMatcherById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.HeaderMatcher{}
	storage := getAndMockStorage(ctrl, &domain.HeaderMatcher{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindHeaderMatcherById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindHashPolicyById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.HashPolicy{}
	storage := getAndMockStorage(ctrl, &domain.HashPolicy{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindHashPolicyById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindEndpointById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.Endpoint{}
	storage := getAndMockStorage(ctrl, &domain.Endpoint{}, expectedResult)

	id := int32(1)
	cluster, err := storage.FindEndpointById(id)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindDeploymentVersionByName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.DeploymentVersion{}
	storage := getAndMockStorage(ctrl, &domain.DeploymentVersion{}, expectedResult)

	version := "version"
	cluster, err := storage.FindDeploymentVersionByName(version)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindCompositeSatelliteByNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.CompositeSatellite{}
	storage := getAndMockStorage(ctrl, &domain.CompositeSatellite{}, expectedResult)

	namespace := "namespace"
	cluster, err := storage.FindCompositeSatelliteByNamespace(namespace)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindClustersNodeGroupByIdAndNodeGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.ClustersNodeGroup{}
	storage := getAndMockStorage(ctrl, &domain.ClustersNodeGroup{}, expectedResult)

	clusterId := int32(1)
	nodeGroup := "nodeGroup"
	cluster, err := storage.FindClustersNodeGroupByIdAndNodeGroup(clusterId, nodeGroup)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindTlsConfigByIdAndNodeGroupNameNodeGroupByIdAndNodeGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.TlsConfigsNodeGroups{}
	storage := getAndMockStorage(ctrl, &domain.TlsConfigsNodeGroups{}, expectedResult)

	tlsConfigId := int32(1)
	nodeGroup := "nodeGroup"
	cluster, err := storage.FindTlsConfigByIdAndNodeGroupName(tlsConfigId, nodeGroup)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func TestFindClusterByName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedResult := &domain.Cluster{}
	storage := getAndMockStorage(ctrl, &domain.Cluster{}, expectedResult)

	clusterName := "name"
	cluster, err := storage.FindClusterByName(clusterName)
	assert.Nil(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, expectedResult, cluster)
}

func getAndMockStorage(ctrl *gomock.Controller, model interface{}, expectedResult interface{}) *StorageImpl {
	mockDBProvider := getMockDBProvider(ctrl)
	mockPGQueryWrapper := getMockPGQueryWrapper(ctrl)
	mockPGConnWrapper := getMockPGConnWrapper(ctrl)
	mockPGTxWrapper := getMockPGTxWrapper(ctrl)
	storage := &StorageImpl{
		DbProvider: mockDBProvider,
		PGQuery:    mockPGQueryWrapper,
		PGConn:     mockPGConnWrapper,
		PGTx:       mockPGTxWrapper,
	}
	tx := &bun.Tx{}
	query := &bun.SelectQuery{}
	conn := &bun.Conn{}

	mockPGTxWrapper.EXPECT().Commit(tx).Return(nil)
	mockPGTxWrapper.EXPECT().Rollback(tx).Return(nil)
	mockPGConnWrapper.EXPECT().Close(conn).Return(nil)

	mockDBProvider.EXPECT().GetConn(gomock.Any()).Return(conn, nil)
	mockPGConnWrapper.EXPECT().Begin(conn).Return(tx, nil)
	mockPGConnWrapper.EXPECT().Model(gomock.Any(), gomock.Any()).DoAndReturn(func(conn *bun.SelectQuery, model1 interface{}) interface{} {
		model1 = expectedResult
		return query
	})
	mockPGQueryWrapper.EXPECT().Select(gomock.Any()).DoAndReturn(func(conn *bun.Conn) interface{} {
		return query
	})
	mockPGQueryWrapper.EXPECT().Scan(gomock.Any()).DoAndReturn(func(query *bun.SelectQuery) interface{} {
		return nil
	})

	return storage
}

func getMockDBProvider(ctrl *gomock.Controller) *mock_db.MockDBProvider {
	mock := mock_db.NewMockDBProvider(ctrl)
	return mock
}

func getMockPGQueryWrapper(ctrl *gomock.Controller) *mock_constancy.MockPGQueryWrapper {
	mock := mock_constancy.NewMockPGQueryWrapper(ctrl)
	return mock
}

func getMockPGConnWrapper(ctrl *gomock.Controller) *mock_constancy.MockPGConnWrapper {
	mock := mock_constancy.NewMockPGConnWrapper(ctrl)
	return mock
}

func getMockPGTxWrapper(ctrl *gomock.Controller) *mock_constancy.MockPGTxWrapper {
	mock := mock_constancy.NewMockPGTxWrapper(ctrl)
	return mock
}
