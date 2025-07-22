package event

import (
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
)

func (parser *changeEventParserImpl) processLuaFilterChanges(actions action.ActionsMap, entityVersions map[string]string, nodeGroup string, changes []memdb.Change) {
	logger.Debug("Processing lua filter multiple change event")
	for _, change := range changes {
		if change.Deleted() {
			luaFilter := change.Before.(*domain.LuaFilter)
			parser.updateLuaFilter(actions, entityVersions, nodeGroup, luaFilter)
		} else {
			luaFilter := change.After.(*domain.LuaFilter)
			parser.updateLuaFilter(actions, entityVersions, nodeGroup, luaFilter)
		}
	}
}

func (parser *changeEventParserImpl) updateLuaFilter(actions action.ActionsMap, entityVersions map[string]string, nodeGroup string, luaFilter *domain.LuaFilter) {
	vHostsToUpdate, err := findVirtualHostsToUpdateByLuaFilter(parser.dao, luaFilter)
	if err != nil {
		logger.Panicf("Could not find virtual hosts to update by Lua filter change:\n %v", err)
	}
	for vHostId := range vHostsToUpdate {
		vHost, err := parser.dao.FindVirtualHostById(vHostId)
		if err != nil {
			logger.Panicf("Could not find virtual host by id to update with Lua filter change:\n %v", err)
		}
		parser.updateVirtualHost(actions, entityVersions, nodeGroup, vHost)
	}
	
}

func (builder *compositeUpdateBuilder) processLuaFilterChanges(changes []memdb.Change) {
	logger.Debug("Processing lua filter multiple change event")
	for _, change := range changes {
		var luaFilter *domain.LuaFilter = nil
		if change.Deleted() {
			luaFilter = change.Before.(*domain.LuaFilter)
		} else {
			luaFilter = change.After.(*domain.LuaFilter)
		}
		builder.updateLuaFilter(luaFilter)
	}
}

func (builder *compositeUpdateBuilder) updateLuaFilter(luaFilter *domain.LuaFilter) {
	vHostsToUpdate, err := findVirtualHostsToUpdateByLuaFilter(builder.repo, luaFilter)
	if err != nil {
		logger.Panicf("Could not find virtual hosts to update by lua filter change:\n %v", err)
	}
	for vHostId := range vHostsToUpdate {
		builder.updateVirtualHost(vHostId)
	}
}

func findVirtualHostsToUpdateByLuaFilter(repo dao.Repository, luaFilter *domain.LuaFilter) (map[int32]bool, error) {
	virtualHostsToUpdate := make(map[int32]bool)

	routes, err := repo.FindRoutesByLuaFilter(luaFilter.Name)
	if err != nil {
		logger.Errorf("Failed to find routes by Lua filter %s using DAO:\n %v", luaFilter.Name, err)
		return nil, err
	}
	for _, route := range routes {
		virtualHostsToUpdate[route.VirtualHostId] = true
		
	}
	return virtualHostsToUpdate, nil
}