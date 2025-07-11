package dao

import (
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
)

//go:generate mockgen -source=api.go -destination=../test/mock/dao/stub_api.go -package=mock_dao -imports memdb=github.com/hashicorp/go-memdb
type Dao interface {
	Repository
	WithWTx(func(dao Repository) error) ([]memdb.Change, error)
	WithWTxVal(func(dao Repository) (interface{}, error)) (interface{}, []memdb.Change, error)
	WithRTx(func(dao Repository) error) error
	WithRTxVal(payload func(dao Repository) (interface{}, error)) (interface{}, error)
}

type Repository interface {
	ClusterRepository
	RouteConfigurationRepository
	NodeGroupRepository
	ListenerRepository
	VirtualHostRepository
	RouteRepository
	EnvoyConfigVersionRepository
	EndpointRepository
	DeploymentVersionRepository
	HashPolicyRepository
	HealthCheckRepository
	RetryPolicyRepository
	TlsConfigRepository
	WasmFilterRepository
	LuaFilterRepository
	CompositeSatelliteRepository
	StatefulSessionRepository
	RateLimitRepository
	MicroserviceVersionRepository
	ExtAuthzFilterRepository
	CircuitBreakersRepository
	ThresholdsRepository
	TcpKeepaliveRepository
	SaveEntity(table string, entity interface{}) error
}

type ClusterRepository interface {
	FindClusterById(id int32) (*domain.Cluster, error)
	FindClusterByName(key string) (*domain.Cluster, error)
	FindClustersByFamilyNameAndNamespace(familyName string, namespace msaddr.Namespace) ([]*domain.Cluster, error)
	FindClusterByNodeGroup(group *domain.NodeGroup) ([]*domain.Cluster, error)
	FindClusterByEndpointIn(endpoints []*domain.Endpoint) ([]*domain.Cluster, error)
	SaveCluster(cluster *domain.Cluster) error
	FindAllClusters() ([]*domain.Cluster, error)
	DeleteCluster(cluster *domain.Cluster) error
	DeleteClusterByName(key string) error
}

type EnvoyConfigVersionRepository interface {
	SaveEnvoyConfigVersion(version *domain.EnvoyConfigVersion) error
	FindEnvoyConfigVersion(nodeGroup, entityType string) (*domain.EnvoyConfigVersion, error)
}

type ListenerRepository interface {
	FindAllListeners() ([]*domain.Listener, error)
	SaveListener(listener *domain.Listener) error
	FindListenerById(id int32) (*domain.Listener, error)
	FindListenerByNodeGroupIdAndName(nodeGroupId, name string) (*domain.Listener, error)
	FindListenersByNodeGroupId(nodeGroupId string) ([]*domain.Listener, error)
	HasWasmFilterWithId(listenerId, wasmFilterId int32) (bool, error)
	DeleteListenerByNodeGroupName(nodeGroupId string) error
	DeleteListenerById(id int32) error
	SaveListenerWasmFilter(relation *domain.ListenersWasmFilter) error
	DeleteListenerWasmFilter(relation *domain.ListenersWasmFilter) error
	FindAllListenerWasmFilter() ([]*domain.ListenersWasmFilter, error)
	FindListenerIdsByWasmFilterId(wasmFilterId int32) ([]int32, error)
	HasLuaFilterWithId(listenerId, luaFilterId int32) (bool, error)
	SaveListenerLuaFilter(relation *domain.ListenersLuaFilter) error
	DeleteListenerLuaFilter(relation *domain.ListenersLuaFilter) error
	FindAllListenerLuaFilter() ([]*domain.ListenersLuaFilter, error)
	FindListenerIdsByLuaFilterId(luaFilterId int32) ([]int32, error)
}

type TlsConfigRepository interface {
	SaveTlsConfig(tls *domain.TlsConfig) error
	FindTlsConfigById(id int32) (*domain.TlsConfig, error)
	FindTlsConfigByName(name string) (*domain.TlsConfig, error)
	FindAllTlsConfigsByNodeGroup(nodeGroup string) ([]*domain.TlsConfig, error)
	FindAllTlsConfigs() ([]*domain.TlsConfig, error)
	DeleteTlsConfigById(id int32) error
	DeleteTlsConfigByIdAndNodeGroupName(relation *domain.TlsConfigsNodeGroups) error
}

