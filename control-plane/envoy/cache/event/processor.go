package event

import (
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
)

type compositeUpdateBuilder struct {
	repo                dao.Repository
	actions             ActionsByNodeGroup
	entityVersions      nodeGroupEntityVersions
	envoyConfigBuilder  builder.EnvoyConfigBuilder
	updateActionFactory map[action.EnvoyEntity]actionFactoryFunc
	deleteActionFactory map[action.EnvoyEntity]actionFactoryFunc

	nodeGroupBeforeActions map[string][]action.SnapshotUpdateAction
}

func newCompositeUpdateBuilder(repo dao.Repository, versionsByNodeGroup nodeGroupEntityVersions, envoyConfigBuilder builder.EnvoyConfigBuilder, actionFactory action.UpdateActionFactory) *compositeUpdateBuilder {
	return &compositeUpdateBuilder{
		repo:               repo,
		actions:            NewActionsByNodeGroupMap(),
		entityVersions:     versionsByNodeGroup,
		envoyConfigBuilder: envoyConfigBuilder,
		updateActionFactory: map[action.EnvoyEntity]actionFactoryFunc{
			action.EnvoyCluster:     actionFactory.ClusterUpdate,
			action.EnvoyRouteConfig: actionFactory.RouteConfigUpdate,
			action.EnvoyListener:    actionFactory.ListenerUpdate,
			action.EnvoyRuntime:     actionFactory.RuntimeUpdate,
		},
		deleteActionFactory: map[action.EnvoyEntity]actionFactoryFunc{
			action.EnvoyCluster:     actionFactory.ClusterDelete,
			action.EnvoyRouteConfig: actionFactory.RouteConfigDelete,
			action.EnvoyListener:    actionFactory.ListenerDelete,
		},
		nodeGroupBeforeActions: make(map[string][]action.SnapshotUpdateAction),
	}
}

func (builder *compositeUpdateBuilder) addBeforeAction(nodeGroup string, beforeAction action.SnapshotUpdateAction) {
	if actions, exist := builder.nodeGroupBeforeActions[nodeGroup]; exist {
		actions = append(actions, beforeAction)
	} else {
		builder.nodeGroupBeforeActions[nodeGroup] = make([]action.SnapshotUpdateAction, 0)
		builder.nodeGroupBeforeActions[nodeGroup] = append(builder.nodeGroupBeforeActions[nodeGroup], beforeAction)
	}
}

type actionFactoryFunc func(nodeGroup, version string, entity interface{}) action.GranularEntityUpdate

//go:generate mockgen -source=processor.go -destination=../../../test/mock/envoy/cache/event/stub_processor.go -package=mock_event -imports memdb=github.com/hashicorp/go-memdb
type ActionsByNodeGroup interface {
	Put(nodeGroup string, envoyEntityType action.EnvoyEntity, granularUpdate action.GranularEntityUpdate)
	GetActionsByNode() map[string]action.ActionsMap
}

type actionsByNodeGroupMap struct {
	actionsByNode map[string]action.ActionsMap // map with nodeGroup -> entityType -> entityId keys is needed to avoid duplicated actions
}

func NewActionsByNodeGroupMap() *actionsByNodeGroupMap {
	return &actionsByNodeGroupMap{actionsByNode: make(map[string]action.ActionsMap)}
}

func (actions *actionsByNodeGroupMap) Put(nodeGroup string, envoyEntityType action.EnvoyEntity, granularUpdate action.GranularEntityUpdate) {
	if nodeGroupActions, exist := actions.actionsByNode[nodeGroup]; exist {
		nodeGroupActions.Put(envoyEntityType, &granularUpdate)
	} else {
		updActionsMap := action.NewUpdateActionsMap()
		updActionsMap.Put(envoyEntityType, &granularUpdate)
		actions.actionsByNode[nodeGroup] = updActionsMap
	}
}

func (actions *actionsByNodeGroupMap) GetActionsByNode() map[string]action.ActionsMap {
	return actions.actionsByNode
}

type nodeGroupEntityVersions map[string]map[action.EnvoyEntity]string

