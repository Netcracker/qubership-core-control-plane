package cluster

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	mock_bus "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/event/bus"
	"github.com/netcracker/qubership-core-lib-go-error-handling/v3/errors"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"testing"
)

const (
	GATEWAY_NAME       = "testGateway"
	CLUSTER_NAME       = "testClusterName"
	TLS_CONFIG_NAME    = "testTlsConfigName"
	TRUSTED_CA         = "testTlsConfigName"
	TLS_ID             = 12
	CIRCUIT_BREAKER_ID = 7
	THRESHOLD_ID       = 2
)

func TestService_AddClusterDaoProvided(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(getCircuitBreaker(), nil)
	mockDao.EXPECT().FindThresholdById(int32(THRESHOLD_ID)).Return(getThreshold(), nil)
	mockDao.EXPECT().SaveThreshold(getThreshold()).Return(nil)
	mockDao.EXPECT().SaveCircuitBreaker(getCircuitBreaker()).Return(nil)
	mockDao.EXPECT().SaveCluster(gomock.Any()).Return(nil)
	mockDao.EXPECT().FindClustersNodeGroup(gomock.Any()).Return(&domain.ClustersNodeGroup{}, nil)
	mockDao.EXPECT().FindDeploymentVersionsByStage(domain.ActiveStage).Return([]*domain.DeploymentVersion{{Version: "v1", Stage: domain.ActiveStage}}, nil)
	mockDao.EXPECT().FindEndpointsByClusterName(gomock.Any()).Return([]*domain.Endpoint{}, nil)
	mockDao.EXPECT().FindEndpointsByClusterIdAndDeploymentVersion(gomock.Any(), gomock.Any()).Return([]*domain.Endpoint{}, nil)
	mockDao.EXPECT().SaveEndpoint(gomock.Any())
	mockDao.EXPECT().SaveEnvoyConfigVersion(gomock.Any()).Return(nil)

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_SaveClusterReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(getCircuitBreaker(), nil)
	mockDao.EXPECT().FindThresholdById(int32(THRESHOLD_ID)).Return(getThreshold(), nil)
	mockDao.EXPECT().SaveThreshold(getThreshold()).Return(nil)
	mockDao.EXPECT().SaveCircuitBreaker(getCircuitBreaker()).Return(nil)
	mockDao.EXPECT().SaveCluster(gomock.Any()).Return(errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_ClusterWithoutConnectionLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(getCircuitBreaker(), nil)
	mockDao.EXPECT().DeleteThresholdById(int32(THRESHOLD_ID)).Return(nil)
	mockDao.EXPECT().DeleteCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(nil)
	mockDao.EXPECT().SaveCluster(gomock.Any()).Return(nil)
	mockDao.EXPECT().FindClustersNodeGroup(gomock.Any()).Return(&domain.ClustersNodeGroup{}, nil)
	mockDao.EXPECT().FindDeploymentVersionsByStage(domain.ActiveStage).Return([]*domain.DeploymentVersion{{Version: "v1", Stage: domain.ActiveStage}}, nil)
	mockDao.EXPECT().FindEndpointsByClusterName(gomock.Any()).Return([]*domain.Endpoint{}, nil)
	mockDao.EXPECT().FindEndpointsByClusterIdAndDeploymentVersion(gomock.Any(), gomock.Any()).Return([]*domain.Endpoint{}, nil)
	mockDao.EXPECT().SaveEndpoint(gomock.Any())
	mockDao.EXPECT().SaveEnvoyConfigVersion(gomock.Any()).Return(nil)

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	dtoCluster := getDtoCluster(false)
	dtoCluster.CircuitBreaker.Threshold.MaxConnections = 0

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, dtoCluster)
}

func TestService_AddClusterDaoProvided_ClusterWithoutConnectionLimit_DeleteCircuitBreakerCascadeByIdReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(nil, errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	dtoCluster := getDtoCluster(false)
	dtoCluster.CircuitBreaker.Threshold.MaxConnections = 0

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, dtoCluster)
}

