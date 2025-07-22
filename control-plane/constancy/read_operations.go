package constancy

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/uptrace/bun"
)

func (s *StorageImpl) FindClusterByName(key string) (*domain.Cluster, error) {
	return find[domain.Cluster](s, "name", key)
}

func (s *StorageImpl) FindClustersNodeGroupByIdAndNodeGroup(clusterId int32, nodeGroup string) (*domain.ClustersNodeGroup, error) {
	result := &domain.ClustersNodeGroup{}
	err := s.findByCondition(result, func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("nodegroups_name = ?", nodeGroup).Where("clusters_id = ?", clusterId)
	})
	return result, err
}

func (s *StorageImpl) FindTlsConfigByIdAndNodeGroupName(tlsConfigId int32, nodeGroupName string) (*domain.TlsConfigsNodeGroups, error) {
	result := &domain.TlsConfigsNodeGroups{}
	err := s.findByCondition(result, func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("tls_config_id = ?", tlsConfigId).Where("node_group_name = ?", nodeGroupName)
	})
	return result, err
}

func (s *StorageImpl) FindCompositeSatelliteByNamespace(namespace string) (*domain.CompositeSatellite, error) {
	return find[domain.CompositeSatellite](s, "namespace", namespace)
}

func (s *StorageImpl) FindDeploymentVersionByName(version string) (*domain.DeploymentVersion, error) {
	return find[domain.DeploymentVersion](s, "version", version)
}

func (s *StorageImpl) FindEndpointById(id int32) (*domain.Endpoint, error) {
	return find[domain.Endpoint](s, "id", id)
}

func (s *StorageImpl) FindHashPolicyById(id int32) (*domain.HashPolicy, error) {
	return find[domain.HashPolicy](s, "id", id)
}

func (s *StorageImpl) FindHeaderMatcherById(id int32) (*domain.HeaderMatcher, error) {
	return find[domain.HeaderMatcher](s, "id", id)
}

func (s *StorageImpl) FindHealthCheckById(id int32) (*domain.HealthCheck, error) {
	return find[domain.HealthCheck](s, "id", id)
}

func (s *StorageImpl) FindListenerById(id int32) (*domain.Listener, error) {
	return find[domain.Listener](s, "id", id)
}

func (s *StorageImpl) FindRetryPolicyById(id int32) (*domain.RetryPolicy, error) {
	return find[domain.RetryPolicy](s, "id", id)
}

func (s *StorageImpl) FindRouteConfigById(id int32) (*domain.RouteConfiguration, error) {
	return find[domain.RouteConfiguration](s, "id", id)
}

func (s *StorageImpl) FindRouteById(id int32) (*domain.Route, error) {
	return find[domain.Route](s, "id", id)
}

func (s *StorageImpl) FindTlsConfigById(id int32) (*domain.TlsConfig, error) {
	return find[domain.TlsConfig](s, "id", id)
}

func (s *StorageImpl) FindVirtualHostById(id int32) (*domain.VirtualHost, error) {
	return find[domain.VirtualHost](s, "id", id)
}

func (s *StorageImpl) FindWasmFilterById(id int32) (*domain.WasmFilter, error) {
	return find[domain.WasmFilter](s, "id", id)
}

func (s *StorageImpl) FindLuaFilterById(id int32) (*domain.LuaFilter, error) {
	return find[domain.LuaFilter](s, "id", id)
}

func (s *StorageImpl) FindStatefulSessionById(id int32) (*domain.StatefulSession, error) {
	return find[domain.StatefulSession](s, "id", id)
}

func (s *StorageImpl) FindRateLimitByNameAndPriority(name string, priority domain.ConfigPriority) (*domain.RateLimit, error) {
	result := &domain.RateLimit{}
	err := s.findByCondition(result, func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("name = ?", name).Where("priority = ?", priority)
	})
	return result, err
}

func (s *StorageImpl) FindNodeGroupByName(name string) (*domain.NodeGroup, error) {
	return find[domain.NodeGroup](s, "name", name)
}

func (s *StorageImpl) FindExtAuthzFilterByName(name string) (*domain.ExtAuthzFilter, error) {
	return find[domain.ExtAuthzFilter](s, "name", name)
}

func (s *StorageImpl) FindCircuitBreakerById(id int32) (*domain.CircuitBreaker, error) {
	return find[domain.CircuitBreaker](s, "id", id)
}

func (s *StorageImpl) FindThresholdById(id int32) (*domain.Threshold, error) {
	return find[domain.Threshold](s, "id", id)
}

func (s *StorageImpl) FindTcpKeepaliveById(id int32) (*domain.TcpKeepalive, error) {
	return find[domain.TcpKeepalive](s, "id", id)
}

func find[T any](s *StorageImpl, fieldName string, fieldValue any) (*T, error) {
	var entity T
	result := &entity
	err := s.findByCondition(result, func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where(fieldName+" = ?", fieldValue)
	})
	return result, err
}

func (s *StorageImpl) findByCondition(receiver interface{}, provideConditions func(query *bun.SelectQuery) *bun.SelectQuery) error {
	err := s.WithTx(func(conn *bun.Conn) error {
		query := s.PGQuery.Select(conn)
		query = s.PGConn.Model(query, receiver)
		query = provideConditions(query)
		return s.PGQuery.Scan(query)
	})
	if err != nil {
		log.ErrorC(ctx, "Error selecting entity from database: %v", err.Error())
	}
	return err
}
