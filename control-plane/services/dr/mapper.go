package dr

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
)

// mapper is used to map db notification event
// to custom action that needs to be performed with in-memory or constant storage.
type mapper struct {
	deleteFunc     func(event map[string]interface{}, repo dao.Repository) error
	loadFromDBFunc func(event map[string]interface{}, storage constancy.Storage) (interface{}, error)
}

var mappers = map[string]mapper{
	domain.ClusterTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteClusterByName(event["name"].(string))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindClusterByName(event["name"].(string))
		},
	},
	domain.ClusterNodeGroupTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteClustersNodeGroup(&domain.ClustersNodeGroup{
				ClustersId:     int32(event["clusters_id"].(float64)),
				NodegroupsName: event["nodegroups_name"].(string),
			})
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindClustersNodeGroupByIdAndNodeGroup(int32(event["clusters_id"].(float64)), event["nodegroups_name"].(string))
		},
	},
	domain.CompositeSatelliteTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteCompositeSatellite(event["namespace"].(string))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindCompositeSatelliteByNamespace(event["namespace"].(string))
		},
	},
	domain.DeploymentVersionTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteDeploymentVersion(&domain.DeploymentVersion{Version: event["version"].(string)})
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindDeploymentVersionByName(event["version"].(string))
		},
	},
	domain.EndpointTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			endpoint, err := repo.FindEndpointById(int32(event["id"].(float64)))
			if err != nil {
				return err
			} else if endpoint == nil {
				return nil
			}
			return repo.DeleteEndpoint(endpoint)
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindEndpointById(int32(event["id"].(float64)))
		},
	},
	domain.EnvoyConfigVersionTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			// no-op
			return nil
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return &domain.EnvoyConfigVersion{
				NodeGroup:  event["node_group"].(string),
				EntityType: event["entity_type"].(string),
				Version:    int64(event["version"].(float64)),
			}, nil
		},
	},
	domain.HashPolicyTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			err := repo.DeleteHashPolicyById(int32(event["id"].(float64)))
			return err
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindHashPolicyById(int32(event["id"].(float64)))
		},
	},
	domain.HeaderMatcherTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			err := repo.DeleteHeaderMatcherById(int32(event["id"].(float64)))
			return err
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindHeaderMatcherById(int32(event["id"].(float64)))
		},
	},
	domain.HealthCheckTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			err := repo.DeleteHealthCheckById(int32(event["id"].(float64)))
			return err
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindHealthCheckById(int32(event["id"].(float64)))
		},
	},
	domain.ListenerTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteListenerById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindListenerById(int32(event["id"].(float64)))
		},
	},
	domain.ListenersWasmFilterTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteListenerWasmFilter(&domain.ListenersWasmFilter{
				ListenerId:   int32(event["listener_id"].(float64)),
				WasmFilterId: int32(event["wasm_filter_id"].(float64)),
			})
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return &domain.ListenersWasmFilter{
				ListenerId:   int32(event["listener_id"].(float64)),
				WasmFilterId: int32(event["wasm_filter_id"].(float64)),
			}, nil
		},
	},
	domain.LuaFilterTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteLuaFilterById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindLuaFilterById(int32(event["id"].(float64)))
		},
	},
	domain.ListenersLuaFilterTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteListenerLuaFilter(&domain.ListenersLuaFilter{
				ListenerId:   int32(event["listener_id"].(float64)),
				LuaFilterId: int32(event["lua_filter_id"].(float64)),
			})
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return &domain.ListenersLuaFilter{
				ListenerId:   int32(event["listener_id"].(float64)),
				LuaFilterId: int32(event["luam_filter_id"].(float64)),
			}, nil
		},
	},
	domain.NodeGroupTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteNodeGroupByName(event["name"].(string))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindNodeGroupByName(event["name"].(string))
		},
	},
	domain.RetryPolicyTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteRetryPolicyById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindRetryPolicyById(int32(event["id"].(float64)))
		},
	},
	domain.RouteConfigurationTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteRouteConfigById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindRouteConfigById(int32(event["id"].(float64)))
		},
	},
	domain.RouteTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteRouteById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindRouteById(int32(event["id"].(float64)))
		},
	},
	domain.TlsConfigTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteTlsConfigById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindTlsConfigById(int32(event["id"].(float64)))
		},
	},
	domain.VirtualHostDomainTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			domains, err := repo.FindVirtualHostDomainByVirtualHostId(int32(event["virtualhostid"].(float64)))
			if err != nil {
				return err
			}
			domain := event["domain"].(string)
			for _, existingDomain := range domains {
				if existingDomain.Domain == domain {
					return repo.DeleteVirtualHostsDomain(existingDomain)
				}
			}
			return nil
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return &domain.VirtualHostDomain{
				Domain:        event["domain"].(string),
				Version:       1,
				VirtualHostId: int32(event["virtualhostid"].(float64)),
			}, nil
		},
	},
	domain.VirtualHostTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			vHost, err := repo.FindVirtualHostById(int32(event["id"].(float64)))
			if err != nil {
				return err
			}
			return repo.DeleteVirtualHost(vHost)
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindVirtualHostById(int32(event["id"].(float64)))
		},
	},
	domain.WasmFilterTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteWasmFilterById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindWasmFilterById(int32(event["id"].(float64)))
		},
	},
	domain.StatefulSessionTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteStatefulSessionConfig(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindStatefulSessionById(int32(event["id"].(float64)))
		},
	},
	domain.RateLimitTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteRateLimitByNameAndPriority(event["name"].(string), domain.ConfigPriority(event["priority"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return &domain.RateLimit{
				Name:                   event["name"].(string),
				LimitRequestsPerSecond: uint32(event["limit_per_second"].(float64)),
				Priority:               domain.ConfigPriority(event["priority"].(float64)),
			}, nil
		},
	},
	domain.TlsConfigsNodeGroupsTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteTlsConfigByIdAndNodeGroupName(&domain.TlsConfigsNodeGroups{
				TlsConfigId:   int32(event["tls_config_id"].(float64)),
				NodeGroupName: event["node_group_name"].(string),
			})
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindTlsConfigByIdAndNodeGroupName(int32(event["tls_config_id"].(float64)), event["node_group_name"].(string))
		},
	},
	domain.MicroserviceVersionTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteMicroserviceVersion(
				event["name"].(string),
				msaddr.Namespace{Namespace: event["namespace"].(string)},
				event["initial_version"].(string))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return &domain.MicroserviceVersion{
				Name:                     event["name"].(string),
				Namespace:                event["namespace"].(string),
				DeploymentVersion:        event["deployment_version"].(string),
				InitialDeploymentVersion: event["initial_version"].(string),
			}, nil
		},
	},
	domain.ExtAuthzFilterTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteExtAuthzFilter(event["name"].(string))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindExtAuthzFilterByName(event["name"].(string))
		},
	},
	domain.CircuitBreakerTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteCircuitBreakerById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindCircuitBreakerById(int32(event["id"].(float64)))
		},
	},
	domain.ThresholdTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteThresholdById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindThresholdById(int32(event["id"].(float64)))
		},
	},
	domain.TcpKeepaliveTable: {
		deleteFunc: func(event map[string]interface{}, repo dao.Repository) error {
			return repo.DeleteTcpKeepaliveById(int32(event["id"].(float64)))
		},
		loadFromDBFunc: func(event map[string]interface{}, storage constancy.Storage) (interface{}, error) {
			return storage.FindTcpKeepaliveById(int32(event["id"].(float64)))
		},
	},
}
