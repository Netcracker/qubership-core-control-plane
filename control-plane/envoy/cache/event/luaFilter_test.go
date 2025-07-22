package event

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
	mock_builder "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/envoy/cache/builder"
)

func TestCompositeUpdateBuilder_processLuaFilterChangesRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	nodeGroup := "nodeGroup"
	nodeGroupEntityVersions := map[string]map[action.EnvoyEntity]string{
		nodeGroup: {
			action.EnvoyRouteConfig: "test-version",
		},
	}
	mockUpdateAction := getMockUpdateAction(ctrl)
	mockBuilder := mock_builder.NewMockEnvoyConfigBuilder(ctrl)
	compositeUpdateBuilder := newCompositeUpdateBuilder(mockDao, nodeGroupEntityVersions, mockBuilder, mockUpdateAction)

	changes := []memdb.Change{
		{
			After: &domain.LuaFilter{
				Id:   int32(2),
				Name: "test-lua-filter",
			},
		},
	}

	routes := []*domain.Route{
		{
			Id:                int32(1),
			VirtualHostId:     1,
			RouteKey:          "/api/v1/test",
			DeploymentVersion: "v1",
			ClusterName:       "clusterName1",
			LuaFilterName:     "test-lua-filter",
		},
	}
	routeConfig := &domain.RouteConfiguration{Id: int32(1), NodeGroupId: "nodeGroup"}

	mockDao.EXPECT().FindVirtualHostById(int32(1)).Return(&domain.VirtualHost{Id: int32(1), RouteConfigurationId: int32(1)}, nil)
	mockDao.EXPECT().FindRouteConfigById(int32(1)).Return(routeConfig, nil)
	mockDao.EXPECT().FindRoutesByLuaFilter("test-lua-filter").Return(routes, nil)

	mockUpdateAction.EXPECT().RouteConfigUpdate(nodeGroup, "test-version", routeConfig)

	compositeUpdateBuilder.processLuaFilterChanges(changes)
}

func TestChangeEventParserImpl_processLuaFilterChanges_Route(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockUpdateAction := getMockUpdateAction(ctrl)
	mockBuilder := mock_builder.NewMockEnvoyConfigBuilder(ctrl)
	changeEventParser := NewChangeEventParser(mockDao, mockUpdateAction, mockBuilder)

	actions := getMockActionsMap(ctrl)
	entityVersions := map[string]string{domain.RouteConfigurationTable: "test1"}
	nodeGroup := "nodeGroup"
	changes := []memdb.Change{
		{
			After: &domain.LuaFilter{
				Id:   int32(2),
				Name: "test-lua-filter",
			},
		},
	}

	routes := []*domain.Route{
		{
			Id:                int32(1),
			VirtualHostId:     1,
			RouteKey:          "/api/v1/test",
			DeploymentVersion: "v1",
			ClusterName:       "clusterName1",
			LuaFilterName:     "test-lua-filter",
		},
	}

	routeConfig := &domain.RouteConfiguration{Id: int32(1), NodeGroupId: "nodeGroup"}

	mockDao.EXPECT().FindRoutesByLuaFilter("test-lua-filter").Return(routes, nil)
	mockDao.EXPECT().FindVirtualHostById(int32(1)).Return(&domain.VirtualHost{Id: int32(1), RouteConfigurationId: int32(1)}, nil)
	mockDao.EXPECT().FindRouteConfigById(int32(1)).Return(routeConfig, nil)

	granularUpdate := action.GranularEntityUpdate{}
	mockUpdateAction.EXPECT().RouteConfigUpdate(nodeGroup, entityVersions[domain.RouteConfigurationTable], routeConfig).Times(1).Return(granularUpdate)
	actions.EXPECT().Put(action.EnvoyRouteConfig, &granularUpdate).Times(1)

	changeEventParser.processLuaFilterChanges(actions, entityVersions, nodeGroup, changes)
}