package ram

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/data"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	error_util "github.com/pkg/errors"
	"reflect"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("ram")
}

//go:generate mockgen -source=storage.go -destination=./storage_mock.go -package=ram -imports memdb=github.com/hashicorp/go-memdb
type RamStorage interface {
	data.RestorableStorage
	WriteTx() Txn
	ReadTx() Txn
	Tx(write bool) Txn
	FindAll(txn Txn, table string) (interface{}, error)
	FindById(tx Txn, table string, ids ...interface{}) (interface{}, error)
	FindFirstByIndex(tx Txn, table string, index string, args ...interface{}) (interface{}, error)
	FindByIndex(tx Txn, table string, index string, args ...interface{}) (interface{}, error)
	Save(txn Txn, table string, entity interface{}) error
	Clear() error
}

type Txn interface {
	TrackChanges()
	Abort()
	Commit()
	Insert(table string, obj interface{}) error
	Delete(table string, obj interface{}) error
	DeletePrefix(table string, prefix_index string, prefix string) (bool, error)
	DeleteAll(table string, index string, args ...interface{}) (int, error)
	FirstWatch(table string, index string, args ...interface{}) (<-chan struct{}, interface{}, error)
	LastWatch(table string, index string, args ...interface{}) (<-chan struct{}, interface{}, error)
	First(table string, index string, args ...interface{}) (interface{}, error)
	Last(table string, index string, args ...interface{}) (interface{}, error)
	LongestPrefix(table string, index string, args ...interface{}) (interface{}, error)
	Get(table string, index string, args ...interface{}) (memdb.ResultIterator, error)
	GetReverse(table string, index string, args ...interface{}) (memdb.ResultIterator, error)
	LowerBound(table string, index string, args ...interface{}) (memdb.ResultIterator, error)
	ReverseLowerBound(table string, index string, args ...interface{}) (memdb.ResultIterator, error)
	Changes() memdb.Changes
	Defer(fn func())
	Snapshot() Txn
}

type TxnImpl struct {
	*memdb.Txn
}

func (txn TxnImpl) Snapshot() Txn {
	r := TxnImpl{txn.Txn.Snapshot()}
	return &r
}

type Storage struct {
	db *memdb.MemDB
}

func NewStorage() RamStorage {
	ms := &Storage{}
	ms.initStorage()
	return ms
}

func (s *Storage) initStorage() {
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		log.Panicf("Failed to create in-memory DB:\n %v", err)
	}
	s.db = db
}

func (s *Storage) WriteTx() Txn {
	return &TxnImpl{s.db.Txn(true)}
}

func (s *Storage) ReadTx() Txn {
	return &TxnImpl{s.db.Txn(false)}
}

func (s *Storage) Tx(write bool) Txn {
	return &TxnImpl{s.db.Txn(write)}
}

func (s *Storage) FindAll(txn Txn, table string) (interface{}, error) {
	if itr, err := txn.Get(table, "id"); err == nil {
		entitySliceType := reflect.SliceOf(domain.TableType[table])
		entitySlice := reflect.MakeSlice(entitySliceType, 0, 0)
		for elem := itr.Next(); elem != nil; elem = itr.Next() {
			entitySlice = reflect.Append(entitySlice, reflect.ValueOf(elem))
		}
		return entitySlice.Interface(), nil
	} else {
		return nil, errors.WrapPrefix(err, fmt.Sprintf("get entities by index 'id' for table %s caused error", table), 1)
	}
}

func (s *Storage) FindById(tx Txn, table string, ids ...interface{}) (interface{}, error) {
	return s.FindFirstByIndex(tx, table, "id", ids...)
}

func (s *Storage) FindFirstByIndex(tx Txn, table string, index string, args ...interface{}) (interface{}, error) {
	if raw, err := tx.First(table, index, args...); err == nil {
		return raw, nil
	} else {
		return nil, error_util.Wrapf(err, "Error getting entity from table: '%v', from index: '%v', with args: '%v'", table, index, args)
	}
}