func (versionsMap nodeGroupEntityVersions) put(envoyConfigVersion *domain.EnvoyConfigVersion) {
	if _, exists := versionsMap[envoyConfigVersion.NodeGroup]; !exists {
		versionsMap[envoyConfigVersion.NodeGroup] = make(map[action.EnvoyEntity]string, 3)
	}
	entityType := action.EnvoyEntityByTable(envoyConfigVersion.EntityType)
	versionsMap[envoyConfigVersion.NodeGroup][entityType] = common.VersionToString(envoyConfigVersion.Version)
}

func (versionsMap nodeGroupEntityVersions) getVersion(nodeGroup string, envoyEntity action.EnvoyEntity) string {
	if entityVersions, exist := versionsMap[nodeGroup]; exist {
		if version, exists := entityVersions[envoyEntity]; exists {
			return version
		}
	}
	logger.Debugf("Version for %s in nodeGroup %s is not specified, so this entity will not be updated in %s", envoyEntity, nodeGroup, nodeGroup)
	return ""
}

func extractVersionsFromMultipleChangeEvent(changeEvent *events.MultipleChangeEvent) nodeGroupEntityVersions {
	changes := changeEvent.Changes
	envoyConfigVersionChanges, ok := changes[domain.EnvoyConfigVersionTable]
	if !ok || len(envoyConfigVersionChanges) < 1 {
		logger.Panic("MultipleChangeEvent changes must contain at least one EnvoyConfigVersion entry")
	}
	versionsMap := make(nodeGroupEntityVersions, len(envoyConfigVersionChanges))

	for _, change := range envoyConfigVersionChanges {
		envoyConfigVersion := change.After.(*domain.EnvoyConfigVersion)
		versionsMap.put(envoyConfigVersion)
	}
	return versionsMap
}

func (builder *compositeUpdateBuilder) withReloadForVersions() *compositeUpdateBuilder {
	for nodeGroup, versionsByType := range builder.entityVersions {
		for envoyType, version := range versionsByType {
			// first, cleanup all the entries for this type because they might be already deleted from in-mem db
			builder.addBeforeAction(nodeGroup, action.NewDeleteAllByTypeAction(version, envoyType.ToTypeURL()))
			// then add update actions for all the existing entities
			switch envoyType {
			case action.EnvoyCluster:
				clusters, err := builder.repo.FindClusterByNodeGroup(&domain.NodeGroup{Name: nodeGroup})
				if err != nil {
					logger.Errorf("Could not load clusters for nodeGroup %s from in-memory storage:\n %v", nodeGroup, err)
					continue
				}
				for _, cluster := range clusters {
					builder.addUpdateAction(nodeGroup, action.EnvoyCluster, cluster)
				}
				break
			case action.EnvoyListener:
				listeners, err := builder.repo.FindListenersByNodeGroupId(nodeGroup)
				if err != nil {
					logger.Errorf("Could not load listeners for nodeGroup %s from in-memory storage:\n %v", nodeGroup, err)
					continue
				}
				for _, listener := range listeners {
					builder.addUpdateAction(nodeGroup, action.EnvoyListener, listener)
				}
				break
			case action.EnvoyRouteConfig:
				routeConfigs, err := builder.repo.FindRouteConfigsByNodeGroupId(nodeGroup)
				if err != nil {
					logger.Errorf("Could not load routeConfigs for nodeGroup %s from in-memory storage:\n %v", nodeGroup, err)
					continue
				}
				for _, routeConfig := range routeConfigs {
					builder.addUpdateAction(nodeGroup, action.EnvoyRouteConfig, routeConfig)
				}
				break
			default:
				logger.Warnf("Got unsupported envoy entity type %v in the nodeGroupEntityVersions map for nodeGroup %s", envoyType, nodeGroup)
				break
			}
		}
	}
	return builder
}

