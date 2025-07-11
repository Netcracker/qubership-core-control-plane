package ram

import (
	"fmt"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"unicode"
)

var (
	schema = &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			domain.NodeGroupTable: {
				Name: domain.NodeGroupTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
				},
			},
			domain.DeploymentVersionTable: {
				Name: domain.DeploymentVersionTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Version"},
					},
					"stage": {
						Name:    "stage",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "Stage"},
					},
				},
			},
			domain.ClusterNodeGroupTable: {
				Name: domain.ClusterNodeGroupTable,
				Indexes: buildRelationIndex(
					&memdb.IntFieldIndex{Field: "ClustersId"},
					&memdb.StringFieldIndex{Field: "NodegroupsName"},
				),
			},
			domain.ListenerTable: {
				Name: domain.ListenerTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"nodeGroup": {
						Name:         "nodeGroup",
						Unique:       false,
						Indexer:      &memdb.StringFieldIndex{Field: "NodeGroupId"},
						AllowMissing: false,
					},
					"nodeGroupAndName": {
						Name:   "nodeGroupAndName",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "NodeGroupId"},
								&memdb.StringFieldIndex{Field: "Name"},
							},
							AllowMissing: false,
						},
					},
				},
			},
			domain.ExtAuthzFilterTable: {
				Name: domain.ExtAuthzFilterTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
					"nodeGroup": {
						Name:         "nodeGroup",
						Unique:       true,
						Indexer:      &memdb.StringFieldIndex{Field: "NodeGroup"},
						AllowMissing: false,
					},
				},
			},
			domain.ClusterTable: {
				Name: domain.ClusterTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"name": {
						Name:    "name",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
				},
			},
			domain.EndpointTable: {
				Name: domain.EndpointTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"dVersion": {
						Name:    "dVersion",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "DeploymentVersion"},
					},
					"clusterId": {
						Name:    "clusterId",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "ClusterId"},
					},
					"clusterIdAndDVersion": {
						Name:   "clusterIdAndDVersion",
						Unique: false,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.IntFieldIndex{Field: "ClusterId"},
								&memdb.StringFieldIndex{Field: "DeploymentVersion"},
							},
							AllowMissing: false,
						},
					},
					"addressAndPortAndDVersion": {
						Name:   "addressAndPortAndDVersion",
						Unique: false,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "Address"},
								&memdb.IntFieldIndex{Field: "Port"},
								&memdb.StringFieldIndex{Field: "DeploymentVersion"},
							},
							AllowMissing: true,
						},
					},
					"statefulSessionId": {
						Name:    "statefulSessionId",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "StatefulSessionId"},
					},
				},
			},
			domain.RouteConfigurationTable: {
				Name: domain.RouteConfigurationTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"nodeGroup": {
						Name:    "nodeGroup",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "NodeGroupId"},
					},
					"nodeGroupAndName": {
						Name:   "nodeGroupAndName",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "NodeGroupId"},
								&memdb.StringFieldIndex{Field: "Name"},
							},
							AllowMissing: false,
						},
					},
				},
			},
			domain.VirtualHostTable: {
				Name: domain.VirtualHostTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"name": {
						Name:    "name",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
					"routeConfigId": {
						Name:    "routeConfigId",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "RouteConfigurationId"},
					},
					"nameAndRouteConfigId": {
						Name:   "nameAndRouteConfigId",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "Name"},
								&memdb.IntFieldIndex{Field: "RouteConfigurationId"},
							},
							AllowMissing: false,
						},
					},
				},
			},
			domain.VirtualHostDomainTable: {
				Name: domain.VirtualHostDomainTable,
				Indexes: buildRelationIndex(
					&memdb.IntFieldIndex{Field: "VirtualHostId"},
					&memdb.StringFieldIndex{Field: "Domain"},
				),
			},
			domain.RouteTable: {
				Name: domain.RouteTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"uuid": {
						Name:    "uuid",
						Unique:  true,
						Indexer: &memdb.UUIDFieldIndex{Field: "Uuid"},
					},
					"uuid_prefix": {
						Name:    "uuid_prefix",
						Unique:  true,
						Indexer: &memdb.UUIDFieldIndex{Field: "Uuid"},
					},
					"virtualHostId": {
						Name:    "virtualHostId",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "VirtualHostId"},
					},
					"vHostIdAndRouteKey": {
						Name:   "vHostIdAndRouteKey",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.IntFieldIndex{Field: "VirtualHostId"},
								&memdb.StringFieldIndex{Field: "RouteKey"},
							},
							AllowMissing: false,
						},
					},
					"dVersion": {
						Name:    "dVersion",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "DeploymentVersion"},
					},
					"clusterName": {
						Name:         "clusterName",
						Unique:       false,
						AllowMissing: true,
						Indexer:      &memdb.StringFieldIndex{Field: "ClusterName"},
					},
					"clusterNameAndDVersion": {
						Name:   "clusterNameAndDVersion",
						Unique: false,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "ClusterName"},
								&memdb.StringFieldIndex{Field: "DeploymentVersion"},
							},
							AllowMissing: true,
						},
					},
					"autoGenAndDVersion": {
						Name:   "autoGenAndDVersion",
						Unique: false,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.BoolFieldIndex{Field: "Autogenerated"},
								&memdb.StringFieldIndex{Field: "DeploymentVersion"},
							},
							AllowMissing: false,
						},
					},
					"deploymentVersionAndRouteKey": {
						Name:   "deploymentVersionAndRouteKey",
						Unique: false,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "RouteKey"},
								&memdb.StringFieldIndex{Field: "DeploymentVersion"},
							},
							AllowMissing: false,
						},
					},
					"vHostIdAndDeploymentVersion": {
						Name:   "vHostIdAndDeploymentVersion",
						Unique: false,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.IntFieldIndex{Field: "VirtualHostId"},
								&memdb.StringFieldIndex{Field: "DeploymentVersion"},
							},
							AllowMissing: false,
						},
					},
					"statefulSessionId": {
						Name:    "statefulSessionId",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "StatefulSessionId"},
					},
					"rateLimitId": {
						Name:         "rateLimitId",
						Unique:       false,
						AllowMissing: true,
						Indexer:      &memdb.StringFieldIndex{Field: "RateLimitId"},
					},
				},
			},
			domain.HeaderMatcherTable: {
				Name: domain.HeaderMatcherTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:   "id",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.IntFieldIndex{Field: "RouteId"},
								&memdb.StringFieldIndex{Field: "Name"},
							},
							AllowMissing: false,
						},
					},
					"routeId": {
						Name:    "routeId",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "RouteId"},
					},
				},
			},
			domain.HashPolicyTable: {
				Name: domain.HashPolicyTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"endpointId": {
						Name:    "endpointId",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "EndpointId"},
					},
					"routeId": {
						Name:    "routeId",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "RouteId"},
					},
				},
			},
			domain.RetryPolicyTable: {
				Name: domain.RetryPolicyTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"routeId": {
						Name:    "routeId",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "RouteId"},
					},
				},
			},
			domain.HealthCheckTable: {
				Name: domain.HealthCheckTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"clusterId": {
						Name:    "clusterId",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "ClusterId"},
					},
				},
			},
			domain.CompositeSatelliteTable: {
				Name: domain.CompositeSatelliteTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Namespace"},
					},
				},
			},
			domain.EnvoyConfigVersionTable: {
				Name: domain.EnvoyConfigVersionTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:   "id",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "NodeGroup"},
								&memdb.StringFieldIndex{Field: "EntityType"},
							},
							AllowMissing: false,
						},
					},
				},
			},
			domain.TlsConfigTable: {
				Name: domain.TlsConfigTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"name": {
						Name:    "name",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
				},
			},
			domain.TlsConfigsNodeGroupsTable: {
				Name: domain.TlsConfigsNodeGroupsTable,
				Indexes: buildRelationIndex(
					&memdb.IntFieldIndex{Field: "TlsConfigId"},
					&memdb.StringFieldIndex{Field: "NodeGroupName"},
				),
			},
			domain.WasmFilterTable: {
				Name: domain.WasmFilterTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"name": {
						Name:    "name",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
				},
			},
			domain.ListenersWasmFilterTable: {
				Name: domain.ListenersWasmFilterTable,
				Indexes: buildRelationIndex(
					&memdb.IntFieldIndex{Field: "ListenerId"},
					&memdb.IntFieldIndex{Field: "WasmFilterId"},
				),
			},
			domain.LuaFilterTable: {
				Name: domain.LuaFilterTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"name": {
						Name:    "name",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
				},
			},
			domain.ListenersLuaFilterTable: {
				Name: domain.ListenersLuaFilterTable,
				Indexes: buildRelationIndex(
					&memdb.IntFieldIndex{Field: "ListenerId"},
					&memdb.IntFieldIndex{Field: "LuaFilterId"},
				),
			},
			domain.StatefulSessionTable: {
				Name: domain.StatefulSessionTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
					"cookieName": {
						Name:    "cookieName",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "CookieName"},
					},
					"clusterNamespaceAndVersion": {
						Name:   "clusterNamespaceAndVersion",
						Unique: false,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "ClusterName"},
								&memdb.StringFieldIndex{Field: "Namespace"},
								&memdb.StringFieldIndex{Field: "DeploymentVersion"},
							},
							AllowMissing: false,
						},
					},
				},
			},
			domain.RateLimitTable: {
				Name: domain.RateLimitTable,
				Indexes: map[string]*memdb.IndexSchema{
					"name": {
						Name:         "name",
						Unique:       false,
						Indexer:      &memdb.StringFieldIndex{Field: "Name"},
						AllowMissing: false,
					},
					"id": {
						Name:   "id",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{Field: "Name"},
								&memdb.IntFieldIndex{Field: "Priority"},
							},
							AllowMissing: false,
						},
					},
				},
			},
			domain.MicroserviceVersionTable: {
				Name: domain.MicroserviceVersionTable,
				Indexes: buildRelationIndex(
					&memdb.StringFieldIndex{Field: "Name"},
					&memdb.StringFieldIndex{Field: "Namespace"},
					&memdb.StringFieldIndex{Field: "InitialDeploymentVersion"},
				),
			},
			domain.CircuitBreakerTable: {
				Name: domain.CircuitBreakerTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
				},
			},
			domain.ThresholdTable: {
				Name: domain.ThresholdTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
				},
			},
			domain.TcpKeepaliveTable: {
				Name: domain.TcpKeepaliveTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "Id"},
					},
				},
			},
		},
	}
)

func buildRelationIndex(indexers ...memdb.Indexer) map[string]*memdb.IndexSchema {
	result := make(map[string]*memdb.IndexSchema, len(indexers)+1)
	result["id"] = &memdb.IndexSchema{
		Name:    "id",
		Unique:  true,
		Indexer: &memdb.CompoundIndex{Indexes: indexers, AllowMissing: false},
	}
	for _, indexer := range indexers {
		name := lowerFirst(fieldName(indexer))
		result[name] = &memdb.IndexSchema{
			Name:    name,
			Unique:  false,
			Indexer: indexer,
		}
	}
	return result
}

func fieldName(indexer memdb.Indexer) string {
	switch v := indexer.(type) {
	case *memdb.StringFieldIndex:
		return v.Field
	case *memdb.IntFieldIndex:
		return v.Field
	}
	panic(fmt.Sprintf("can not get field name for %v", indexer))
}

func lowerFirst(str string) string {
	for i, l := range str {
		return string(unicode.ToLower(l)) + str[i+1:]
	}
	return ""
}