type WasmFilterRepository interface {
	FindAllWasmFilters() ([]*domain.WasmFilter, error)
	SaveWasmFilter(filter *domain.WasmFilter) error
	FindWasmFilterByName(filterName string) (*domain.WasmFilter, error)
	FindWasmFilterByListenerId(listenerId int32) ([]*domain.WasmFilter, error)
	DeleteWasmFilterByName(filterName string) (int32, error)
	DeleteWasmFilterById(id int32) error
}

type LuaFilterRepository interface {
	FindAllLuaFilters() ([]*domain.LuaFilter, error)
	SaveLuaFilter(filter *domain.LuaFilter) error
	FindLuaFilterByName(filterName string) (*domain.LuaFilter, error)
	FindLuaFilterByListenerId(listenerId int32) ([]*domain.LuaFilter, error)
	DeleteLuaFilterByName(filterName string) (int32, error)
	DeleteLuaFilterById(id int32) error
}

type StatefulSessionRepository interface {
	FindAllStatefulSessionConfigs() ([]*domain.StatefulSession, error)
	SaveStatefulSessionConfig(statefulSession *domain.StatefulSession) error
	FindStatefulSessionConfigById(id int32) (*domain.StatefulSession, error)
	FindStatefulSessionConfigsByCluster(cluster *domain.Cluster) ([]*domain.StatefulSession, error)
	FindStatefulSessionConfigsByClusterName(clusterName string, namespace msaddr.Namespace) ([]*domain.StatefulSession, error)
	FindStatefulSessionConfigsByCookieName(cookieName string) ([]*domain.StatefulSession, error)
	FindStatefulSessionConfigsByClusterAndVersion(clusterName string, namespace msaddr.Namespace, version *domain.DeploymentVersion) ([]*domain.StatefulSession, error)
	DeleteStatefulSessionConfig(id int32) error
}

type RouteConfigurationRepository interface {
	FindRouteConfigById(routeConfigurationId int32) (*domain.RouteConfiguration, error)
	FindAllRouteConfigs() ([]*domain.RouteConfiguration, error)
	FindRouteConfigByNodeGroupIdAndName(nodeGroupId, name string) (*domain.RouteConfiguration, error)
	FindRouteConfigsByNodeGroupId(nodeGroupId string) ([]*domain.RouteConfiguration, error)
	FindRouteConfigsByEndpoint(endpoint *domain.Endpoint) ([]*domain.RouteConfiguration, error)
	FindRouteConfigsByRouteDeploymentVersion(deploymentVersion string) ([]*domain.RouteConfiguration, error)
	SaveRouteConfig(routeConfig *domain.RouteConfiguration) error
	DeleteRouteConfigById(id int32) error
}

type NodeGroupRepository interface {
	FindAllClusterWithNodeGroup() ([]*domain.ClustersNodeGroup, error)
	FindAllNodeGroups() ([]*domain.NodeGroup, error)
	FindNodeGroupByName(name string) (*domain.NodeGroup, error)
	FindNodeGroupsByCluster(cluster *domain.Cluster) ([]*domain.NodeGroup, error)
	SaveNodeGroup(nodeGroup *domain.NodeGroup) error
	SaveClustersNodeGroup(relation *domain.ClustersNodeGroup) error
	FindClustersNodeGroup(relation *domain.ClustersNodeGroup) (*domain.ClustersNodeGroup, error)
	DeleteClustersNodeGroupByClusterId(clusterId int32) (int, error)
	DeleteClustersNodeGroup(relation *domain.ClustersNodeGroup) error
	DeleteNodeGroupByName(nodeGroupName string) error
}

type VirtualHostRepository interface {
	FindFirstVirtualHostByNameAndRouteConfigurationId(name string, id int32) (*domain.VirtualHost, error)
	FindFirstVirtualHostByRouteConfigurationId(routeConfigId int32) (*domain.VirtualHost, error)
	FindVirtualHostDomainByVirtualHostId(virtualHostId int32) ([]*domain.VirtualHostDomain, error)
	FindVirtualHostDomainsByHost(virtualHostDomain string) ([]*domain.VirtualHostDomain, error)
	FindVirtualHostById(virtualHostId int32) (*domain.VirtualHost, error)
	SaveVirtualHost(virtualHost *domain.VirtualHost) error
	SaveVirtualHostDomain(virtualHostDomain *domain.VirtualHostDomain) error
	FindVirtualHostsByRouteConfigurationId(routeConfigId int32) ([]*domain.VirtualHost, error)
	FindAllVirtualHosts() ([]*domain.VirtualHost, error)
	FindAllVirtualHostsDomain() ([]*domain.VirtualHostDomain, error)
	DeleteVirtualHostsDomain(virtualHostDomain *domain.VirtualHostDomain) error
	DeleteVirtualHost(host *domain.VirtualHost) error
}