func (builder *compositeUpdateBuilder) withChanges(table string, changes []memdb.Change) {
	switch table {
	case domain.EnvoyConfigVersionTable:
		break
	case domain.NodeGroupTable:
		builder.processNodeGroupChanges(changes)
		break
	case domain.DeploymentVersionTable:
		builder.processDeploymentVersionChanges(changes)
		break
	case domain.ClusterNodeGroupTable:
		builder.processClusterNodeGroupChanges(changes)
		break
	case domain.ListenerTable:
		builder.processListenerChanges(changes)
		break
	case domain.ClusterTable:
		builder.processClusterChanges(changes)
		break
	case domain.EndpointTable:
		builder.processEndpointChanges(changes)
		break
	case domain.RouteConfigurationTable:
		builder.processRouteConfigurationChanges(changes)
		break
	case domain.VirtualHostTable:
		builder.processVirtualHostChanges(changes)
		break
	case domain.VirtualHostDomainTable:
		builder.processVirtualHostDomainChanges(changes)
		break
	case domain.RouteTable:
		builder.processRouteChanges(changes)
		break
	case domain.HeaderMatcherTable:
		builder.processHeaderMatcherChanges(changes)
		break
	case domain.HashPolicyTable:
		builder.processHashPolicyChanges(changes)
		break
	case domain.RetryPolicyTable:
		builder.processRetryPolicyChanges(changes)
		break
	case domain.HealthCheckTable:
		builder.processHealthCheckChanges(changes)
		break
	case domain.WasmFilterTable:
		builder.processWasmFilterChanges(changes)
		break
	case domain.ListenersWasmFilterTable:
		builder.processListenersWasmFilterChanges(changes)
		break
	case domain.LuaFilterTable:
		builder.processLuaFilterChanges(changes)
		break
	case domain.StatefulSessionTable:
		builder.processStatefulSessionCookieChanges(changes)
		break
	case domain.RateLimitTable:
		builder.processRateLimitChanges(changes)
		break
	case domain.ExtAuthzFilterTable:
		builder.processExtAuthzFilterChanges(changes)
		break
	case domain.TcpKeepaliveTable:
		builder.processTcpKeepaliveChanges(changes)
		break
	default:
		logger.Warnf("Unsupported entity %v presents in change event", table)
	}
}

func (builder *compositeUpdateBuilder) build() map[string]action.SnapshotUpdateAction {
	mergedActionsByNode := make(map[string][]action.SnapshotUpdateAction)
	// first, add actions that need to be performed before granular updates
	for nodeGroup, beforeActions := range builder.nodeGroupBeforeActions {
		if len(beforeActions) > 0 {
			if mergedActionsByNode[nodeGroup] == nil {
				mergedActionsByNode[nodeGroup] = make([]action.SnapshotUpdateAction, 0)
			}
			mergedActionsByNode[nodeGroup] = append(mergedActionsByNode[nodeGroup], beforeActions...)
		}
	}

	// merge with granular update actions
	for nodeGroup, granularUpdates := range builder.actions.GetActionsByNode() {
		if mergedActionsByNode[nodeGroup] == nil {
			mergedActionsByNode[nodeGroup] = make([]action.SnapshotUpdateAction, 0)
		}
		mergedActionsByNode[nodeGroup] = append(mergedActionsByNode[nodeGroup], granularUpdates.CompositeAction())
	}

	// compose actions for each node group if necessary
	result := make(map[string]action.SnapshotUpdateAction, len(mergedActionsByNode))
	for nodeGroup, updateActions := range mergedActionsByNode {
		if len(updateActions) > 1 {
			result[nodeGroup] = action.NewCompositeUpdateAction(updateActions)
		} else {
			result[nodeGroup] = updateActions[0]
		}
	}
	return result
}

func (builder *compositeUpdateBuilder) addUpdateAction(nodeGroup string, envoyEntityType action.EnvoyEntity, entity interface{}) {
	version := builder.entityVersions.getVersion(nodeGroup, envoyEntityType)
	if version != "" {
		factoryFunc := builder.updateActionFactory[envoyEntityType]
		builder.actions.Put(nodeGroup, envoyEntityType, factoryFunc(nodeGroup, version, entity))
	}
}

func (builder *compositeUpdateBuilder) addDeleteAction(nodeGroup string, envoyEntityType action.EnvoyEntity, entity interface{}) {
	version := builder.entityVersions.getVersion(nodeGroup, envoyEntityType)
	if version != "" {
		factoryFunc := builder.deleteActionFactory[envoyEntityType]
		builder.actions.Put(nodeGroup, envoyEntityType, factoryFunc(nodeGroup, version, entity))
	}
}

func (builder *compositeUpdateBuilder) updateRuntimeConfigs(nodeGroup *domain.NodeGroup) {
	builder.addUpdateAction(nodeGroup.Name, action.EnvoyRuntime, nodeGroup)
}