func (s *Storage) FindByIndex(tx Txn, table string, index string, args ...interface{}) (interface{}, error) {
	if itr, err := tx.Get(table, index, args...); err == nil {
		entitySliceType := reflect.SliceOf(domain.TableType[table])
		entitySlice := reflect.MakeSlice(entitySliceType, 0, 0)
		for elem := itr.Next(); elem != nil; elem = itr.Next() {
			entitySlice = reflect.Append(entitySlice, reflect.ValueOf(elem))
		}
		return entitySlice.Interface(), nil
	} else {
		return nil, error_util.Wrapf(err, "Find By Index for table %v caused error", table)
	}
}

func (s *Storage) Save(txn Txn, table string, entity interface{}) error {
	v := reflect.ValueOf(entity)
	switch v.Kind() {
	case reflect.Slice:
		return s.saveAll(txn, table, util.ToSlice(entity))
	case reflect.Struct:
		return s.saveAll(txn, table, []interface{}{entity})
	case reflect.Ptr:
		return s.saveAll(txn, table, []interface{}{entity})
	default:
		return fmt.Errorf("can't handle entity of type: %v", v.Kind())
	}
}

func (s *Storage) saveAll(txn Txn, table string, entities []interface{}) error {
	for _, entity := range entities {
		if err := txn.Insert(table, entity); err != nil {
			return error_util.Wrapf(err, "Error insert entity: '%v' to table: %v, ", entity, table)
		}
	}
	return nil
}

func (s *Storage) Clear() error {
	tx := s.WriteTx()
	defer tx.Abort()
	for _, tableName := range domain.TableRelationOrder {
		_, err := tx.DeleteAll(tableName, "id")
		if err != nil {
			return err
		}
	}
	tx.Commit()
	return nil
}

