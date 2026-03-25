package event

import (
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
)

func (parser *changeEventParserImpl) processHashPolicyChanges(actions action.ActionsMap, entityVersions map[string]string, nodeGroup string, changes []memdb.Change) {
	for _, change := range changes {
		if change.Deleted() {
			hashPolicy := change.Before.(*domain.HashPolicy)
			parser.updateHashPolicy(actions, entityVersions, nodeGroup, hashPolicy)
		} else {
			hashPolicy := change.After.(*domain.HashPolicy)
			parser.updateHashPolicy(actions, entityVersions, nodeGroup, hashPolicy)
		}
	}
}

func (parser *changeEventParserImpl) updateHashPolicy(actions action.ActionsMap, entityVersions map[string]string, nodeGroup string, hashPolicy *domain.HashPolicy) {
	if hashPolicy.RouteId != 0 {
		parser.updateRouteHashPolicy(actions, entityVersions, nodeGroup, hashPolicy)
	}
	if hashPolicy.EndpointId != 0 {
		parser.updateEndpointHashPolicy(actions, entityVersions, nodeGroup, hashPolicy)
	}
}

func (parser *changeEventParserImpl) updateRouteHashPolicy(actions action.ActionsMap, entityVersions map[string]string, nodeGroup string, hashPolicy *domain.HashPolicy) {
	route, err := parser.dao.FindRouteById(hashPolicy.RouteId)
	if err != nil {
		logger.Panicf("Failed to find route by id using DAO: %v", err)
	}
	if route != nil {
		parser.updateRoute(actions, entityVersions, nodeGroup, route)
	}
}

func (parser *changeEventParserImpl) updateEndpointHashPolicy(actions action.ActionsMap, entityVersions map[string]string, nodeGroup string, hashPolicy *domain.HashPolicy) {
	endpoint, err := parser.dao.FindEndpointById(hashPolicy.EndpointId)
	if err != nil {
		logger.Panicf("Failed to find endpoint by id using DAO: %v", err)
	}
	if endpoint != nil {
		parser.updateEndpoint(actions, entityVersions, nodeGroup, endpoint)

		// now update all routes related to this endpoint (since their hash policies have been changed)
		routeConfigs, err := parser.dao.FindRouteConfigsByEndpoint(endpoint)
		if err != nil {
			logger.Panicf("Failed to find route configurations by endpoint using DAO: %v", err)
		}
		if routeConfigs != nil {
			for _, routeConfig := range routeConfigs {
				parser.updateRouteConfig(actions, entityVersions, nodeGroup, routeConfig)
			}
		}
	}
}

func (builder *compositeUpdateBuilder) updateRoute(routeId int32) {
	route, err := builder.repo.FindRouteById(routeId)
	if err != nil {
		logger.Panicf("Failed to load route by id using DAO: %v", err)
	}
	if route != nil { // if nil, route is deleted and its deletion is processed by domain.Route change event
		builder.updateVirtualHost(route.VirtualHostId)
	}
}

func (builder *compositeUpdateBuilder) processHashPolicyChanges(changes []memdb.Change) {
	for _, change := range changes {
		var hashPolicy *domain.HashPolicy = nil
		if change.Deleted() {
			hashPolicy = change.Before.(*domain.HashPolicy)
		} else {
			hashPolicy = change.After.(*domain.HashPolicy)
		}
		if hashPolicy.RouteId != 0 {
			builder.updateRoute(hashPolicy.RouteId)
		} else if hashPolicy.EndpointId != 0 {
			builder.updateEndpoint(hashPolicy.EndpointId)
		}
	}
}
