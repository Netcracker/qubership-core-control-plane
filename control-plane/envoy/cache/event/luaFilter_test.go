package event

import (
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
	mock_builder "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/envoy/cache/builder"
	"testing"
)

func TestCompositeUpdateBuilder_processListenersLuaFilterChangesprocessLuaFilterChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	nodeGroupId1 := "nodeId1"
	nodeGroupId2 := "nodeId2"
	versionsByNodeGroup := nodeGroupEntityVersions{
		nodeGroupId1: {
			action.EnvoyListener: "test1",
		},
		nodeGroupId2: {
			action.EnvoyListener: "test2",
		},
	}
	mockUpdateAction := getMockUpdateAction(ctrl)
	mockBuilder := mock_builder.NewMockEnvoyConfigBuilder(ctrl)
	compositeUpdateBuilder := newCompositeUpdateBuilder(mockDao, versionsByNodeGroup, mockBuilder, mockUpdateAction)

	listeners := []*domain.Listener{
		{
			Id:          int32(1),
			Name:        "listener1",
			NodeGroupId: nodeGroupId1,
		},
		{
			Id:          int32(2),
			Name:        "listener2",
			NodeGroupId: nodeGroupId2,
		},
	}

	changes := []memdb.Change{
		{
			Before: &domain.ListenersLuaFilter{
				ListenerId: listeners[0].Id,
			},
			After: &domain.ListenersLuaFilter{
				ListenerId: listeners[1].Id,
			},
		},
	}

	mockDao.EXPECT().FindListenerById(changes[0].After.(*domain.ListenersLuaFilter).ListenerId).Return(listeners[0], nil)
	mockUpdateAction.EXPECT().ListenerUpdate(nodeGroupId1, versionsByNodeGroup[nodeGroupId1][action.EnvoyListener], gomock.Any())

	compositeUpdateBuilder.processListenersLuaFilterChanges(changes)
}

func TestCompositeUpdateBuilder_processLuaFilterChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	nodeGroupId1 := "nodeId1"
	nodeGroupId2 := "nodeId2"
	versionsByNodeGroup := nodeGroupEntityVersions{
		nodeGroupId1: {
			action.EnvoyListener: "test1",
		},
		nodeGroupId2: {
			action.EnvoyListener: "test2",
		},
	}
	mockUpdateAction := getMockUpdateAction(ctrl)
	mockBuilder := mock_builder.NewMockEnvoyConfigBuilder(ctrl)
	compositeUpdateBuilder := newCompositeUpdateBuilder(mockDao, versionsByNodeGroup, mockBuilder, mockUpdateAction)

	changes := []memdb.Change{
		{
			Before: &domain.LuaFilter{
				Id:   int32(1),
				Name: "before",
			},
			After: &domain.LuaFilter{
				Id:   int32(2),
				Name: "after",
			},
		},
	}

	luaFilterIds := []int32{int32(1), int32(2)}
	mockDao.EXPECT().FindListenerIdsByLuaFilterId(changes[0].After.(*domain.LuaFilter).Id).Return(luaFilterIds, nil)

	listeners := []*domain.Listener{
		{
			Id:          int32(1),
			Name:        "listener1",
			NodeGroupId: nodeGroupId1,
		},
		{
			Id:          int32(2),
			Name:        "listener2",
			NodeGroupId: nodeGroupId2,
		},
	}
	mockDao.EXPECT().FindListenerById(luaFilterIds[0]).Return(listeners[0], nil)
	mockDao.EXPECT().FindListenerById(luaFilterIds[1]).Return(listeners[1], nil)
	mockUpdateAction.EXPECT().ListenerUpdate(nodeGroupId1, versionsByNodeGroup[nodeGroupId1][action.EnvoyListener], gomock.Any())
	mockUpdateAction.EXPECT().ListenerUpdate(nodeGroupId2, versionsByNodeGroup[nodeGroupId2][action.EnvoyListener], gomock.Any())

	compositeUpdateBuilder.processLuaFilterChanges(changes)
}

func TestChangeEventParser_processLuaFilterChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := getMockDao(ctrl)
	mockUpdateAction := getMockUpdateAction(ctrl)
	mockBuilder := mock_builder.NewMockEnvoyConfigBuilder(ctrl)
	changeEventParser := NewChangeEventParser(mockDao, mockUpdateAction, mockBuilder)

	actions := getMockActionsMap(ctrl)
	entityVersions := map[string]string{
		domain.ListenerTable: "test",
	}
	nodeGroup := "nodeGroup"
	listeners := []*domain.Listener{{}}

	mockDao.EXPECT().FindListenersByNodeGroupId(nodeGroup).Return(listeners, nil)
	granularEntityUpdate := action.GranularEntityUpdate{}
	mockUpdateAction.EXPECT().ListenerUpdate(nodeGroup, entityVersions[domain.ListenerTable], listeners[0]).Return(granularEntityUpdate)
	actions.EXPECT().Put(action.EnvoyListener, &granularEntityUpdate)

	changeEventParser.processLuaFilterChanges(actions, entityVersions, nodeGroup)
}