func (s *Storage) Backup() (*data.Snapshot, error) {
	//dataMap := make(map[string]interface{})
	tx := s.ReadTx()
	defer tx.Abort()
	snapshot := data.NewSnapshot()
	for _, tableName := range domain.TableRelationOrder {
		entities, err := s.FindAll(tx, tableName)
		if err != nil {
			return nil, err
		}
		switch tableName {
		case domain.NodeGroupTable:
			nodeGroupPtrs := entities.([]*domain.NodeGroup)
			nodeGroups := make([]domain.NodeGroup, len(nodeGroupPtrs))
			for i, nodeGroup := range nodeGroupPtrs {
				nodeGroups[i] = *nodeGroup
				_ = nodeGroups[i].MarshalPrepare()
			}
			snapshot.NodeGroups = nodeGroups
		case domain.DeploymentVersionTable:
			deploymentVersionPtrs := entities.([]*domain.DeploymentVersion)
			deploymentVersions := make([]domain.DeploymentVersion, len(deploymentVersionPtrs))
			for i, deploymentVersion := range deploymentVersionPtrs {
				deploymentVersions[i] = *deploymentVersion
				_ = deploymentVersions[i].MarshalPrepare()
			}
			snapshot.DeploymentVersions = deploymentVersions
		case domain.ClusterNodeGroupTable:
			clusterNodeGroupPtrs := entities.([]*domain.ClustersNodeGroup)
			clusterNodeGroups := make([]domain.ClustersNodeGroup, len(clusterNodeGroupPtrs))
			for i, clusterNodeGroup := range clusterNodeGroupPtrs {
				clusterNodeGroups[i] = *clusterNodeGroup
				_ = clusterNodeGroups[i].MarshalPrepare()
			}
			snapshot.ClusterNodeGroups = clusterNodeGroups
		case domain.ListenerTable:
			listenerPtrs := entities.([]*domain.Listener)
			listeners := make([]domain.Listener, len(listenerPtrs))
			for i, listener := range listenerPtrs {
				listeners[i] = *listener
				_ = listeners[i].MarshalPrepare()
			}
			snapshot.Listeners = listeners
		case domain.MicroserviceVersionTable:
			ptrs := entities.([]*domain.MicroserviceVersion)
			entitiesSlice := make([]domain.MicroserviceVersion, len(ptrs))
			for i, ptr := range ptrs {
				entitiesSlice[i] = *ptr
				_ = entitiesSlice[i].MarshalPrepare()
			}
			snapshot.MicroserviceVersions = entitiesSlice
		case domain.ClusterTable:
			clusterPtrs := entities.([]*domain.Cluster)
			clusters := make([]domain.Cluster, len(clusterPtrs))
			for i, cluster := range clusterPtrs {
				clusters[i] = *cluster
				_ = clusters[i].MarshalPrepare()
			}
			snapshot.Clusters = clusters
		case domain.EndpointTable:
			endpointPtrs := entities.([]*domain.Endpoint)
			endpoints := make([]domain.Endpoint, len(endpointPtrs))
			for i, endpoint := range endpointPtrs {
				endpoints[i] = *endpoint
				_ = endpoints[i].MarshalPrepare()
			}
			snapshot.Endpoints = endpoints
		case domain.RouteConfigurationTable:
			routeConfigurationPtrs := entities.([]*domain.RouteConfiguration)
			routeConfigurations := make([]domain.RouteConfiguration, len(routeConfigurationPtrs))
			for i, routeConfiguration := range routeConfigurationPtrs {
				routeConfigurations[i] = *routeConfiguration
				_ = routeConfigurations[i].MarshalPrepare()
			}
			snapshot.RouteConfigurations = routeConfigurations
		case domain.VirtualHostTable:
			virtualHostPtrs := entities.([]*domain.VirtualHost)
			virtualHosts := make([]domain.VirtualHost, len(virtualHostPtrs))
			for i, virtualHost := range virtualHostPtrs {
				virtualHosts[i] = *virtualHost
				_ = virtualHosts[i].MarshalPrepare()
			}
			snapshot.VirtualHosts = virtualHosts
		case domain.VirtualHostDomainTable:
			virtualHostDomainPtrs := entities.([]*domain.VirtualHostDomain)
			virtualHostDomains := make([]domain.VirtualHostDomain, len(virtualHostDomainPtrs))
			for i, virtualHostDomain := range virtualHostDomainPtrs {
				virtualHostDomains[i] = *virtualHostDomain
				_ = virtualHostDomains[i].MarshalPrepare()
			}
			snapshot.VirtualHostDomains = virtualHostDomains
		case domain.RouteTable:
			routePtrs := entities.([]*domain.Route)
			routes := make([]domain.Route, len(routePtrs))
			for i, route := range routePtrs {
				routes[i] = *route
				_ = routes[i].MarshalPrepare()
			}
			snapshot.Routes = routes
		case domain.HeaderMatcherTable:
			headerMatcherPtrs := entities.([]*domain.HeaderMatcher)
			headerMatchers := make([]domain.HeaderMatcher, len(headerMatcherPtrs))
			for i, headerMatcher := range headerMatcherPtrs {
				headerMatchers[i] = *headerMatcher
				_ = headerMatchers[i].MarshalPrepare()
			}
			snapshot.HeaderMatchers = headerMatchers
		case domain.HashPolicyTable:
			hashPolicyPtrs := entities.([]*domain.HashPolicy)
			hashPolicies := make([]domain.HashPolicy, len(hashPolicyPtrs))
			for i, hashPolicy := range hashPolicyPtrs {
				hashPolicies[i] = *hashPolicy
				_ = hashPolicies[i].MarshalPrepare()
			}
			snapshot.HashPolicies = hashPolicies
		case domain.RetryPolicyTable:
			retryPolicyPtrs := entities.([]*domain.RetryPolicy)
			retryPolicies := make([]domain.RetryPolicy, len(retryPolicyPtrs))
			for i, retryPolicy := range retryPolicyPtrs {
				retryPolicies[i] = *retryPolicy
				_ = retryPolicies[i].MarshalPrepare()
			}
			snapshot.RetryPolicies = retryPolicies
		case domain.HealthCheckTable:
			healthCheckPtrs := entities.([]*domain.HealthCheck)
			healthChecks := make([]domain.HealthCheck, len(healthCheckPtrs))
			for i, healthCheck := range healthCheckPtrs {
				healthChecks[i] = *healthCheck
				_ = healthChecks[i].MarshalPrepare()
			}
			snapshot.HealthChecks = healthChecks
		case domain.EnvoyConfigVersionTable:
			envoyConfigVersionPtrs := entities.([]*domain.EnvoyConfigVersion)
			envoyConfigVersions := make([]domain.EnvoyConfigVersion, len(envoyConfigVersionPtrs))
			for i, envoyConfigVersion := range envoyConfigVersionPtrs {
				envoyConfigVersions[i] = *envoyConfigVersion
				_ = envoyConfigVersions[i].MarshalPrepare()
			}
			snapshot.EnvoyConfigVersions = envoyConfigVersions
		case domain.TlsConfigTable:
			tlsConfigPtrs := entities.([]*domain.TlsConfig)
			tlsConfigs := make([]domain.TlsConfig, len(tlsConfigPtrs))
			for i, tlsConfig := range tlsConfigPtrs {
				tlsConfigs[i] = *tlsConfig
				_ = tlsConfigs[i].MarshalPrepare()
			}
			snapshot.TlsConfigs = tlsConfigs
		case domain.TlsConfigsNodeGroupsTable:
			tlsConfigsNodeGroupsPtrs := entities.([]*domain.TlsConfigsNodeGroups)
			tlsConfigsNodeGroups := make([]domain.TlsConfigsNodeGroups, len(tlsConfigsNodeGroupsPtrs))
			for i, tlsConfig := range tlsConfigsNodeGroupsPtrs {
				tlsConfigsNodeGroups[i] = *tlsConfig
				_ = tlsConfigsNodeGroups[i].MarshalPrepare()
			}
			snapshot.TlsConfigsNodeGroups = tlsConfigsNodeGroups
		case domain.WasmFilterTable:
			wasmFiltersPtrs := entities.([]*domain.WasmFilter)
			wasmFilters := make([]domain.WasmFilter, len(wasmFiltersPtrs))
			for i, wasmFilter := range wasmFiltersPtrs {
				wasmFilters[i] = *wasmFilter
				_ = wasmFilters[i].MarshalPrepare()
			}
			snapshot.WasmFilters = wasmFilters
		case domain.ListenersWasmFilterTable:
			listenerWasmFilterPtrs := entities.([]*domain.ListenersWasmFilter)
			listenerWasmFilters := make([]domain.ListenersWasmFilter, len(listenerWasmFilterPtrs))
			for i, listenerWasmFilter := range listenerWasmFilterPtrs {
				listenerWasmFilters[i] = *listenerWasmFilter
				_ = listenerWasmFilters[i].MarshalPrepare()
			}
			snapshot.ListenerWasmFilters = listenerWasmFilters
		case domain.LuaFilterTable:
			luaFiltersPtrs := entities.([]*domain.LuaFilter)
			luaFilters := make([]domain.LuaFilter, len(luaFiltersPtrs))
			for i, luaFilter := range luaFiltersPtrs {
				luaFilters[i] = *luaFilter
				_ = luaFilters[i].MarshalPrepare()
			}
			snapshot.LuaFilters = luaFilters
		case domain.CompositeSatelliteTable:
			satellitePtrs := entities.([]*domain.CompositeSatellite)
			satellites := make([]domain.CompositeSatellite, len(satellitePtrs))
			for i, satellite := range satellitePtrs {
				satellites[i] = *satellite
				_ = satellites[i].MarshalPrepare()
			}
			snapshot.CompositeSatellites = satellites
		case domain.StatefulSessionTable:
			sessionConfigPtrs := entities.([]*domain.StatefulSession)
			sessionConfigs := make([]domain.StatefulSession, len(sessionConfigPtrs))
			for i, sessionConfig := range sessionConfigPtrs {
				sessionConfigs[i] = *sessionConfig
				_ = sessionConfigs[i].MarshalPrepare()
			}
			snapshot.StatefulSessionConfigs = sessionConfigs
		case domain.RateLimitTable:
			rateLimitPtrs := entities.([]*domain.RateLimit)
			rateLimits := make([]domain.RateLimit, len(rateLimitPtrs))
			for i, rateLimit := range rateLimitPtrs {
				rateLimits[i] = *rateLimit
				_ = rateLimits[i].MarshalPrepare()
			}
			snapshot.RateLimits = rateLimits
		case domain.ExtAuthzFilterTable:
			ptrs := entities.([]*domain.ExtAuthzFilter)
			vals := make([]domain.ExtAuthzFilter, len(ptrs))
			for i, val := range ptrs {
				vals[i] = *val
				_ = vals[i].MarshalPrepare()
			}
			snapshot.ExtAuthzFilters = vals
		case domain.CircuitBreakerTable:
			ptrs := entities.([]*domain.CircuitBreaker)
			vals := make([]domain.CircuitBreaker, len(ptrs))
			for i, val := range ptrs {
				vals[i] = *val
				_ = vals[i].MarshalPrepare()
			}
			snapshot.CircuitBreakers = vals
		case domain.ThresholdTable:
			ptrs := entities.([]*domain.Threshold)
			vals := make([]domain.Threshold, len(ptrs))
			for i, val := range ptrs {
				vals[i] = *val
				_ = vals[i].MarshalPrepare()
			}
			snapshot.Thresholds = vals
		case domain.TcpKeepaliveTable:
			ptrs := entities.([]*domain.TcpKeepalive)
			vals := make([]domain.TcpKeepalive, len(ptrs))
			for i, val := range ptrs {
				vals[i] = *val
				_ = vals[i].MarshalPrepare()
			}
			snapshot.TcpKeepalives = vals
		}
	}
	log.Debugf("Snapshot has been created. Id: %s", snapshot.Id)
	return snapshot, nil
}

