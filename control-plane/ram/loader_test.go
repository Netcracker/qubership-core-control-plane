package ram

import (
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	test_mock_constancy "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/constancy"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TODO rewrite
func TestStorageLoader_Load(t *testing.T) {
	/*dao := dao2.NewStorage("localhost:5432", "postgres", "postgres", "postgres")
	storage := NewStorage()
	type fields struct {
		Db *dao2.Storage
	}
	type args struct {
		storage *memdb.MemDB
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{name: "#1", fields: struct{ Db *dao2.Storage }{Db: dao}, args: struct{ storage *memdb.MemDB }{storage: storage.db}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &StorageLoader{
				PersistentStorage: tt.fields.Db,
			}
			if err := l.Load(tt.args.storage); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}*/
}

func Test_StorageLoader_ClearAndLoad(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	storage := test_mock_constancy.NewMockStorage(ctrl)
	ramStorage := NewMockRamStorage(ctrl)
	txn := NewMockTxn(ctrl)

	loader := StorageLoader{PersistentStorage: storage}

	storage.EXPECT().FindAllTlsConfigs().Return([]*domain.TlsConfig{}, nil)
	storage.EXPECT().FindAllNodeGroups().Return([]*domain.NodeGroup{}, nil)
	storage.EXPECT().FindAllDeploymentVersions().Return([]*domain.DeploymentVersion{}, nil)
	storage.EXPECT().FindAllClustersNodeGroups().Return([]*domain.ClustersNodeGroup{}, nil)
	storage.EXPECT().FindAllMicroserviceVersions().Return([]*domain.MicroserviceVersion{}, nil)
	storage.EXPECT().FindAllStatefulSessionConfigs().Return([]*domain.StatefulSession{}, nil)
	storage.EXPECT().FindAllListeners().Return([]*domain.Listener{}, nil)
	storage.EXPECT().FindAllEndpoints().Return([]*domain.Endpoint{}, nil)
	storage.EXPECT().FindAllRouteConfigs().Return([]*domain.RouteConfiguration{}, nil)
	storage.EXPECT().FindAllVirtualHosts().Return([]*domain.VirtualHost{}, nil)
	storage.EXPECT().FindAllVirtualHostsDomains().Return([]*domain.VirtualHostDomain{}, nil)
	storage.EXPECT().FindAllRoutes().Return([]*domain.Route{}, nil)
	storage.EXPECT().FindAllHeaderMatchers().Return([]*domain.HeaderMatcher{}, nil)
	storage.EXPECT().FindAllHashPolicies().Return([]*domain.HashPolicy{}, nil)
	storage.EXPECT().FindAllRetryPolicies().Return([]*domain.RetryPolicy{}, nil)
	storage.EXPECT().FindAllEnvoyConfigVersions().Return([]*domain.EnvoyConfigVersion{}, nil)
	storage.EXPECT().FindAllHealthChecks().Return([]*domain.HealthCheck{}, nil)
	storage.EXPECT().FindAllListenerWasmFilters().Return([]*domain.ListenersWasmFilter{}, nil)
	storage.EXPECT().FindAllCompositeSatellites().Return([]*domain.CompositeSatellite{}, nil)
	storage.EXPECT().FindWasmFilters().Return([]*domain.WasmFilter{}, nil)
	storage.EXPECT().FindAllTlsConfigsNodeGroups().Return([]*domain.TlsConfigsNodeGroups{}, nil)
	storage.EXPECT().FindAllRateLimits().Return([]*domain.RateLimit{}, nil)
	storage.EXPECT().FindAllExtAuthzFilters().Return([]*domain.ExtAuthzFilter{}, nil)
	storage.EXPECT().FindAllCircuitBreakers().Return([]*domain.CircuitBreaker{}, nil)
	storage.EXPECT().FindAllThresholds().Return([]*domain.Threshold{}, nil)
	storage.EXPECT().FindAllTcpKeepalives().Return([]*domain.TcpKeepalive{}, nil)
	storage.EXPECT().FindAllListenerLuaFilters().Return([]*domain.ListenersLuaFilter{}, nil)
	storage.EXPECT().FindLuaFilters().Return([]*domain.LuaFilter{}, nil)

	ramStorage.EXPECT().WriteTx().Return(txn)
	txn.EXPECT().Abort()
	txn.EXPECT().Commit()

	cluster := domain.NewCluster("test", false)
	storage.EXPECT().FindAllClusters().Return([]*domain.Cluster{cluster}, nil)

	txn.EXPECT().DeleteAll(gomock.Any(), gomock.Eq("id")).Times(27)
	txn.EXPECT().Insert(gomock.Eq(domain.ClusterTable), gomock.Eq(cluster)).Times(1)

	err := loader.ClearAndLoad(ramStorage)
	assert.NoError(t, err)
}

func Test_StorageLoader_ClearAndLoad_ErrorOnLoad(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	storage := test_mock_constancy.NewMockStorage(ctrl)
	ramStorage := NewMockRamStorage(ctrl)
	txn := NewMockTxn(ctrl)

	loader := StorageLoader{PersistentStorage: storage}

	errorMessage := fmt.Errorf("error during load")
	storage.EXPECT().FindAllTcpKeepalives().Return(nil, errorMessage)

	ramStorage.EXPECT().WriteTx().Return(txn)
	txn.EXPECT().Abort()

	txn.EXPECT().DeleteAll(gomock.Any(), gomock.Eq("id")).Times(27)
	txn.EXPECT().Insert(gomock.Any(), gomock.Any()).Times(0)
	txn.EXPECT().Commit().Times(0)

	err := loader.ClearAndLoad(ramStorage)
	assert.Error(t, err)
	assert.Equal(t, errorMessage, err)
}
