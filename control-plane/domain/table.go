package domain

import "reflect"

const (
	NodeGroupTable            = "node_groups"
	DeploymentVersionTable    = "deployment_versions"
	ClusterNodeGroupTable     = "clusters_node_groups"
	ListenerTable             = "listeners"
	ExtAuthzFilterTable       = "ext_authz_filters"
	MicroserviceVersionTable  = "microservice_versions"
	ClusterTable              = "clusters"
	EndpointTable             = "endpoints"
	RouteConfigurationTable   = "route_configurations"
	VirtualHostTable          = "virtual_hosts"
	VirtualHostDomainTable    = "virtual_host_domains"
	RouteTable                = "routes"
	HeaderMatcherTable        = "header_matchers"
	HashPolicyTable           = "hash_policy"
	RetryPolicyTable          = "retry_policy"
	EnvoyConfigVersionTable   = "envoy_config_version"
	HealthCheckTable          = "health_check"
	TlsConfigTable            = "tls_configs"
	TlsConfigsNodeGroupsTable = "tls_configs_node_groups"
	WasmFilterTable           = "wasm_filters"
	ListenersWasmFilterTable  = "listeners_wasm_filters"
	LuaFilterTable            = "lua_filters"
	CompositeSatelliteTable   = "composite_satellites"
	StatefulSessionTable      = "stateful_session"
	RateLimitTable            = "rate_limits"
	CircuitBreakerTable       = "circuit_breakers"
	TcpKeepaliveTable         = "tcp_keepalive"
	ThresholdTable            = "thresholds"
)

var (
	TableRelationOrder = []string{
		TcpKeepaliveTable,
		ThresholdTable,
		CircuitBreakerTable,
		NodeGroupTable,
		TlsConfigTable,
		TlsConfigsNodeGroupsTable,
		DeploymentVersionTable,
		ClusterTable,
		ClusterNodeGroupTable,
		StatefulSessionTable,
		EndpointTable,
		RouteConfigurationTable,
		ListenerTable,
		ExtAuthzFilterTable,
		WasmFilterTable,
		ListenersWasmFilterTable,
		LuaFilterTable,
		VirtualHostTable,
		VirtualHostDomainTable,
		RouteTable,
		HeaderMatcherTable,
		HashPolicyTable,
		RetryPolicyTable,
		EnvoyConfigVersionTable,
		HealthCheckTable,
		CompositeSatelliteTable,
		RateLimitTable,
		MicroserviceVersionTable,
	}
	TableType = map[string]reflect.Type{
		NodeGroupTable:            reflect.TypeOf((*NodeGroup)(nil)),
		DeploymentVersionTable:    reflect.TypeOf((*DeploymentVersion)(nil)),
		ClusterNodeGroupTable:     reflect.TypeOf((*ClustersNodeGroup)(nil)),
		ListenerTable:             reflect.TypeOf((*Listener)(nil)),
		ExtAuthzFilterTable:       reflect.TypeOf((*ExtAuthzFilter)(nil)),
		MicroserviceVersionTable:  reflect.TypeOf((*MicroserviceVersion)(nil)),
		ClusterTable:              reflect.TypeOf((*Cluster)(nil)),
		EndpointTable:             reflect.TypeOf((*Endpoint)(nil)),
		RouteConfigurationTable:   reflect.TypeOf((*RouteConfiguration)(nil)),
		VirtualHostTable:          reflect.TypeOf((*VirtualHost)(nil)),
		VirtualHostDomainTable:    reflect.TypeOf((*VirtualHostDomain)(nil)),
		RouteTable:                reflect.TypeOf((*Route)(nil)),
		HeaderMatcherTable:        reflect.TypeOf((*HeaderMatcher)(nil)),
		HashPolicyTable:           reflect.TypeOf((*HashPolicy)(nil)),
		RetryPolicyTable:          reflect.TypeOf((*RetryPolicy)(nil)),
		EnvoyConfigVersionTable:   reflect.TypeOf((*EnvoyConfigVersion)(nil)),
		HealthCheckTable:          reflect.TypeOf((*HealthCheck)(nil)),
		TlsConfigTable:            reflect.TypeOf((*TlsConfig)(nil)),
		TlsConfigsNodeGroupsTable: reflect.TypeOf((*TlsConfigsNodeGroups)(nil)),
		WasmFilterTable:           reflect.TypeOf((*WasmFilter)(nil)),
		ListenersWasmFilterTable:  reflect.TypeOf((*ListenersWasmFilter)(nil)),
		LuaFilterTable:            reflect.TypeOf((*LuaFilter)(nil)),
		CompositeSatelliteTable:   reflect.TypeOf((*CompositeSatellite)(nil)),
		StatefulSessionTable:      reflect.TypeOf((*StatefulSession)(nil)),
		RateLimitTable:            reflect.TypeOf((*RateLimit)(nil)),
		CircuitBreakerTable:       reflect.TypeOf((*CircuitBreaker)(nil)),
		ThresholdTable:            reflect.TypeOf((*Threshold)(nil)),
		TcpKeepaliveTable:         reflect.TypeOf((*TcpKeepalive)(nil)),
	}
)