type RouteRepository interface {
	FindRoutesByDeploymentVersionAndRouteKey(deploymentVersion, routeKey string) ([]*domain.Route, error)
	FindRoutesByVirtualHostId(virtualHostId int32) ([]*domain.Route, error)
	FindHeaderMatcherByRouteId(routeId int32) ([]*domain.HeaderMatcher, error)
	DeleteHeaderMatcherById(Id int32) error
	FindRoutesByAutoGeneratedAndDeploymentVersion(autoGenerated bool, dVersion string) ([]*domain.Route, error)
	FindRoutesByVirtualHostIdAndRouteKey(vHostId int32, routeKey string) ([]*domain.Route, error)
	FindRoutesByVirtualHostIdAndDeploymentVersion(virtualHostId int32, version string) ([]*domain.Route, error)
	FindRoutesByDeploymentVersion(dVersion string) ([]*domain.Route, error)
	FindRoutesByDeploymentVersions(dVersions ...*domain.DeploymentVersion) ([]*domain.Route, error)
	FindRoutesByDeploymentVersionIn(dVersions ...string) ([]*domain.Route, error)
	FindRoutesByDeploymentVersionStageIn(dVersionStage ...string) ([]*domain.Route, error)
	FindRoutesByClusterName(clusterName string) ([]*domain.Route, error)
	FindRoutesByClusterNamePrefix(clusterNamePrefix string) ([]*domain.Route, error)
	FindRoutesByClusterNameAndDeploymentVersion(clusterName, dVersion string) ([]*domain.Route, error)
	FindRoutesByUUIDPrefix(prefixUuid string) ([]*domain.Route, error)
	FindRoutesByNamespaceHeaderIsNot(headerName string) ([]*domain.Route, error)
	FindRouteById(routeId int32) (*domain.Route, error)
	FindRouteByUuid(uuid string) (*domain.Route, error)
	FindRouteByStatefulSession(statefulSessionId int32) (*domain.Route, error)
	SaveRoute(route *domain.Route) error
	SaveHeaderMatcher(headerMatcher *domain.HeaderMatcher) error
	DeleteHeaderMatchersByRouteId(routeId int32) (int, error)
	DeleteRoutesByAutoGeneratedAndDeploymentVersion(autoGenerated bool, dVersion string) (int, error)
	DeleteRouteById(routeId int32) error
	DeleteRouteByUUID(uuid string) error
	DeleteHeaderMatcher(headerMatcher *domain.HeaderMatcher) error
	FindAllRoutes() ([]*domain.Route, error)
	FindRoutesByRateLimit(rateLimitId string) ([]*domain.Route, error)
}

type DeploymentVersionRepository interface {
	DeleteDeploymentVersion(dVersion *domain.DeploymentVersion) error
	DeleteDeploymentVersions(dVersion []*domain.DeploymentVersion) error
	FindAllDeploymentVersions() ([]*domain.DeploymentVersion, error)
	FindDeploymentVersionsByStage(stage string) ([]*domain.DeploymentVersion, error)
	FindDeploymentVersion(version string) (*domain.DeploymentVersion, error)
	SaveDeploymentVersion(dVersion *domain.DeploymentVersion) error
}

type EndpointRepository interface {
	DeleteEndpoint(endpoint *domain.Endpoint) error
	FindAllEndpoints() ([]*domain.Endpoint, error)
	FindEndpointsByClusterId(clusterId int32) ([]*domain.Endpoint, error)
	FindEndpointsByClusterIdAndDeploymentVersion(clusterId int32, dVersions *domain.DeploymentVersion) ([]*domain.Endpoint, error)
	FindEndpointsByDeploymentVersionsIn(dVersions []*domain.DeploymentVersion) ([]*domain.Endpoint, error)
	FindEndpointById(endpointId int32) (*domain.Endpoint, error)
	FindEndpointsByClusterName(clusterName string) ([]*domain.Endpoint, error)
	FindEndpointsByDeploymentVersion(version string) ([]*domain.Endpoint, error)
	FindEndpointsByAddressAndPortAndDeploymentVersion(address string, port int32, dVersion string) ([]*domain.Endpoint, error)
	FindEndpointByStatefulSession(statefulSessionId int32) (*domain.Endpoint, error)
	SaveEndpoint(endpoint *domain.Endpoint) error
}

