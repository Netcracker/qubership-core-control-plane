package domain

import (
	"database/sql"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

type GatewayType string

const (
	Ingress GatewayType = "ingress"
	Egress  GatewayType = "egress"
	Mesh    GatewayType = "mesh"
)

type NodeGroup struct {
	Name        string      `bun:",pk" json:"name"`
	GatewayType GatewayType `bun:"gateway_type" json:"gatewayType"`
	// ForbidVirtualHosts indicates whether registering virtual hosts for this gateway is forbidden.
	// Negative form is chosen to represent default value easily - `false` is default.
	ForbidVirtualHosts bool       `bun:"forbid_virtual_hosts" json:"forbidVirtualHosts"`
	Clusters           []*Cluster `bun:"m2m:clusters_node_groups,join:NodeGroup=Cluster" json:"-"`
}

type Listener struct {
	Id                     int32           `bun:",pk" json:"id"`
	Name                   string          `bun:",notnull" json:"name"`
	BindHost               string          `bun:"bindhost,notnull" json:"bindHost"`
	BindPort               string          `bun:"bindport,notnull" json:"bindPort"`
	RouteConfigurationName string          `bun:"routeconfigname,notnull" json:"routeConfigName"`
	Version                int32           `json:"version"`
	NodeGroupId            string          `bun:"nodegroup,notnull" json:"nodeGroup"`
	NodeGroup              *NodeGroup      `bun:"rel:belongs-to,join:nodegroup=name" json:"-"`
	WasmFilters            []WasmFilter    `bun:"m2m:listeners_wasm_filters,join:WasmFilter=Listener,notnull" json:"wasmFilters"`
	WithTls                bool            `bun:"withtls" json:"withTls"`
	ExtAuthzFilter         *ExtAuthzFilter `bun:"-" json:"extAuthzFilter"`
	LuaFilters             []LuaFilter     `bun:"m2m:listeners_lua_filters,join:LuaFilter=Listener,notnull" json:"luaFilters"`
}

func (l Listener) EqualsTo(listener Listener) bool {
	listener.Id = l.Id
	return reflect.DeepEqual(l, listener)
}

type Cluster struct {
	Id               int32           `bun:",pk" json:"id"`
	Name             string          `bun:",notnull" json:"name"`
	LbPolicy         string          `bun:"lbpolicy,notnull" json:"lbPolicy"`
	DiscoveryType    string          `bun:"column:discovery_type,notnull" json:"type"`
	DiscoveryTypeOld string          `bun:"column:type,notnull" json:"-"`
	Version          int32           `bun:",notnull" json:"version"`
	HttpVersion      *int32          `bun:"http_version,nullzero,notnull,default:1" json:"httpVersion"`
	EnableH2         bool            `bun:"enableh2" json:"enableH2"`
	TLSId            int32           `bun:"tls_id,nullzero,notnull" json:"tlsId"`
	TLS              *TlsConfig      `bun:"rel:belongs-to,join:tls_id=id" json:"tlsConfigName"`
	NodeGroups       []*NodeGroup    `bun:"m2m:clusters_node_groups,join:Cluster=NodeGroup,notnull" json:"nodeGroups"`
	Endpoints        []*Endpoint     `bun:"rel:has-many,join:id=clusterid" json:"endpoints"`
	HealthChecks     []*HealthCheck  `bun:"rel:has-many,join:id=clusterid" json:"healthChecks"`
	CommonLbConfig   *CommonLbConfig `bun:"common_lb_config,type:jsonb" json:"commonLbConfig"`
	DnsResolvers     []DnsResolver   `bun:"dns_resolvers,type:jsonb" json:"dnsResolvers"`
	CircuitBreakerId int32           `bun:"circuit_breaker_id,nullzero" json:"circuitBreakerId"`
	CircuitBreaker   *CircuitBreaker `bun:"rel:belongs-to,join:circuit_breaker_id=id" json:"circuitBreaker"`
	TcpKeepaliveId   int32           `bun:"tcp_keepalive_id,nullzero" json:"tcpKeepaliveId"`
	TcpKeepalive     *TcpKeepalive   `bun:"rel:belongs-to,join:tcp_keepalive_id=id" json:"tcpKeepalive"`
}

type CircuitBreaker struct {
	bun.BaseModel `bun:"table:circuit_breakers"`

	Id          int32      `bun:",pk" json:"id"`
	ThresholdId int32      `bun:"threshold_id,nullzero" json:"thresholdId"`
	Threshold   *Threshold `bun:"rel:belongs-to,join:threshold_id=id" json:"threshold"`
}

type TcpKeepalive struct {
	bun.BaseModel `bun:"table:tcp_keepalive"`

	Id       int32 `bun:",pk" json:"id"`
	Probes   int32 `bun:"probes" json:"probes"`
	Time     int32 `bun:"_time" json:"time"`
	Interval int32 `bun:"_interval" json:"interval"`
}

type Threshold struct {
	bun.BaseModel `bun:"table:thresholds"`

	Id             int32 `bun:",pk" json:"id"`
	MaxConnections int32 `bun:"max_connections" json:"maxConnections"`
}

type TlsConfig struct {
	bun.BaseModel `bun:"table:tls_configs"`

	Id         int32        `bun:",pk" json:"id"`
	NodeGroups []*NodeGroup `bun:"m2m:tls_configs_node_groups,join:TlsConfig=NodeGroup" json:"nodeGroups"`
	Name       string       `bun:"name,notnull" json:"name"`
	Enabled    bool         `bun:"enabled" json:"enabled"`
	Insecure   bool         `bun:"insecure" json:"insecure"`
	TrustedCA  string       `bun:"trusted_ca" json:"trusted_ca"`
	ClientCert string       `bun:"client_cert" json:"client_cert"`
	PrivateKey string       `bun:"private_key" json:"private_key"`
	SNI        string       `bun:"sni" json:"sni"`
}

type TlsConfigsNodeGroups struct {
	TlsConfigId   int32      `bun:"tls_config_id,pk" json:"tlsConfigId"`
	TlsConfig     *TlsConfig `bun:"rel:belongs-to,join:tls_config_id=id" json:"tlsConfig"`
	NodeGroupName string     `bun:"node_group_name,pk" json:"nodeGroupName"`
	NodeGroup     *NodeGroup `bun:"rel:belongs-to,join:node_group_name=name" json:"nodeGroup"`
}

func (c TlsConfig) String() string {
	return fmt.Sprintf("TlsConfig{id=%d,name=%s,enabled=%v,insecure=%v,trustedCA=***,sni=%s",
		c.Id, c.Name, c.Enabled, c.Insecure, c.SNI)
}

type ClustersNodeGroup struct {
	ClustersId     int32      `bun:"clusters_id,pk" json:"clustersId"`
	Cluster        *Cluster   `bun:"rel:belongs-to,join:clusters_id=id" json:"cluster"`
	NodegroupsName string     `bun:"nodegroups_name,pk" json:"nodegroupsName"`
	NodeGroup      *NodeGroup `bun:"rel:belongs-to,join:nodegroups_name=name" json:"nodeGroup"`
}

type ListenersWasmFilter struct {
	ListenerId   int32       `bun:"listener_id,pk"`
	Listener     *Listener   `bun:"rel:belongs-to,join:listener_id=id"`
	WasmFilterId int32       `bun:"wasm_filter_id,pk"`
	WasmFilter   *WasmFilter `bun:"rel:belongs-to,join:wasm_filter_id=id"`
}

//swagger:model Endpoint
type Endpoint struct {
	bun.BaseModel `bun:"table:endpoints"`

	Id                       int32              `bun:",pk" json:"id"`
	Address                  string             `bun:",notnull" json:"address"`
	Port                     int32              `bun:",notnull" json:"port"`
	Protocol                 string             `bun:"protocol,notnull" json:"protocol"`
	ClusterId                int32              `bun:"clusterid,nullzero,notnull" json:"clusterId"`
	Cluster                  *Cluster           `bun:"rel:belongs-to,join:clusterid=id" json:"cluster"`
	DeploymentVersion        string             `bun:"deployment_version,nullzero,notnull" json:"deploymentVersion"`
	InitialDeploymentVersion string             `bun:"initialdeploymentversion,notnull" json:"initialDeploymentVersion"`
	DeploymentVersionVal     *DeploymentVersion `bun:"rel:belongs-to,join:deployment_version=version" json:"deploymentVersionVal"`
	HashPolicies             []*HashPolicy      `bun:"rel:has-many,join:id=endpointid" json:"hashPolicies"`
	Hostname                 string             `bun:"hostname" json:"hostname"`
	OrderId                  int32              `bun:"order_id" json:"orderId"`
	StatefulSessionId        int32              `bun:"statefulsessionid,nullzero,notnull" json:"statefulSessionId"`
	StatefulSession          *StatefulSession   `bun:"rel:belongs-to,join:statefulsessionid=id" json:"statefulSession"`
}

type RouteConfiguration struct {
	Id           int32          `bun:",pk" json:"id"`
	Name         string         `bun:",notnull" json:"name"`
	Version      int32          `bun:",notnull" json:"version"`
	NodeGroupId  string         `bun:"nodegroup,notnull" json:"nodeGroupId"`
	NodeGroup    *NodeGroup     `bun:"rel:belongs-to,join:nodegroup=name" json:"nodeGroup"`
	VirtualHosts []*VirtualHost `bun:"rel:has-many,join:id=routeconfigid" json:"virtualHosts"`
}

type DeploymentVersion struct {
	Version     string    `bun:",pk" json:"version"`
	Stage       string    `bun:",notnull" json:"stage"`
	CreatedWhen time.Time `bun:"createdwhen,default:current_timestamp,notnull" json:"createdWhen"`
	UpdatedWhen time.Time `bun:"updatedwhen,default:current_timestamp,notnull" json:"updatedWhen"`
}

type VirtualHost struct {
	Id                     int32                `bun:",pk" json:"id"`
	Name                   string               `bun:",notnull" json:"name"`
	Version                int32                `bun:",notnull" json:"version"`
	RouteConfigurationId   int32                `bun:"routeconfigid,notnull" json:"routeConfigurationId"`
	RouteConfiguration     *RouteConfiguration  `bun:"rel:belongs-to,join:routeconfigid=id" json:"routeConfiguration"`
	Routes                 []*Route             `bun:"rel:has-many,join:id=virtualhostid" json:"routes"`
	Domains                []*VirtualHostDomain `bun:"rel:has-many,join:id=virtualhostid" json:"domains"`
	RequestHeadersToAdd    []Header             `bun:"request_header_to_add,type:jsonb" json:"requestHeadersToAdd"`
	RequestHeadersToRemove []string             `bun:"request_header_to_remove,type:jsonb" json:"requestHeadersToRemove"`
	RateLimitId            string               `bun:"rate_limit_id,nullzero,notnull" json:"rateLimitId"`
	RateLimit              *RateLimit           `bun:"rel:belongs-to,join:rate_limit_id=name" json:"rateLimit"`
}

func (vh *VirtualHost) HasGenericDomain() bool {
	for _, vHostDomain := range vh.Domains {
		if strings.HasPrefix(vHostDomain.Domain, "*") {
			return true
		}
	}
	return false
}

type ExtAuthzFilter struct {
	bun.BaseModel `bun:"table:ext_authz_filters"`

	Name              string            `bun:"name,pk" json:"name"`
	ClusterName       string            `bun:"cluster_name,notnull" json:"clusterName"`
	Timeout           int64             `bun:"timeout" json:"timeout"`
	ContextExtensions map[string]string `bun:"context_extensions,type:jsonb" json:"contextExtensions"`
	NodeGroup         string            `bun:"node_group,notnull" json:"nodeGroup"`
}

type Header struct {
	Name  string
	Value string
}

func (h Header) Equals(elem Header) bool {
	return h.Name == elem.Name && h.Value == elem.Value
}

type VirtualHostDomain struct {
	Domain        string       `bun:",notnull,pk" json:"domain"`
	Version       int32        `bun:",notnull" json:"version"`
	VirtualHostId int32        `bun:"virtualhostid,notnull,pk" json:"virtualHostId"`
	VirtualHost   *VirtualHost `bun:"rel:belongs-to,join:virtualhostid=id" json:"virtualHost"`
}

func (vhd *VirtualHostDomain) Equals(elem *VirtualHostDomain) bool {
	if elem == nil {
		return false
	}
	if vhd == elem {
		return true
	}
	return vhd.VirtualHostId == elem.VirtualHostId && vhd.Domain == elem.Domain
}

type Route struct {
	Id                       int32              `bun:",pk" json:"id"`
	Uuid                     string             `bun:"uuid,notnull,type:varchar,unique" json:"uuid"`
	VirtualHostId            int32              `bun:"virtualhostid,notnull" json:"virtualHostId"`
	VirtualHost              *VirtualHost       `bun:"rel:belongs-to,join:virtualhostid=id" json:"virtualHost"`
	RouteKey                 string             `bun:"routekey,notnull" json:"routeKey"`
	DirectResponseCode       uint32             `bun:"directresponse_status" json:"directResponseCode"`
	Prefix                   string             `bun:"rm_prefix" json:"prefix"`
	Regexp                   string             `bun:"rm_regexp" json:"regexp"`
	Path                     string             `bun:"rm_path" json:"path"`
	ClusterName              string             `bun:"ra_clustername" json:"clusterName"`
	HostRewriteLiteral       string             `bun:"ra_hostrewrite_literal" json:"hostRewriteLiteral"`
	HostRewrite              string             `bun:"ra_hostrewrite" json:"hostRewrite"`
	HostAutoRewrite          NullBool           `bun:"ra_hostautorewrite" json:"hostAutoRewrite" swaggertype:"boolean"`
	PrefixRewrite            string             `bun:"ra_prefixrewrite" json:"prefixRewrite"`
	RegexpRewrite            string             `bun:"ra_regexprewrite" json:"regexpRewrite"`
	PathRewrite              string             `bun:"ra_pathrewrite" json:"pathRewrite"`
	Version                  int32              `bun:",notnull" json:"version"`
	Timeout                  NullInt            `json:"timeout" swaggertype:"integer"`
	IdleTimeout              NullInt            `bun:"idle_timeout" json:"idleTimeout" swaggertype:"integer"`
	DeploymentVersion        string             `bun:"deployment_version,notnull" json:"deploymentVersionString"`
	DeploymentVersionVal     *DeploymentVersion `bun:"rel:belongs-to,join:deployment_version=version" json:"deploymentVersion"`
	InitialDeploymentVersion string             `bun:"initialdeploymentversion,notnull" json:"initialDeploymentVersion"`
	Autogenerated            bool               `bun:"autogenerated" json:"autogenerated"`
	HeaderMatchers           []*HeaderMatcher   `bun:"rel:has-many,join:id=routeid" json:"headerMatchers"`
	HashPolicies             []*HashPolicy      `bun:"rel:has-many,join:id=routeid" json:"hashPolicies"`
	RetryPolicy              *RetryPolicy       `bun:"rel:has-one,join:id=routeid" json:"retryPolicy"`
	RequestHeadersToAdd      []Header           `bun:"request_header_to_add,type:jsonb" json:"requestHeadersToAdd"`
	RequestHeadersToRemove   []string           `bun:"request_header_to_remove,type:jsonb" json:"requestHeadersToRemove"`
	Fallback                 sql.NullBool       `bun:"fallback,type:boolean" swaggertype:"boolean" json:"fallback"`
	RateLimitId              string             `bun:"rate_limit_id,nullzero,notnull" json:"rateLimitId"`
	RateLimit                *RateLimit         `bun:"rel:belongs-to,join:rate_limit_id=name" json:"rateLimit"`
	StatefulSessionId        int32              `bun:"statefulsessionid,nullzero,notnull" json:"statefulSessionId"`
	StatefulSession          *StatefulSession   `bun:"rel:belongs-to,join:statefulsessionid=id" json:"statefulSession"`
	Deny                     bool               `bun:"deny" json:"deny"`
}

// RouteAction is used for route transformation business logic. RouteAction structures are not persisted.
type RouteAction struct {
	ClusterName     string
	HostRewrite     string
	HostAutoRewrite NullBool
	PrefixRewrite   string
	RegexpRewrite   string
	PathRewrite     string
}

type HeaderMatcher struct {
	Id             int32      `bun:",pk" json:"id"`
	Name           string     `bun:",notnull" json:"name"`
	Version        int32      `bun:",notnull" json:"version"`
	ExactMatch     string     `bun:"exactmatch" json:"exactMatch"`
	SafeRegexMatch string     `bun:"saferegexmatch" json:"safeRegexMatch"`
	RangeMatch     RangeMatch `bun:"rangematch,type:jsonb" json:"rangeMatch"`
	PresentMatch   NullBool   `bun:"presentmatch" json:"presentMatch" swaggertype:"boolean"`
	PrefixMatch    string     `bun:"prefixmatch" json:"prefixMatch"`
	SuffixMatch    string     `bun:"suffixmatch" json:"suffixMatch"`
	InvertMatch    bool       `bun:"invertmatch,default:false" json:"invertMatch" swaggertype:"boolean"`
	RouteId        int32      `bun:"routeid,notnull" json:"-"`
	Route          *Route     `bun:"rel:belongs-to,join:routeid=id" json:"-" yaml:"-"`
}

type RangeMatch struct {
	Start NullInt `json:"start" swaggertype:"integer"`
	End   NullInt `json:"end" swaggertype:"integer"`
}

func (h *HeaderMatcher) Equals(elem *HeaderMatcher) bool {
	if elem == nil {
		return false
	}
	if h == elem {
		return true
	}
	return h.Name == elem.Name && h.ExactMatch == elem.ExactMatch
}

func (h *HeaderMatcher) String() string {
	return fmt.Sprintf("HeaderMatcher{id=%d,name=%s,version=%d,exactMatch=%s,safeRegexMatch=%s,rangeMatch=%v,presentMatch=%v,prefixMatch=%s,suffixMatch=%s,invertMatch=%t}",
		h.Id, h.Name, h.Version, h.ExactMatch, h.SafeRegexMatch, h.RangeMatch, h.PresentMatch, h.PrefixMatch, h.SuffixMatch, h.InvertMatch)
}

// CookieTTL has type NullInt. It's needed for gob library.
// pointer of zero-value  is omitted from the transmission, and Slave node gets th nil instead of zero.
// for details see https://pkg.go.dev/encoding/gob#hdr-Encoding_Details

type HashPolicy struct {
	bun.BaseModel `bun:"table:hash_policy"`

	Id                 int32     `bun:",pk" json:"id"`
	HeaderName         string    `bun:"h_headername" json:"headerName"`
	CookieName         string    `bun:"c_name" json:"cookieName"`
	CookieTTL          NullInt   `bun:"c_ttl,type:integer" json:"cookieTTL"` // do not change the type. see above
	CookiePath         string    `bun:"c_path" json:"cookiePath"`
	QueryParamSourceIP NullBool  `bun:"qp_sourceip,type:boolean" swaggertype:"boolean" json:"queryParamSourceIP"`
	QueryParamName     string    `bun:"qp_name" json:"queryParamName"`
	Terminal           NullBool  `bun:"terminal,default:false,type:boolean" swaggertype:"boolean" json:"terminal"`
	RouteId            int32     `bun:"routeid,nullzero" json:"routeId"`
	Route              *Route    `bun:"rel:belongs-to,join:routeid=id" json:"route"`
	EndpointId         int32     `bun:"endpointid,nullzero" json:"endpointId"`
	Endpoint           *Endpoint `bun:"rel:belongs-to,join:endpointid=id" json:"endpoint"`
}

func (h *HashPolicy) Equals(another *HashPolicy) bool {
	if h == another {
		return h.Terminal == another.Terminal
	}
	if len(h.HeaderName) > 0 && h.HeaderName == another.HeaderName {
		return h.Terminal == another.Terminal
	}
	if len(h.CookieName) > 0 && h.CookieName == another.CookieName && h.CookieTTL == another.CookieTTL &&
		h.CookiePath == another.CookiePath {
		return h.Terminal == another.Terminal
	}
	if len(h.QueryParamName) > 0 && h.QueryParamSourceIP.Valid == another.QueryParamSourceIP.Valid && h.QueryParamSourceIP.Bool == another.QueryParamSourceIP.Bool && h.QueryParamName == another.QueryParamName {
		return h.Terminal == another.Terminal
	}
	return false
}

type StatefulSession struct {
	bun.BaseModel `bun:"table:stateful_session"`

	Id         int32  `bun:",pk"`
	CookieName string `bun:"cookie_name"`
	CookieTtl  *int64 `bun:"cookie_ttl"`
	CookiePath string `bun:"cookie_path"`
	Enabled    bool   `bun:"enabled"`

	// ClusterName here is a microservice family name, NOT cluster key: no port included! E.g. `trace-service`.
	ClusterName string   `bun:"clustername"`
	Namespace   string   `bun:"namespace"`
	Gateways    []string `bun:"gateways"`

	DeploymentVersion        string             `bun:"deployment_version,notnull" json:"deploymentVersionString"`
	DeploymentVersionVal     *DeploymentVersion `bun:"rel:belongs-to,join:deployment_version=version" json:"deploymentVersion"`
	InitialDeploymentVersion string             `bun:"initialdeploymentversion,notnull" json:"initialDeploymentVersion"`
}

func (s *StatefulSession) Equals(another *StatefulSession) bool {
	if s == another {
		return true
	}
	if !s.Enabled && !another.Enabled {
		return true
	}
	if s.CookieTtl == nil && another.CookieTtl != nil {
		return false
	}
	if s.CookieTtl != nil {
		if another.CookieTtl == nil || *s.CookieTtl != *another.CookieTtl {
			return false
		}
	}
	return s.Enabled == another.Enabled &&
		s.CookieName == another.CookieName &&
		s.CookiePath == another.CookiePath
}

type RetryPolicy struct {
	bun.BaseModel `bun:"table:retry_policy"`

	Id                            int32         `bun:",pk" json:"id"`
	RetryOn                       string        `bun:"retry_on" json:"retryOn"`
	NumRetries                    uint32        `bun:"num_retries" json:"numRetries"`
	PerTryTimeout                 int64         `bun:"per_try_timeout" json:"perTryTimeout"`
	HostSelectionRetryMaxAttempts int64         `bun:"host_selection_retry_max_attempts" json:"hostSelectionRetryMaxAttempts"`
	RetriableStatusCodes          []uint32      `bun:"retriable_status_codes" json:"retriableStatusCodes"`
	RetryBackOff                  *RetryBackOff `bun:"retry_back_off,type:jsonb" json:"retryBackOff"`
	RouteId                       int32         `bun:"routeid" json:"routeId"`
	Route                         *Route        `bun:"rel:belongs-to,join:routeid=id" json:"route"`
}

type RetryBackOff struct {
	BaseInterval int64 `bun:"base_interval"`
	MaxInterval  int64 `bun:"max_interval"`
}

type HealthCheck struct {
	bun.BaseModel `bun:"table:health_check"`

	Id                           int32            `bun:",pk" json:"id"`
	Timeout                      int64            `bun:"timeout" json:"timeout"`
	Interval                     int64            `bun:"interval" json:"interval"`
	InitialJitter                int64            `bun:"initial_jitter" json:"initialJitter"`
	IntervalJitter               int64            `bun:"interval_jitter" json:"intervalJitter"`
	IntervalJitterPercent        uint32           `bun:"interval_jitterPercent" json:"intervalJitterPercent"`
	UnhealthyThreshold           uint32           `bun:"unhealthy_threshold" json:"unhealthyThreshold"`
	HealthyThreshold             uint32           `bun:"healthy_threshold" json:"healthyThreshold"`
	ReuseConnection              bool             `bun:"reuse_connection" json:"reuseConnection"`
	HttpHealthCheck              *HttpHealthCheck `bun:"http_health_check,type:jsonb" json:"httpHealthCheck"`
	NoTrafficInterval            int64            `bun:"no_traffic_interval" json:"noTrafficInterval"`
	UnhealthyInterval            int64            `bun:"unhealthy_interval" json:"unhealthyInterval"`
	UnhealthyEdgeInterval        int64            `bun:"unhealthy_edge_interval" json:"unhealthyEdgeInterval"`
	HealthyEdgeInterval          int64            `bun:"healthy_edge_interval" json:"healthyEdgeInterval"`
	EventLogPath                 string           `bun:"event_log_path" json:"eventLogPath"`
	AlwaysLogHealthCheckFailures bool             `bun:"always_log_health_check_failures" json:"alwaysLogHealthCheckFailures"`
	TlsOptions                   *TlsOptions      `bun:"tls_options,type:jsonb" json:"tlsOptions"`
	ClusterId                    int32            `bun:"clusterid" json:"clusterId"`
	Cluster                      *Cluster         `bun:"rel:belongs-to,join:clusterid=id" json:"cluster"`
}

type WasmFilter struct {
	Id            int32                  `bun:",pk" json:"id"`
	Name          string                 `bun:"name" json:"name"`
	URL           string                 `bun:"url" json:"url"`
	SHA256        string                 `bun:"sha256" json:"sha256"`
	TlsConfigName string                 `bun:"tls_config_name" json:"tlsConfigName"`
	Timeout       int64                  `bun:"timeout" json:"timeout"`
	Params        map[string]interface{} `bun:"params,type:jsonb" json:"params"`
	Listeners     []Listener             `bun:"m2m:listeners_wasm_filters,join:WasmFilter=Listener" json:"listeners"`
}

func (w WasmFilter) Cluster() (string, error) {
	parse, err := url.Parse(w.URL)
	if err != nil {
		return "", err
	}
	return parse.Host + "-cluster", nil
}

type LuaFilter struct {
	Id            int32                  `bun:",pk" json:"id"`
	Name          string                 `bun:"name" json:"name"`
	URL           string                 `bun:"url" json:"url"`
	SHA256        string                 `bun:"sha256" json:"sha256"`
	HeaderName    string                 `bun:"header_name" json:"headerName"`
    LuaScript     string                 `bun:"lua_script" json:"luaScript"`
    IsActive      bool                   `bun:"is_active" json:"isActive"`
	Timeout       int64                  `bun:"timeout" json:"timeout"`
	Params        map[string]interface{} `bun:"params,type:jsonb" json:"params"`
	Listeners     []Listener             `bun:"m2m:listeners_wasm_filters,join:WasmFilter=Listener" json:"listeners"`
}

func (w LuaFilter) Cluster() (string, error) {
	parse, err := url.Parse(w.URL)
	if err != nil {
		return "", err
	}
	return parse.Host + "-cluster", nil
}

type ListenersLuaFilter struct {
	ListenerId   int32       `bun:"listener_id,pk"`
	Listener     *Listener   `bun:"rel:belongs-to,join:listener_id=id"`
	LuaFilterId  int32       `bun:"lua_filter_id,pk"`
	LuaFilter    *LuaFilter  `bun:"rel:belongs-to,join:lua_filter_id=id"`
}

type HttpHealthCheck struct {
	Id                     int32        `bun:",pk"`
	Host                   string       `bun:"host"`
	Path                   string       `bun:"path"`
	RequestHeadersToAdd    []Header     `bun:"request_headers_to_add,type:jsonb"`
	RequestHeadersToRemove []string     `bun:"request_headers_to_remove,type:jsonb"`
	UseHttp2               bool         `bun:"use_http2"`
	ExpectedStatuses       []RangeMatch `bun:"expected_statuses,type:jsonb" json:"expectedStatuses"`
	CodecClientType        string       `bun:"codec_client_type"`
	HealthCheckId          int32        `bun:"healthcheckid"`
	HealthCheck            *HealthCheck `bun:"rel:belongs-to,join:healthcheckid=id"`
}

type TlsOptions struct {
	AlpnProtocols []string `json:"alpn_protocols"`
}

type CommonLbConfig struct {
	Id                              int32                      `bun:",pk"`
	HealthyPanicThreshold           float64                    `bun:"healthy_panic_threshold"`
	UpdateMergeWindow               *int64                     `bun:"update_merge_window"`
	IgnoreNewHostsUntilFirstHc      bool                       `bun:"ignore_new_hosts_until_first_hc"`
	CloseConnectionsOnHostSetChange bool                       `bun:"close_connections_on_host_set_change"`
	ConsistentHashingLbConfig       *ConsistentHashingLbConfig `bun:"consistent_hashing_lb_config,type:jsonb"`
}

type ConsistentHashingLbConfig struct {
	UseHostnameForHashing bool `bun:"use_hostname_for_hashing"`
}

type EnvoyConfigVersion struct {
	NodeGroup  string `bun:",pk" json:"nodeGroup"`
	EntityType string `bun:",pk" json:"entityType"`
	Version    int64  `bun:",notnull" json:"version"`
}

type SystemProperty struct {
	bun.BaseModel `bun:"table:system_properties"`

	Name    string `bun:",pk"`
	Value   string `bun:"value,notnull"`
	Version int32  `bun:"version"`
}

type DnsResolver struct {
	SocketAddress *SocketAddress
}

type SocketAddress struct {
	Address     string `bun:"address,notnull"`
	Port        uint32 `bun:"port"`
	Protocol    string `bun:"protocol"`
	IPv4_compat bool   `bun:"ipv4_compat"`
}

type CompositeSatellite struct {
	bun.BaseModel `bun:"table:composite_satellites"`

	Namespace string `bun:",pk" json:"namespace"`
}

type Namespace struct {
	bun.BaseModel `bun:"table:namespace"`

	Namespace string `bun:",pk"`
}

type ConfigPriority int32

const (
	Product ConfigPriority = iota
	Project
)

type RateLimit struct {
	bun.BaseModel `bun:"table:rate_limits"`

	Name                   string         `bun:",pk" json:"name"`
	LimitRequestsPerSecond uint32         `bun:"limit_requests_per_second,notnull" json:"limitRequestsPerSecond"`
	Priority               ConfigPriority `bun:",pk,notnull" json:"priority"`
}

func (rl *RateLimit) Clone() *RateLimit {
	rlCopy := *rl
	return &rlCopy
}

func (dv DeploymentVersion) Clone() *DeploymentVersion {
	dvCopy := dv
	return &dvCopy
}

func (dv *DeploymentVersion) String() string {
	return fmt.Sprintf("DeploymentVersion{version=%s,stage=%s}", dv.Version, dv.Stage)
}

func (dv *DeploymentVersion) NumericVersion() (int, error) {
	val, err := strconv.Atoi(strings.TrimPrefix(dv.Version, "v"))
	if err != nil {
		return 0, err
	}
	return val, err
}

func (e Endpoint) Clone() *Endpoint {
	endpointCopy := e
	if e.DeploymentVersionVal != nil {
		verCopy := *e.DeploymentVersionVal
		endpointCopy.DeploymentVersionVal = &verCopy
	}
	if e.Cluster != nil {
		clusterCopy := *e.Cluster
		endpointCopy.Cluster = &clusterCopy
	}
	if e.HashPolicies != nil {
		endpointCopy.HashPolicies = make([]*HashPolicy, len(e.HashPolicies))
		for idx, policy := range e.HashPolicies {
			policyCopy := *policy
			endpointCopy.HashPolicies[idx] = &policyCopy
		}
	}
	if e.StatefulSession != nil {
		endpointCopy.StatefulSession = e.StatefulSession.Clone()
	}
	return &endpointCopy
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("Endpoint{id=%d,address=%s,port=%d,version=%s,initVersion=%s}",
		e.Id, e.Address, e.Port, e.DeploymentVersion, e.InitialDeploymentVersion)
}

func (s *StatefulSession) Clone() *StatefulSession {
	clone := *s
	if s.CookieTtl != nil {
		ttlCopy := *s.CookieTtl
		clone.CookieTtl = &ttlCopy
	}
	if s.DeploymentVersionVal != nil {
		verCopy := *s.DeploymentVersionVal
		clone.DeploymentVersionVal = &verCopy
	}
	return &clone
}

func (r *Route) String() string {
	return fmt.Sprintf("%+v", *r)
}

func (r Route) Clone() *Route {
	routeCopy := r
	if r.DeploymentVersionVal != nil {
		verCopy := *r.DeploymentVersionVal
		routeCopy.DeploymentVersionVal = &verCopy
	}
	if r.HashPolicies != nil {
		routeCopy.HashPolicies = make([]*HashPolicy, len(r.HashPolicies))
		for idx, policy := range r.HashPolicies {
			policyCopy := *policy
			routeCopy.HashPolicies[idx] = &policyCopy
		}
	}
	if r.HeaderMatchers != nil {
		routeCopy.HeaderMatchers = make([]*HeaderMatcher, len(r.HeaderMatchers))
		for idx, matcher := range r.HeaderMatchers {
			matcherCopy := *matcher
			matcherCopy.Route = &routeCopy
			routeCopy.HeaderMatchers[idx] = &matcherCopy
		}
	}
	if r.StatefulSession != nil {
		routeCopy.StatefulSession = r.StatefulSession.Clone()
	}
	if r.RateLimit != nil {
		routeCopy.RateLimit = r.RateLimit.Clone()
	}
	return &routeCopy
}

func (r Route) RouteAction() RouteAction {
	return RouteAction{ClusterName: r.ClusterName, HostRewrite: r.HostRewrite, HostAutoRewrite: r.HostAutoRewrite, PrefixRewrite: r.PrefixRewrite, RegexpRewrite: r.RegexpRewrite, PathRewrite: r.PathRewrite}
}

func (r Route) Merge(route *Route) Route {
	return *route
}

func (r Route) IsProhibit() bool {
	return r.DirectResponseCode == uint32(404)
}

func (c Cluster) String() string {
	return fmt.Sprintf("Cluster{id=%d,name=%s}", c.Id, c.Name)
}

func (vh *VirtualHost) String() string {
	return fmt.Sprintf("VirtualHost{id=%d,name=%s,domains=%v,routes=%v}", vh.Id, vh.Name, vh.Domains, vh.Routes)
}

func (vhd *VirtualHostDomain) String() string {
	return fmt.Sprintf("Domain{%s}", vhd.Domain)
}

func (l Listener) String() string {
	return fmt.Sprintf("Listener{id=%d,name=%s,nodeGroup=%s,bindHost=%s,bindPort=%s,routeConfigurationName=%s}",
		l.Id, l.Name, l.NodeGroupId, l.BindHost, l.BindPort, l.RouteConfigurationName)
}

func (rc RouteConfiguration) String() string {
	return fmt.Sprintf("RouteConfiguration{id=%d,name=%s,nodeGroup=%s,virtualHosts=%s}", rc.Id, rc.Name, rc.NodeGroupId, rc.VirtualHosts)
}

func (cv *EnvoyConfigVersion) String() string {
	return fmt.Sprintf("EnvoyConfigVersion{nodeGroup=%s,entityType=%s,version=%d}", cv.NodeGroup, cv.EntityType, cv.Version)
}

func (ng *NodeGroup) String() string {
	return fmt.Sprintf("NodeGroup{name=%s}", ng.Name)
}

func (p ConfigPriority) String() string {
	switch p {
	case Product:
		return "PRODUCT"
	case Project:
		return "PROJECT"
	default:
		return fmt.Sprintf("CustomPriority{%d}", p)
	}
}

func PriorityFromString(priority string) ConfigPriority {
	if priority == "" || strings.EqualFold("PRODUCT", priority) {
		return Product
	} else if strings.EqualFold("PROJECT", priority) {
		return Project
	} else { // try parse custom priority number
		priorityNum, err := strconv.Atoi(priority)
		if err != nil {
			panic(fmt.Sprintf("domain: could not resolve priority value from '%v'", priority))
		}
		return ConfigPriority(priorityNum)
	}
}

type MicroserviceVersion struct {
	bun.BaseModel `bun:"table:microservice_versions"`

	Name                     string             `bun:"name,pk" json:"name"`
	Namespace                string             `bun:"namespace,pk" json:"namespace"`
	DeploymentVersion        string             `bun:"deployment_version,nullzero,notnull" json:"deploymentVersion"`
	InitialDeploymentVersion string             `bun:"initial_version,notnull,pk" json:"initialDeploymentVersion"`
	DeploymentVersionVal     *DeploymentVersion `bun:"rel:belongs-to,join:deployment_version=version" json:"deploymentVersionVal"`
}

func (m *MicroserviceVersion) Clone() *MicroserviceVersion {
	clone := *m
	if m.DeploymentVersionVal != nil {
		verCopy := *m.DeploymentVersionVal
		clone.DeploymentVersionVal = &verCopy
	}
	return &clone
}
