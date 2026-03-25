package action

import (
	"errors"
	v3listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/cluster"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var logger logging.Logger
var oobGateways = []string{domain.PublicGateway, domain.PrivateGateway, domain.InternalGateway}
var defaultPort = "8080"

func init() {
	logger = logging.GetLogger("EnvoyUpdateManager#action")
}

//go:generate mockgen -source=action.go -destination=../../../test/mock/envoy/cache/action/stub_action.go -package=mock_action -imports cache=github.com/envoyproxy/go-control-plane/pkg/cache/v3
type SnapshotUpdateAction interface {
	Perform(original *cache.Snapshot) (*cache.Snapshot, error)
}

type UpdateActionProvider func(nodeGroup, version string, entity interface{}) SnapshotUpdateAction

type CompositeUpdateAction struct {
	actions []SnapshotUpdateAction
}

var ErrSomeActionsFailed = errors.New("cache: some snapshot updates have failed")

func (comp *CompositeUpdateAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	var resultErr error
	resultErr = nil
	snapshot := original
	for _, action := range comp.actions {
		if newSnapshot, err := action.Perform(snapshot); err == nil {
			snapshot = newSnapshot
		} else {
			logger.Errorf("Snapshot update action %v failed with error: %v", action, err)
			resultErr = ErrSomeActionsFailed
		}
	}
	return snapshot, resultErr
}

func NewCompositeUpdateAction(actions []SnapshotUpdateAction) *CompositeUpdateAction {
	return &CompositeUpdateAction{actions: actions}
}

type GenericDeleteAction struct {
	resourceName string
	envoyTypeURL resource.Type
}

func NewGenericDeleteAction(resourceName string, envoyTypeURL string) *GenericDeleteAction {
	return &GenericDeleteAction{resourceName: resourceName, envoyTypeURL: envoyTypeURL}
}

func (action *GenericDeleteAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	newResourceMap := copyResourcesMap(original.GetResourcesAndTTL(action.envoyTypeURL))
	delete(newResourceMap, action.resourceName)
	original.Resources[cache.GetResponseType(action.envoyTypeURL)] = newSnapshotResources(common.GenerateResourceVersion(), newResourceMap)
	return original, nil
}

type GenericUpdateAction struct {
	resourceName string
	envoyTypeURL resource.Type
	resource     types.ResourceWithTTL
}

func NewGenericUpdateAction(resourceName string, envoyTypeURL resource.Type, resource types.Resource) *GenericUpdateAction {
	return &GenericUpdateAction{resourceName: resourceName, envoyTypeURL: envoyTypeURL, resource: types.ResourceWithTTL{Resource: resource, TTL: nil}}
}

func (action *GenericUpdateAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	newResourceMap := copyResourcesMap(original.GetResourcesAndTTL(action.envoyTypeURL))
	newResourceMap[action.resourceName] = action.resource
	original.Resources[cache.GetResponseType(action.envoyTypeURL)] = newSnapshotResources(common.GenerateResourceVersion(), newResourceMap)
	return original, nil
}

func newSnapshotResources(version string, resourceMap map[string]types.ResourceWithTTL) cache.Resources {
	return cache.Resources{
		Version: version,
		Items:   resourceMap,
	}
}

func copyResourcesMap(items map[string]types.ResourceWithTTL) map[string]types.ResourceWithTTL {
	newResourceMap := make(map[string]types.ResourceWithTTL, len(items))
	for s, res := range items {
		newResourceMap[s] = res
	}
	return newResourceMap
}

type ListenerUpdateAction struct {
	envoyConfigBuilder builder.EnvoyConfigBuilder
	version            string
	listener           *domain.Listener
	namespaceMapping   string
}

func NewListenerUpdateAction(envoyConfigBuilder builder.EnvoyConfigBuilder, version string, listener *domain.Listener, namespaceMapping string) *ListenerUpdateAction {
	return &ListenerUpdateAction{envoyConfigBuilder: envoyConfigBuilder, version: version, listener: listener, namespaceMapping: namespaceMapping}
}

func (action *ListenerUpdateAction) Perform(snapshot *cache.Snapshot) (*cache.Snapshot, error) {
	withTls := action.withTls()
	if tlsmode.GetMode() == tlsmode.Preferred && withTls {
		envoyListener, err := action.envoyConfigBuilder.BuildListener(action.listener, action.namespaceMapping, true)
		if err != nil {
			logger.Errorf("Could not build tls version for listener %s:\n %v", action.listener.Name, err)
			return nil, err
		}
		snapshot, err = NewGenericUpdateAction(action.listener.Name+"-tls", resource.ListenerType, envoyListener).Perform(snapshot)
		if err != nil {
			logger.Errorf("Could not update tls listener %s-tls in envoy:\n %v", action.listener.Name, err)
			return nil, err
		}
	}

	var envoyListener *v3listener.Listener
	var err error
	if withTls {
		envoyListener, err = action.envoyConfigBuilder.BuildListener(action.listener, action.namespaceMapping, false)
	} else {
		envoyListener, err = action.envoyConfigBuilder.BuildListener(action.listener, action.namespaceMapping, action.listener.WithTls)
	}

	if err != nil {
		return nil, err
	}
	return NewGenericUpdateAction(action.listener.Name, resource.ListenerType, envoyListener).Perform(snapshot)
}

func (action *ListenerUpdateAction) withTls() bool {
	isOobGateway := util.SliceContains(oobGateways, action.listener.NodeGroupId)
	if isOobGateway || action.listener.BindPort == defaultPort {
		return true
	}

	return false
}

type ListenerDeleteAction struct {
	version  string
	listener *domain.Listener
}

func NewListenerDeleteAction(version string, listener *domain.Listener) *ListenerDeleteAction {
	return &ListenerDeleteAction{version: version, listener: listener}
}

func (action *ListenerDeleteAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	if tlsmode.GetMode() == tlsmode.Preferred {
		var err error
		original, err = NewGenericDeleteAction(action.listener.Name+"-tls", resource.ListenerType).Perform(original)
		if err != nil {
			logger.Errorf("Could not delete tls version of listener %s from envoy:\n %v", action.listener.Name, err)
			return nil, err
		}
	}
	return NewGenericDeleteAction(action.listener.Name, resource.ListenerType).Perform(original)
}

type ClusterUpdateAction struct {
	envoyConfigBuilder builder.EnvoyConfigBuilder
	version            string
	nodeGroup          string
	cluster            *domain.Cluster
}

func NewClusterUpdateAction(envoyConfigBuilder builder.EnvoyConfigBuilder, nodeGroup, version string, newCluster *domain.Cluster) *ClusterUpdateAction {
	return &ClusterUpdateAction{envoyConfigBuilder: envoyConfigBuilder, nodeGroup: nodeGroup, version: version, cluster: newCluster}
}

func (action *ClusterUpdateAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	envoyCluster, err := action.envoyConfigBuilder.BuildCluster(action.nodeGroup, action.cluster)
	if err != nil {
		return nil, err
	}
	return NewGenericUpdateAction(cluster.ReplaceDotsByUnderscore(action.cluster.Name), resource.ClusterType, envoyCluster).Perform(original)
}

type ClusterDeleteAction struct {
	version string
	cluster *domain.Cluster
}

func NewClusterDeleteAction(version string, cluster *domain.Cluster) *ClusterDeleteAction {
	return &ClusterDeleteAction{version: version, cluster: cluster}
}

func (action *ClusterDeleteAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	return NewGenericDeleteAction(action.cluster.Name, resource.ClusterType).Perform(original)
}

type RouteConfigUpdateAction struct {
	envoyConfigBuilder builder.EnvoyConfigBuilder
	version            string
	routeConfig        *domain.RouteConfiguration
}

func NewRouteConfigUpdateAction(envoyConfigBuilder builder.EnvoyConfigBuilder, version string, routeConfig *domain.RouteConfiguration) *RouteConfigUpdateAction {
	return &RouteConfigUpdateAction{envoyConfigBuilder: envoyConfigBuilder, version: version, routeConfig: routeConfig}
}

func (action *RouteConfigUpdateAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	envoyRouteConfig, err := action.envoyConfigBuilder.BuildRouteConfig(action.routeConfig)
	if err != nil {
		return nil, err
	}
	return NewGenericUpdateAction(action.routeConfig.Name, resource.RouteType, envoyRouteConfig).Perform(original)
}

type RouteConfigDeleteAction struct {
	version     string
	routeConfig *domain.RouteConfiguration
}

func NewRouteConfigDeleteAction(version string, routeConfig *domain.RouteConfiguration) *RouteConfigDeleteAction {
	return &RouteConfigDeleteAction{version: version, routeConfig: routeConfig}
}

func (action *RouteConfigDeleteAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	return NewGenericDeleteAction(action.routeConfig.Name, resource.RouteType).Perform(original)
}

type DeleteAllByTypeAction struct {
	nodeGroup    string
	version      string
	envoyTypeURL resource.Type
}

func NewDeleteAllByTypeAction(version string, envoyTypeURL resource.Type) *DeleteAllByTypeAction {
	return &DeleteAllByTypeAction{version: version, envoyTypeURL: envoyTypeURL}
}

func (action *DeleteAllByTypeAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	resourceType := cache.GetResponseType(action.envoyTypeURL)
	original.Resources[resourceType].Version = action.version
	original.Resources[resourceType].Items = make(map[string]types.ResourceWithTTL)
	return original, nil
}

type RuntimeUpdateAction struct {
	envoyConfigBuilder builder.EnvoyConfigBuilder
	nodeGroup          string
}

func NewRuntimeUpdateAction(envoyConfigBuilder builder.EnvoyConfigBuilder, nodeGroup string) *RuntimeUpdateAction {
	return &RuntimeUpdateAction{envoyConfigBuilder: envoyConfigBuilder, nodeGroup: nodeGroup}
}

func (a *RuntimeUpdateAction) Perform(original *cache.Snapshot) (*cache.Snapshot, error) {
	runtimeName := "rtds_layer0"
	envoyRuntime, err := a.envoyConfigBuilder.BuildRuntime(a.nodeGroup, runtimeName)
	if err != nil {
		return nil, err
	}
	return NewGenericUpdateAction(runtimeName, resource.RuntimeType, envoyRuntime).Perform(original)
}