func (s *Storage) Restore(snapshot data.Snapshot) error {
	tx := s.WriteTx()
	defer tx.Abort()
	for _, tableName := range domain.TableRelationOrder {
		_, err := tx.DeleteAll(tableName, "id")
		if err != nil {
			log.Errorf("Can't clean entity %s. Cause: %v", tableName, err)
			//return err
		}
		switch tableName {
		case domain.NodeGroupTable:
			for _, entity := range snapshot.NodeGroups {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.DeploymentVersionTable:
			for _, entity := range snapshot.DeploymentVersions {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.ClusterNodeGroupTable:
			for _, entity := range snapshot.ClusterNodeGroups {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.ListenerTable:
			for _, entity := range snapshot.Listeners {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.MicroserviceVersionTable:
			for _, entity := range snapshot.MicroserviceVersions {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.ClusterTable:
			for _, entity := range snapshot.Clusters {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.EndpointTable:
			for _, entity := range snapshot.Endpoints {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.RouteConfigurationTable:
			for _, entity := range snapshot.RouteConfigurations {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.VirtualHostTable:
			for _, entity := range snapshot.VirtualHosts {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.VirtualHostDomainTable:
			for _, entity := range snapshot.VirtualHostDomains {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.RouteTable:
			for _, entity := range snapshot.Routes {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.HeaderMatcherTable:
			for _, entity := range snapshot.HeaderMatchers {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.HashPolicyTable:
			for _, entity := range snapshot.HashPolicies {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.RetryPolicyTable:
			for _, entity := range snapshot.RetryPolicies {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.HealthCheckTable:
			for _, entity := range snapshot.HealthChecks {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.EnvoyConfigVersionTable:
			for _, entity := range snapshot.EnvoyConfigVersions {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.TlsConfigTable:
			for _, entity := range snapshot.TlsConfigs {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.TlsConfigsNodeGroupsTable:
			for _, entity := range snapshot.TlsConfigsNodeGroups {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.WasmFilterTable:
			for _, entity := range snapshot.WasmFilters {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.ListenersWasmFilterTable:
			for _, entity := range snapshot.ListenerWasmFilters {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.LuaFilterTable:
			for _, entity := range snapshot.LuaFilters {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.CompositeSatelliteTable:
			for _, entity := range snapshot.CompositeSatellites {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.StatefulSessionTable:
			for _, entity := range snapshot.StatefulSessionConfigs {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.RateLimitTable:
			for _, entity := range snapshot.RateLimits {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.ExtAuthzFilterTable:
			for _, entity := range snapshot.ExtAuthzFilters {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}

		case domain.CircuitBreakerTable:
			for _, entity := range snapshot.CircuitBreakers {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.ThresholdTable:
			for _, entity := range snapshot.Thresholds {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		case domain.TcpKeepaliveTable:
			for _, entity := range snapshot.TcpKeepalives {
				e := entity
				err := tx.Insert(tableName, &e)
				if err != nil {
					return err
				}
			}
		}
	}
	tx.Commit()
	log.Debugf("In-memory storage has been restored from snapshot: %v", snapshot.Id)
	return nil
}