func TestService_AddClusterDaoProvided_SaveCircuitBreakerReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(getCircuitBreaker(), nil)
	mockDao.EXPECT().FindThresholdById(int32(THRESHOLD_ID)).Return(getThreshold(), nil)
	mockDao.EXPECT().SaveThreshold(getThreshold()).Return(nil)
	mockDao.EXPECT().SaveCircuitBreaker(getCircuitBreaker()).Return(errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_SaveThresholdReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(getCircuitBreaker(), nil)
	mockDao.EXPECT().FindThresholdById(int32(THRESHOLD_ID)).Return(getThreshold(), nil)
	mockDao.EXPECT().SaveThreshold(getThreshold()).Return(errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_FindCircuitBreakerByIdReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(nil, errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_FindThresholdByIdReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(getDomainCluster(), nil)
	mockDao.EXPECT().FindCircuitBreakerById(int32(CIRCUIT_BREAKER_ID)).Return(getCircuitBreaker(), nil)
	mockDao.EXPECT().FindThresholdById(int32(THRESHOLD_ID)).Return(nil, errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_FindClusterWithoutCircuitBreaker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	cluster := getDomainCluster()
	cluster.CircuitBreakerId = 0

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(cluster, nil)
	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(cluster, nil)
	mockDao.EXPECT().SaveThreshold(gomock.Any()).Return(nil)
	mockDao.EXPECT().SaveCircuitBreaker(gomock.Any()).Return(nil)
	mockDao.EXPECT().SaveCluster(gomock.Any()).Return(nil)
	mockDao.EXPECT().FindClustersNodeGroup(gomock.Any()).Return(&domain.ClustersNodeGroup{}, nil)
	mockDao.EXPECT().FindDeploymentVersionsByStage(domain.ActiveStage).Return([]*domain.DeploymentVersion{{Version: "v1", Stage: domain.ActiveStage}}, nil)
	mockDao.EXPECT().FindEndpointsByClusterName(gomock.Any()).Return([]*domain.Endpoint{}, nil)
	mockDao.EXPECT().FindEndpointsByClusterIdAndDeploymentVersion(gomock.Any(), gomock.Any()).Return([]*domain.Endpoint{}, nil)
	mockDao.EXPECT().SaveEndpoint(gomock.Any())
	mockDao.EXPECT().SaveEnvoyConfigVersion(gomock.Any()).Return(nil)

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_FindTlsConfigReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(nil, errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_AddClusterDaoProvided_RebaseClusterReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	mockDao.EXPECT().FindTlsConfigByName(TLS_CONFIG_NAME).Return(&domain.TlsConfig{
		Id:        TLS_ID,
		TrustedCA: TRUSTED_CA,
	}, nil)

	mockDao.EXPECT().FindClusterByName(CLUSTER_NAME).Return(nil, errors.NewError(errors.ErrorCode{Code: "500"}, "testError", nil))

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	clusterSrv.AddClusterDaoProvided(context.Background(), mockDao, GATEWAY_NAME, getDtoCluster(false))
}

func TestService_SingleOverriddenWithTrueValueForRoutingConfigRequestV3(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)

	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := NewClusterService(entitySrv, mockDao, bus)

	specs := getDtoCluster(true)

	isOverridden := clusterSrv.GetClusterResource().GetDefinition().IsOverriddenByCR(nil, nil, specs)
	assert.True(t, isOverridden)
}

func getDtoCluster(overridden bool) *dto.ClusterConfigRequestV3 {
	endpoints := []dto.RawEndpoint{
		"http://test.t",
	}
	circuitBreaker := dto.CircuitBreaker{
		dto.Threshold{
			MaxConnections: 2,
		},
	}
    connectionIdleTimeout := int64(10)
	return &dto.ClusterConfigRequestV3{
		Gateways:       []string{GATEWAY_NAME},
		Name:           CLUSTER_NAME,
		Endpoints:      endpoints,
		CircuitBreaker: circuitBreaker,
		TLS:            TLS_CONFIG_NAME,
		Overridden:     overridden,
		MaxRequestsPerConnection:  100,
		ConnectionIdleTimeout:     &connectionIdleTimeout,
	}
}

func getDomainCluster() *domain.Cluster {
	return &domain.Cluster{
		Name: CLUSTER_NAME,
		TLS: &domain.TlsConfig{
			BaseModel:  bun.BaseModel{},
			Id:         TLS_ID,
			NodeGroups: nil,
			Name:       CLUSTER_NAME,
			Enabled:    true,
			TrustedCA:  TRUSTED_CA,
			SNI:        "",
		},
		Endpoints: []*domain.Endpoint{
			{
				Address: "control-plane",
			},
			{
				Address: "test-service",
			},
		},
		CircuitBreakerId: CIRCUIT_BREAKER_ID,
	}
}

func getThreshold() *domain.Threshold {
	return &domain.Threshold{
		Id:             THRESHOLD_ID,
		MaxConnections: 2,
	}
}

func getCircuitBreaker() *domain.CircuitBreaker {
	return &domain.CircuitBreaker{
		Id:          CIRCUIT_BREAKER_ID,
		ThresholdId: THRESHOLD_ID,
	}
}
