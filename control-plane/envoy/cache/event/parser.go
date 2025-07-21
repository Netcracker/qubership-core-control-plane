package event

import (
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/action"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var (
	logger logging.Logger
)

func init() {
	logger = logging.GetLogger("change-event-parser")
}

//go:generate mockgen -source=parser.go -destination=../../../test/mock/envoy/cache/event/stub_parser.go -package=mock_event -imports memdb=github.com/hashicorp/go-memdb
type ChangeEventParser interface {
	ParseChangeEvent(changeEvent *events.ChangeEvent) action.ActionsMap
	ParsePartialReloadEvent(changeEvent *events.PartialReloadEvent) map[string]action.SnapshotUpdateAction
	ParseMultipleChangeEvent(changeEvent *events.MultipleChangeEvent) map[string]action.SnapshotUpdateAction
}

type changeEventParserImpl struct {
	dao                 dao.Repository
	updateActionFactory action.UpdateActionFactory
	configBuilder       builder.EnvoyConfigBuilder
}

func NewChangeEventParser(dao dao.Repository, updateActionFactory action.UpdateActionFactory, configBuilder builder.EnvoyConfigBuilder) *changeEventParserImpl {
	return &changeEventParserImpl{dao: dao, updateActionFactory: updateActionFactory, configBuilder: configBuilder}
}

func (parser *changeEventParserImpl) ParseChangeEvent(changeEvent *events.ChangeEvent) action.ActionsMap {
	nodeGroup := changeEvent.NodeGroup

	actions := action.NewUpdateActionsMap()
	for table, changes := range changeEvent.Changes {
		parser.processChange(actions, make(map[string]string, 0), nodeGroup, table, changes)
	}
	return actions
}

func (parser *changeEventParserImpl) ParsePartialReloadEvent(changeEvent *events.PartialReloadEvent) map[string]action.SnapshotUpdateAction {
	versionsByNodeGroup := make(nodeGroupEntityVersions, len(changeEvent.EnvoyVersions))
	for _, envoyConfigVersion := range changeEvent.EnvoyVersions {
		versionsByNodeGroup.put(envoyConfigVersion)
	}

	actionsByNode := newCompositeUpdateBuilder(parser.dao, versionsByNodeGroup, parser.configBuilder, parser.updateActionFactory).
		withReloadForVersions().
		build()
	return actionsByNode
}

// ParseMultipleChangeEvent parses MultipleChangeEvent which is an event with changes for multiple node groups.
func (parser *changeEventParserImpl) ParseMultipleChangeEvent(changeEvent *events.MultipleChangeEvent) map[string]action.SnapshotUpdateAction {
	versionsByNodeGroup := extractVersionsFromMultipleChangeEvent(changeEvent)

	builder := newCompositeUpdateBuilder(parser.dao, versionsByNodeGroup, parser.configBuilder, parser.updateActionFactory)
	for table, changes := range changeEvent.Changes {
		builder.withChanges(table, changes)
	}
	return builder.build()
}

func (parser *changeEventParserImpl) processChange(actions action.ActionsMap, entityVersions map[string]string, nodeGroup, table string, changes []memdb.Change) {
	switch table {
	case domain.EnvoyConfigVersionTable:
		break
	case domain.NodeGroupTable:
		parser.processNodeGroupChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.DeploymentVersionTable:
		parser.processDeploymentVersionChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.ClusterNodeGroupTable:
		parser.processClusterNodeGroupChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.ListenerTable:
		parser.processListenerChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.ClusterTable:
		parser.processClusterChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.EndpointTable:
		parser.processEndpointChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.RouteConfigurationTable:
		parser.processRouteConfigurationChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.VirtualHostTable:
		parser.processVirtualHostChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.VirtualHostDomainTable:
		parser.processVirtualHostDomainChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.RouteTable:
		parser.processRouteChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.HeaderMatcherTable:
		parser.processHeaderMatcherChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.HashPolicyTable:
		parser.processHashPolicyChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.RetryPolicyTable:
		parser.processRetryPolicyChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.HealthCheckTable:
		parser.processHealthCheckChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.TlsConfigTable:
		parser.processTlsConfigChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.TlsConfigsNodeGroupsTable:
		//nothing
	case domain.WasmFilterTable:
	case domain.ListenersWasmFilterTable:
		parser.processWasmFilterChanges(actions, entityVersions, nodeGroup)
		break
	case domain.LuaFilterTable:
		parser.processLuaFilterChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.StatefulSessionTable:
		parser.processStatefulSessionChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.RateLimitTable:
		parser.processRateLimitChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.ExtAuthzFilterTable:
		parser.processExtAuthzFilterChanges(actions, entityVersions, changes)
		break
	case domain.CircuitBreakerTable:
		parser.processCircuitBreakerChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.ThresholdTable:
		parser.processThresholdChanges(actions, entityVersions, nodeGroup, changes)
		break
	case domain.TcpKeepaliveTable:
		parser.processTcpKeepaliveChanges(actions, entityVersions, nodeGroup, changes)
		break
	default:
		logger.Warnf("Unsupported entity %v presents in change event", table)
	}
}
