package ram

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
)

var (
	tableEntityMap = map[string]func(db constancy.Storage) ([]interface{}, error){
		domain.NodeGroupTable: func(db constancy.Storage) ([]interface{}, error) {
			nodeGroups, err := db.FindAllNodeGroups()
			return util.ToSlice(nodeGroups), err
		},
		domain.DeploymentVersionTable: func(db constancy.Storage) ([]interface{}, error) {
			dVersions, err := db.FindAllDeploymentVersions()
			return util.ToSlice(dVersions), err
		},
		domain.ClusterTable: func(db constancy.Storage) ([]interface{}, error) {
			clusters, err := db.FindAllClusters()
			return util.ToSlice(clusters), err
		},
		domain.ClusterNodeGroupTable: func(db constancy.Storage) ([]interface{}, error) {
			nodeGroupClusters, err := db.FindAllClustersNodeGroups()
			return util.ToSlice(nodeGroupClusters), err
		},
		domain.EndpointTable: func(db constancy.Storage) ([]interface{}, error) {
			endpoints, err := db.FindAllEndpoints()
			return util.ToSlice(endpoints), err
		},
		domain.ListenerTable: func(db constancy.Storage) ([]interface{}, error) {
			listeners, err := db.FindAllListeners()
			return util.ToSlice(listeners), err
		},
		domain.RouteConfigurationTable: func(db constancy.Storage) ([]interface{}, error) {
			routeConfigs, err := db.FindAllRouteConfigs()
			return util.ToSlice(routeConfigs), err
		},
		domain.VirtualHostTable: func(db constancy.Storage) ([]interface{}, error) {
			virtualHosts, err := db.FindAllVirtualHosts()
			return util.ToSlice(virtualHosts), err
		},
		domain.VirtualHostDomainTable: func(db constancy.Storage) ([]interface{}, error) {
			virtualHostsDomains, err := db.FindAllVirtualHostsDomains()
			return util.ToSlice(virtualHostsDomains), err
		},
		domain.RouteTable: func(db constancy.Storage) ([]interface{}, error) {
			routes, err := db.FindAllRoutes()
			return util.ToSlice(routes), err
		},
		domain.HeaderMatcherTable: func(db constancy.Storage) ([]interface{}, error) {
			headerMatchers, err := db.FindAllHeaderMatchers()
			return util.ToSlice(headerMatchers), err
		},
		domain.HashPolicyTable: func(db constancy.Storage) ([]interface{}, error) {
			hashPolicies, err := db.FindAllHashPolicies()
			return util.ToSlice(hashPolicies), err
		},
		domain.RetryPolicyTable: func(db constancy.Storage) ([]interface{}, error) {
			retryPolicies, err := db.FindAllRetryPolicies()
			return util.ToSlice(retryPolicies), err
		},
		domain.EnvoyConfigVersionTable: func(db constancy.Storage) ([]interface{}, error) {
			versions, err := db.FindAllEnvoyConfigVersions()
			return util.ToSlice(versions), err
		},
		domain.HealthCheckTable: func(db constancy.Storage) ([]interface{}, error) {
			healthChecks, err := db.FindAllHealthChecks()
			return util.ToSlice(healthChecks), err
		},
		domain.TlsConfigTable: func(db constancy.Storage) ([]interface{}, error) {
			tlsConfigs, err := db.FindAllTlsConfigs()
			return util.ToSlice(tlsConfigs), err
		},
		domain.TlsConfigsNodeGroupsTable: func(db constancy.Storage) ([]interface{}, error) {
			tlsConfigsNodeGroups, err := db.FindAllTlsConfigsNodeGroups()
			return util.ToSlice(tlsConfigsNodeGroups), err
		},
		domain.WasmFilterTable: func(db constancy.Storage) ([]interface{}, error) {
			wasmFilters, err := db.FindWasmFilters()
			return util.ToSlice(wasmFilters), err
		},
		domain.ListenersWasmFilterTable: func(db constancy.Storage) ([]interface{}, error) {
			listenerWasmFilters, err := db.FindAllListenerWasmFilters()
			return util.ToSlice(listenerWasmFilters), err
		},
		domain.CompositeSatelliteTable: func(db constancy.Storage) ([]interface{}, error) {
			satellites, err := db.FindAllCompositeSatellites()
			return util.ToSlice(satellites), err
		},
		domain.StatefulSessionTable: func(db constancy.Storage) ([]interface{}, error) {
			cookies, err := db.FindAllStatefulSessionConfigs()
			return util.ToSlice(cookies), err
		},
		domain.RateLimitTable: func(db constancy.Storage) ([]interface{}, error) {
			rateLimits, err := db.FindAllRateLimits()
			return util.ToSlice(rateLimits), err
		},
		domain.MicroserviceVersionTable: func(db constancy.Storage) ([]interface{}, error) {
			msVersions, err := db.FindAllMicroserviceVersions()
			return util.ToSlice(msVersions), err
		},
		domain.ExtAuthzFilterTable: func(db constancy.Storage) ([]interface{}, error) {
			msVersions, err := db.FindAllExtAuthzFilters()
			return util.ToSlice(msVersions), err
		},
		domain.CircuitBreakerTable: func(db constancy.Storage) ([]interface{}, error) {
			msVersions, err := db.FindAllCircuitBreakers()
			return util.ToSlice(msVersions), err
		},
		domain.ThresholdTable: func(db constancy.Storage) ([]interface{}, error) {
			msVersions, err := db.FindAllThresholds()
			return util.ToSlice(msVersions), err
		},
		domain.TcpKeepaliveTable: func(db constancy.Storage) ([]interface{}, error) {
			entities, err := db.FindAllTcpKeepalives()
			return util.ToSlice(entities), err
		},
		domain.LuaFilterTable: func(db constancy.Storage) ([]interface{}, error) {
			luaFilters, err := db.FindLuaFilters()
			return util.ToSlice(luaFilters), err
		},
		domain.ListenersLuaFilterTable: func(db constancy.Storage) ([]interface{}, error) {
			listenerLuaFilters, err := db.FindAllListenerLuaFilters()
			return util.ToSlice(listenerLuaFilters), err
		},
	}
)

type StorageLoader struct {
	PersistentStorage constancy.Storage
}

func (l *StorageLoader) loadEntity(tx Txn, table string, loadFunc func(db constancy.Storage) ([]interface{}, error)) error {
	entities, err := loadFunc(l.PersistentStorage)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		err := tx.Insert(table, entity)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *StorageLoader) Load(storage *Storage) error {
	inMemDbTx := storage.WriteTx()
	defer inMemDbTx.Abort()
	for _, entity := range domain.TableRelationOrder {
		loadFunc := tableEntityMap[entity]
		err := l.loadEntity(inMemDbTx, entity, loadFunc)
		if err != nil {
			return err
		}
	}
	inMemDbTx.Commit()
	return nil
}

func (l *StorageLoader) ClearAndLoad(storage RamStorage) error {
	inMemDbTx := storage.WriteTx()
	defer inMemDbTx.Abort()
	for _, tableName := range domain.TableRelationOrder {
		_, err := inMemDbTx.DeleteAll(tableName, "id")
		if err != nil {
			return err
		}
	}
	for _, entity := range domain.TableRelationOrder {
		loadFunc := tableEntityMap[entity]
		err := l.loadEntity(inMemDbTx, entity, loadFunc)
		if err != nil {
			return err
		}
	}
	inMemDbTx.Commit()
	return nil
}
