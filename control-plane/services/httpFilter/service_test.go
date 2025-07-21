package httpFilter

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter/extAuthz"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	mock_bus "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/event/bus"
	mock_extAuthz "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/services/httpFilter/extAuthz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
)

const (
	nodeGroup     = "test-node-group"
	clusterName   = "secure.test-cluster"
	filterName    = "test-filter"
	filterSHA256  = "123456789"
	filterTimeout = int64(10)
)

func TestService_Resources(t *testing.T) {
	srv, _ := getService()

	resAdd := srv.GetHttpFiltersResourceAdd()
	assert.NotNil(t, resAdd)

	resDrop := srv.GetHttpFiltersResourceDrop()
	assert.NotNil(t, resDrop)
}

func TestService_ValidateApply(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	extAuthzSrv := mock_extAuthz.NewMockService(ctrl)
	srv, _ := getService(extAuthzSrv)

	extAuthzFilter := dto.ExtAuthz{
		Name:        "text-extAuthz",
		Destination: dto.RouteDestination{Cluster: "test-srv", Endpoint: "grpc://test-srv:8080"},
	}

	isValid, msg := srv.ValidateApply(context.Background(), &dto.HttpFiltersConfigRequestV3{
		Gateways:       nil,
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.False(t, isValid)
	assert.NotEmpty(t, msg)

	extAuthzSrv.EXPECT().ValidateApply(context.Background(), extAuthzFilter, "test-gw").Return(false, "err msg")
	isValid, msg = srv.ValidateApply(context.Background(), &dto.HttpFiltersConfigRequestV3{
		Gateways:       []string{"test-gw"},
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.False(t, isValid)
	assert.Equal(t, "err msg", msg)

	extAuthzSrv.EXPECT().ValidateApply(context.Background(), extAuthzFilter, "test-gw").Return(true, "")
	isValid, msg = srv.ValidateApply(context.Background(), &dto.HttpFiltersConfigRequestV3{
		Gateways:       []string{"test-gw"},
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.True(t, isValid)
	assert.Empty(t, msg)
}

func TestService_ValidateDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	extAuthzSrv := mock_extAuthz.NewMockService(ctrl)
	srv, _ := getService(extAuthzSrv)

	extAuthzFilter := dto.ExtAuthz{
		Name:        "text-extAuthz",
		Destination: dto.RouteDestination{Cluster: "test-srv", Endpoint: "grpc://test-srv:8080"},
	}

	isValid, msg := srv.ValidateDelete(context.Background(), &dto.HttpFiltersDropConfigRequestV3{
		Gateways:       nil,
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.False(t, isValid)
	assert.NotEmpty(t, msg)

	extAuthzSrv.EXPECT().ValidateDelete(context.Background(), extAuthzFilter, "test-gw").Return(false, "err msg")
	isValid, msg = srv.ValidateDelete(context.Background(), &dto.HttpFiltersDropConfigRequestV3{
		Gateways:       []string{"test-gw"},
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.False(t, isValid)
	assert.Equal(t, "err msg", msg)

	extAuthzSrv.EXPECT().ValidateDelete(context.Background(), extAuthzFilter, "test-gw").Return(true, "")
	isValid, msg = srv.ValidateDelete(context.Background(), &dto.HttpFiltersDropConfigRequestV3{
		Gateways:       []string{"test-gw"},
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.True(t, isValid)
	assert.Empty(t, msg)
}

func TestService_OverriddenTrueAndNotApplyHttpFilterConfig(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)
	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := cluster.NewClusterService(entitySrv, mockDao, bus)
	extAuthzService := mock_extAuthz.NewMockService(ctrl)
	srv := NewWasmFilterService(mockDao, bus, clusterSrv, entitySrv, extAuthzService)
	wasmFilter := dto.WasmFilter{
		Name:    "test-wasm",
		URL:     "http://test-url:80",
		SHA256:  "test-sha",
		Timeout: 10000,
		Params:  []map[string]any{{"param1": "val1"}},
	}
	extAuthzFilter := dto.ExtAuthz{
		Name:        "text-extAuthz",
		Destination: dto.RouteDestination{Cluster: "test-srv", Endpoint: "grpc://test-srv:8080"},
	}
	req := &dto.HttpFiltersConfigRequestV3{
		Gateways:       []string{"test-gw"},
		WasmFilters:    []dto.WasmFilter{wasmFilter},
		ExtAuthzFilter: &extAuthzFilter,
		Overridden:     true,
	}
	isOverridden := srv.GetHttpFiltersResourceAdd().GetDefinition().IsOverriddenByCR(nil, nil, req)
	assert.True(t, isOverridden)
}

func TestService_OverriddenTrueAndNotApplyDropHttpFilterConfig(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)
	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := cluster.NewClusterService(entitySrv, mockDao, bus)
	extAuthzService := mock_extAuthz.NewMockService(ctrl)
	srv := NewWasmFilterService(mockDao, bus, clusterSrv, entitySrv, extAuthzService)
	wasmFilter := dto.WasmFilter{
		Name:    "test-wasm",
		URL:     "http://test-url:80",
		SHA256:  "test-sha",
		Timeout: 10000,
		Params:  []map[string]any{{"param1": "val1"}},
	}
	extAuthzFilter := dto.ExtAuthz{
		Name:        "text-extAuthz",
		Destination: dto.RouteDestination{Cluster: "test-srv", Endpoint: "grpc://test-srv:8080"},
	}
	req := &dto.HttpFiltersDropConfigRequestV3{
		Gateways:       []string{"test-gw"},
		WasmFilters:    []map[string]any{{"name": wasmFilter.Name}},
		ExtAuthzFilter: &extAuthzFilter,
		Overridden:     true,
	}

	isOverridden := srv.GetHttpFiltersResourceDrop().GetDefinition().IsOverriddenByCR(nil, nil, req)
	assert.True(t, isOverridden)
}

func TestService_ApplyGetDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	extAuthzSrv := mock_extAuthz.NewMockService(ctrl)
	srv, inMemDao := getService(extAuthzSrv)

	inMemDao.WithWTx(func(dao dao.Repository) error {
		err := dao.SaveNodeGroup(&domain.NodeGroup{Name: "test-gw"})
		assert.Nil(t, err)
		err = dao.SaveListener(&domain.Listener{Id: 1, Name: "test-listener", NodeGroupId: "test-gw"})
		assert.Nil(t, err)
		return nil
	})

	wasmFilter := dto.WasmFilter{
		Name:    "test-wasm",
		URL:     "http://test-url:80",
		SHA256:  "test-sha",
		Timeout: 10000,
		Params:  []map[string]any{{"param1": "val1"}},
	}
	extAuthzFilter := dto.ExtAuthz{
		Name:        "text-extAuthz",
		Destination: dto.RouteDestination{Cluster: "test-srv", Endpoint: "grpc://test-srv:8080"},
	}
	req := &dto.HttpFiltersConfigRequestV3{
		Gateways:       []string{"test-gw"},
		WasmFilters:    []dto.WasmFilter{wasmFilter},
		ExtAuthzFilter: &extAuthzFilter,
	}

	extAuthzSrv.EXPECT().Apply(context.Background(), extAuthzFilter, "test-gw").Return(errors.New("expected err in test"))

	err := srv.Apply(context.Background(), req)
	assert.NotNil(t, err)

	extAuthzSrv.EXPECT().Apply(context.Background(), extAuthzFilter, "test-gw").Return(nil)

	err = srv.Apply(context.Background(), req)
	assert.Nil(t, err)

	extAuthzSrv.EXPECT().Get(context.Background(), "test-gw").Return(&extAuthzFilter, errors.New("expected err in test"))

	filters, err := srv.GetGatewayFilters(context.Background(), "test-gw")
	assert.NotNil(t, err)

	extAuthzSrv.EXPECT().Get(context.Background(), "test-gw").Return(&extAuthzFilter, nil)

	filters, err = srv.GetGatewayFilters(context.Background(), "test-gw")
	assert.Nil(t, err)
	assert.Equal(t, *req, filters)

	extAuthzSrv.EXPECT().Delete(context.Background(), extAuthzFilter, "test-gw").Return(errors.New("expected err in test"))

	err = srv.Delete(context.Background(), &dto.HttpFiltersDropConfigRequestV3{
		Gateways:       []string{"test-gw"},
		WasmFilters:    []map[string]any{{"name": wasmFilter.Name}},
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.NotNil(t, err)

	extAuthzSrv.EXPECT().Delete(context.Background(), extAuthzFilter, "test-gw").Return(nil)

	err = srv.Delete(context.Background(), &dto.HttpFiltersDropConfigRequestV3{
		Gateways:       []string{"test-gw"},
		WasmFilters:    []map[string]any{{"name": wasmFilter.Name}},
		ExtAuthzFilter: &extAuthzFilter,
	})
	assert.Nil(t, err)

	extAuthzSrv.EXPECT().Get(context.Background(), "test-gw").Return(nil, nil)
	filters, err = srv.GetGatewayFilters(context.Background(), "test-gw")
	assert.Nil(t, err)
	assert.Equal(t, dto.HttpFiltersConfigRequestV3{Gateways: []string{"test-gw"}, WasmFilters: []dto.WasmFilter{}}, filters)
}

func TestNewListenerService_ClusterCreated(t *testing.T) {
	filterURL := "https://secure.test"
	service, inMemDao := getService()
	addListener(inMemDao, nodeGroup)
	defer deleteListener(inMemDao, nodeGroup)
	err := service.AddWasmFilter(nil,
		nodeGroup,
		[]dto.WasmFilter{
			{Name: filterName, URL: filterURL, SHA256: filterSHA256, Timeout: filterTimeout},
		},
	)

	require.Nil(t, err)
	listeners, err := inMemDao.FindListenersByNodeGroupId(nodeGroup)
	require.Nil(t, err)
	require.Len(t, listeners, 1)
	listener := listeners[0]

	wasmFilters, err := inMemDao.FindWasmFilterByListenerId(listener.Id)
	require.Nil(t, err)
	require.Len(t, wasmFilters, 1)
	wasmFilter := wasmFilters[0]
	require.Equal(t, filterName, wasmFilter.Name)
	filterCluster, err := wasmFilter.Cluster()
	require.Nil(t, err)
	require.Equal(t, clusterName, filterCluster)
	require.Equal(t, filterURL, wasmFilter.URL)
	require.Equal(t, filterSHA256, wasmFilter.SHA256)
	require.Equal(t, filterTimeout, wasmFilter.Timeout)

	createdCluster, err := inMemDao.FindClusterByName(clusterName)
	require.NotNil(t, createdCluster)
	deleteCluster(inMemDao, createdCluster.Name)
}

func TestNewListenerService_HTTPS(t *testing.T) {
	service, inMemDao := getService()
	addListener(inMemDao, nodeGroup)
	defer deleteListener(inMemDao, nodeGroup)
	err := service.AddWasmFilter(nil,
		nodeGroup,
		[]dto.WasmFilter{
			{Name: filterName, URL: "https://secure.test", SHA256: filterSHA256, Timeout: filterTimeout},
		},
	)
	createdCluster, err := inMemDao.FindClusterByName(clusterName)
	require.Nil(t, err)
	require.NotNil(t, createdCluster)
	tlsConfig, err := inMemDao.FindTlsConfigById(createdCluster.TLSId)
	require.Nil(t, err)
	require.True(t, tlsConfig.Enabled)
	require.False(t, tlsConfig.Insecure)
	deleteCluster(inMemDao, createdCluster.Name)
}

func TestNewListenerService_HTTP(t *testing.T) {
	service, inMemDao := getService()
	addListener(inMemDao, nodeGroup)
	defer deleteListener(inMemDao, nodeGroup)
	err := service.AddWasmFilter(nil,
		nodeGroup,
		[]dto.WasmFilter{
			{Name: filterName, URL: "http://insecure.test", SHA256: filterSHA256, Timeout: filterTimeout},
		},
	)
	createdCluster, err := inMemDao.FindClusterByName("insecure.test-cluster")
	require.Nil(t, err)
	require.NotNil(t, createdCluster)
	require.Nil(t, createdCluster.TLS)
	deleteCluster(inMemDao, createdCluster.Name)
}

func TestNewListenerService_No_Cluster_Name(t *testing.T) {
	service, inMemDao := getService()
	addListener(inMemDao, nodeGroup)
	defer deleteListener(inMemDao, nodeGroup)
	err := service.AddWasmFilter(nil,
		nodeGroup,
		[]dto.WasmFilter{
			{Name: filterName, URL: "http://insecure.test", SHA256: filterSHA256, Timeout: filterTimeout},
		},
	)
	createdCluster, err := inMemDao.FindClusterByName("insecure.test-cluster")
	require.Nil(t, err)
	require.NotNil(t, createdCluster)
	require.Equal(t, "insecure.test-cluster", createdCluster.Name)
	deleteCluster(inMemDao, createdCluster.Name)
}

func NewService(dao dao.Dao, bus bus.BusPublisher, clusterService *cluster.Service, extAuthzService ...extAuthz.Service) *Service {
	srv := &Service{
		dao:            dao,
		bus:            bus,
		clusterService: clusterService,
		entityService:  entity.NewService("v1"),
	}
	if len(extAuthzService) > 0 {
		srv.extAuthzService = extAuthzService[0]
	}
	return srv
}

func addListener(inMemDao dao.Dao, nodeGroupId string) {
	_, _ = inMemDao.WithWTx(func(dao dao.Repository) error {
		_ = dao.SaveListener(&domain.Listener{
			Id:          1,
			Name:        "test-listener",
			NodeGroupId: nodeGroupId,
		})
		return nil
	})
}

func deleteListener(inMemDao dao.Dao, nodeGroupId string) {
	_, _ = inMemDao.WithWTx(func(dao dao.Repository) error {
		_ = dao.DeleteListenerByNodeGroupName(nodeGroupId)
		return nil
	})
}

func deleteCluster(inMemDao dao.Dao, clusterName string) {
	_, _ = inMemDao.WithWTx(func(dao dao.Repository) error {
		_ = dao.DeleteClusterByName(clusterName)
		return nil
	})
}

func getService(extAuthzSrv ...extAuthz.Service) (*Service, *dao.InMemDao) {
	entityService := entity.NewService("v1")
	inMemStorage := ram.NewStorage()
	internalBus := bus.GetInternalBusInstance()
	eventBus := bus.NewEventBusAggregator(inMemStorage, internalBus, internalBus, nil, nil)
	genericDao := dao.NewInMemDao(inMemStorage, &idGeneratorMock{}, []func([]memdb.Change) error{flushChanges})
	clusterService := cluster.NewClusterService(entityService, genericDao, eventBus)

	service := NewService(genericDao, eventBus, clusterService, extAuthzSrv...)
	_, _ = genericDao.WithWTx(func(dao dao.Repository) error {
		_ = dao.SaveDeploymentVersion(&domain.DeploymentVersion{
			Version: "v1",
			Stage:   domain.ActiveStage,
		})
		return nil
	})
	return service, genericDao
}

func flushChanges(_ []memdb.Change) error {
	return nil
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