type HashPolicyRepository interface {
	DeleteHashPolicyById(Id int32) error
	DeleteHashPolicyByRouteId(routeId int32) (int, error)
	DeleteHashPolicyByEndpointId(endpointId int32) (int, error)
	FindHashPolicyByEndpointId(endpointId int32) ([]*domain.HashPolicy, error)
	FindHashPolicyByRouteId(routeId int32) ([]*domain.HashPolicy, error)
	FindHashPolicyByClusterAndVersions(clusterName string, versions ...string) ([]*domain.HashPolicy, error)
	SaveHashPolicy(hashPolicy *domain.HashPolicy) error
}

type HealthCheckRepository interface {
	DeleteHealthCheckById(Id int32) error
	DeleteHealthChecksByClusterId(clusterId int32) (int, error)
	FindHealthChecksByClusterId(clusterId int32) ([]*domain.HealthCheck, error)
	SaveHealthCheck(healthCheck *domain.HealthCheck) error
}

type RetryPolicyRepository interface {
	DeleteRetryPolicyById(Id int32) error
	DeleteRetryPolicyByRouteId(routeId int32) error
	FindRetryPolicyByRouteId(routeId int32) (*domain.RetryPolicy, error)
	SaveRetryPolicy(retryPolicy *domain.RetryPolicy) error
}

type CompositeSatelliteRepository interface {
	SaveCompositeSatellite(satellite *domain.CompositeSatellite) error
	DeleteCompositeSatellite(namespace string) error
	FindAllCompositeSatellites() ([]*domain.CompositeSatellite, error)
	FindCompositeSatelliteByNamespace(namespace string) (*domain.CompositeSatellite, error)
}

type RateLimitRepository interface {
	FindRateLimitByNameWithHighestPriority(name string) (*domain.RateLimit, error)
	SaveRateLimit(rateLimit *domain.RateLimit) error
	DeleteRateLimitByNameAndPriority(name string, priority domain.ConfigPriority) error
	FindAllRateLimits() ([]*domain.RateLimit, error)
}

type MicroserviceVersionRepository interface {
	SaveMicroserviceVersion(msVersion *domain.MicroserviceVersion) error
	DeleteMicroserviceVersion(name string, namespace msaddr.Namespace, initialVersion string) error
	FindAllMicroserviceVersions() ([]*domain.MicroserviceVersion, error)
	FindMicroserviceVersionByNameAndInitialVersion(name string, namespace msaddr.Namespace, initialVersion string) (*domain.MicroserviceVersion, error)
	FindMicroserviceVersionsByVersion(version *domain.DeploymentVersion) ([]*domain.MicroserviceVersion, error)
	FindMicroserviceVersionsByNameAndNamespace(name string, namespace msaddr.Namespace) ([]*domain.MicroserviceVersion, error)
}

type ExtAuthzFilterRepository interface {
	SaveExtAuthzFilter(filter *domain.ExtAuthzFilter) error
	DeleteExtAuthzFilter(extAuthzFilterName string) error
	FindExtAuthzFilterByName(name string) (*domain.ExtAuthzFilter, error)
	FindExtAuthzFilterByNodeGroup(nodeGroup string) (*domain.ExtAuthzFilter, error)
	FindAllExtAuthzFilters() ([]*domain.ExtAuthzFilter, error)
}

type CircuitBreakersRepository interface {
	SaveCircuitBreaker(circuitBreaker *domain.CircuitBreaker) error
	FindCircuitBreakerById(Id int32) (*domain.CircuitBreaker, error)
	FindAllCircuitBreakers() ([]*domain.CircuitBreaker, error)
	DeleteCircuitBreakerById(Id int32) error
}

type ThresholdsRepository interface {
	SaveThreshold(threshold *domain.Threshold) error
	FindThresholdById(Id int32) (*domain.Threshold, error)
	DeleteThresholdById(Id int32) error
	FindAllThresholds() ([]*domain.Threshold, error)
}

type TcpKeepaliveRepository interface {
	SaveTcpKeepalive(tcpKeepalive *domain.TcpKeepalive) error
	FindTcpKeepaliveById(Id int32) (*domain.TcpKeepalive, error)
	DeleteTcpKeepaliveById(Id int32) error
	FindAllTcpKeepalives() ([]*domain.TcpKeepalive, error)
}
