package event

import (
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
)

func (parser *changeEventParserImpl) processLuaFilterChanges(actions action.ActionsMap, entityVersions map[string]string, nodeGroup string) {
	listeners, err := parser.dao.FindListenersByNodeGroupId(nodeGroup)
	if err != nil {
		logger.Panicf("failed to find listeners using DAO: %v", err)
	}

	for _, listener := range listeners {
		granularUpdate := parser.updateActionFactory.ListenerUpdate(nodeGroup, entityVersions[domain.ListenerTable], listener)
		actions.Put(action.EnvoyListener, &granularUpdate)
	}
}

func (builder *compositeUpdateBuilder) processLuaFilterChanges(changes []memdb.Change) {
	for _, change := range changes {
		var luaFilter *domain.LuaFilter = nil
		if change.Deleted() {
			luaFilter = change.Before.(*domain.LuaFilter)
		} else {
			luaFilter = change.After.(*domain.LuaFilter)
		}
		listenerIds, err := builder.repo.FindListenerIdsByLuaFilterId(luaFilter.Id)
		if err != nil {
			logger.Panicf("failed to find listeners using DAO: %v", err)
		}
		for _, listenerId := range listenerIds {
			listener, err := builder.repo.FindListenerById(listenerId)
			if err != nil {
				logger.Panicf("failed to find listener by id using DAO: %v", err)
			}
			builder.addUpdateAction(listener.NodeGroupId, action.EnvoyListener, listener)
		}
	}
}

func (builder *compositeUpdateBuilder) processListenersLuaFilterChanges(changes []memdb.Change) {
	for _, change := range changes {
		var listenersLuaFilter *domain.ListenersLuaFilter = nil
		if change.Deleted() {
			listenersLuaFilter = change.Before.(*domain.ListenersLuaFilter)
		} else {
			listenersLuaFilter = change.After.(*domain.ListenersLuaFilter)
		}
		listener, err := builder.repo.FindListenerById(listenersLuaFilter.ListenerId)
		if err != nil {
			logger.Panicf("failed to find listener by id using DAO: %v", err)
		}
		builder.addUpdateAction(listener.NodeGroupId, action.EnvoyListener, listener)
	}
}
