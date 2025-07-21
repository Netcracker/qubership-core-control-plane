package data

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"

	"github.com/google/uuid"
)

type Snapshot struct {
	Id string `json:"id"`
	//Data map[string]interface{}
	NodeGroups             []domain.NodeGroup            `json:"nodeGroups"`
	DeploymentVersions     []domain.DeploymentVersion    `json:"deploymentVersions"`
	ClusterNodeGroups      []domain.ClustersNodeGroup    `json:"clusterNodeGroups"`
	Listeners              []domain.Listener             `json:"listeners"`
	ExtAuthzFilters        []domain.ExtAuthzFilter       `json:"extAuthzFilters"`
	MicroserviceVersions   []domain.MicroserviceVersion  `json:"microserviceVersions"`
	Clusters               []domain.Cluster              `json:"clusters"`
	Endpoints              []domain.Endpoint             `json:"endpoints"`
	RouteConfigurations    []domain.RouteConfiguration   `json:"routeConfigurations"`
	VirtualHosts           []domain.VirtualHost          `json:"virtualHosts"`
	VirtualHostDomains     []domain.VirtualHostDomain    `json:"virtualHostDomains"`
	Routes                 []domain.Route                `json:"routes"`
	HeaderMatchers         []domain.HeaderMatcher        `json:"headerMatchers"`
	HashPolicies           []domain.HashPolicy           `json:"hashPolicies"`
	RetryPolicies          []domain.RetryPolicy          `json:"retryPolicies"`
	EnvoyConfigVersions    []domain.EnvoyConfigVersion   `json:"envoyConfigVersions"`
	HealthChecks           []domain.HealthCheck          `json:"healthChecks"`
	TlsConfigs             []domain.TlsConfig            `json:"tlsConfigs"`
	TlsConfigsNodeGroups   []domain.TlsConfigsNodeGroups `json:"tlsConfigsNodeGroups"`
	WasmFilters            []domain.WasmFilter           `json:"wasmFilters"`
	ListenerWasmFilters    []domain.ListenersWasmFilter  `json:"listenerWasmFilters"`
	LuaFilters             []domain.LuaFilter            `json:"luaFilters"`
	CompositeSatellites    []domain.CompositeSatellite   `json:"compositeSatellites"`
	StatefulSessionConfigs []domain.StatefulSession      `json:"statefulSessionConfigs"`
	RateLimits             []domain.RateLimit            `json:"rateLimits"`
	CircuitBreakers        []domain.CircuitBreaker       `json:"circuitBreakers"`
	Thresholds             []domain.Threshold            `json:"thresholds"`
	TcpKeepalives          []domain.TcpKeepalive         `json:"tcpKeepalives"`
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		Id: uuid.New().String(),
	}
}

type RestorableStorage interface {
	Backup() (*Snapshot, error)
	Restore(snapshot Snapshot) error
}
