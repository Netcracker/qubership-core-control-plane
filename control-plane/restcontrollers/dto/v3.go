package dto

import (
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	constants "github.com/netcracker/qubership-core-lib-go/v3/const"
	"net"
	"net/url"
	"strconv"
	"strings"
)

//swagger:model RouteDeleteRequestV3
type RouteDeleteRequestV3 struct {
	Gateways           []string `json:"gateways"`
	VirtualService     string   `json:"virtualService"`
	RouteDeleteRequest `mapstructure:",squash"`
	Overridden         bool `json:"overridden"`
}

type DomainDeleteRequestV3 struct {
	VirtualService string   `json:"virtualService"`
	Gateway        string   `json:"gateway"`
	Domains        []string `json:"domains"`
}

type RoutingConfigRequestV3 struct {
	Namespace       string           `json:"namespace"`
	Gateways        []string         `json:"gateways"`
	ListenerPort    int              `json:"listenerPort"`
	TlsSupported    bool             `json:"tlsSupported"`
	VirtualServices []VirtualService `json:"virtualServices"`
	Overridden      bool             `json:"overridden"`
}

//swagger:model HttpFiltersConfigRequestV3
type HttpFiltersConfigRequestV3 struct {
	Gateways       []string     `json:"gateways"`
	WasmFilters    []WasmFilter `json:"wasmFilters"`
	ExtAuthzFilter *ExtAuthz    `json:"extAuthzFilter"`
	LuaFilters     []LuaFilter  `json:"luaFilters"`
	Overridden     bool         `json:"overridden"`
}

//swagger:model HttpFiltersDropConfigRequestV3
type HttpFiltersDropConfigRequestV3 struct {
	Gateways       []string                 `json:"gateways"`
	WasmFilters    []map[string]interface{} `json:"wasmFilters"`
	ExtAuthzFilter *ExtAuthz                `json:"extAuthzFilter"`
	LuaFilters     []map[string]interface{} `json:"luaFilters"`
	Overridden     bool                     `json:"overridden"`
}

type RawEndpoint string

func (endpoint RawEndpoint) HostPort() (string, int, error) {
	stringEndpoint := string(endpoint)
	hasScheme := strings.Contains(stringEndpoint, "://")
	if !hasScheme {
		stringEndpoint = constants.SelectUrl("http", "https") + "://" + stringEndpoint
	}
	u, err := url.Parse(stringEndpoint)
	if err != nil {
		return "", 0, err
	}
	var port int
	if u.Port() != "" {
		portUint, err := strconv.ParseUint(u.Port(), 10, 16)
		if err != nil {
			return "", 0, err
		}
		port = int(portUint)
	}
	host := u.Hostname()
	if port == 0 {
		port, err = net.LookupPort("tcp", u.Scheme)
		if err != nil {
			return "", 0, err
		}
	}
	return host, port, nil
}

type ClusterConfigRequestV3 struct {
	Gateways                  []string       `json:"gateways"`
	Name                      string         `json:"name"`
	Endpoints                 []RawEndpoint  `json:"endpoints"`
	TLS                       string         `json:"tls"`
	CircuitBreaker            CircuitBreaker `json:"circuitBreaker"`
	TcpKeepalive              *TcpKeepalive  `json:"tcpKeepalive"`
	Overridden                bool           `json:"overridden"`
	MaxRequestsPerConnection  int            `json:"maxRequestsPerConnection"`
	ConnectionIdleTimeout     *int64         `json:"connectionIdleTimeout"`
}

type CircuitBreaker struct {
	Threshold Threshold `json:"threshold"`
}

type TcpKeepalive struct {
	Probes   int `json:"probes"`
	Time     int `json:"time"`
	Interval int `json:"interval"`
}

type Threshold struct {
	MaxConnections int `json:"maxConnections"`
}

type VirtualService struct {
	Name               string             `json:"name"`
	Hosts              []string           `json:"hosts"`
	RateLimit          string             `json:"rateLimit"`
	AddHeaders         []HeaderDefinition `json:"addHeaders"`
	RemoveHeaders      []string           `json:"removeHeaders"`
	RouteConfiguration RouteConfig        `json:"routeConfiguration"`
	Overridden         bool               `json:"overridden"`
}

//swagger:model ExtAuthz
type ExtAuthz struct {
	Name              string            `json:"name"`
	Destination       RouteDestination  `json:"destination"`
	ContextExtensions map[string]string `json:"contextExtensions"`
	Timeout           *int64            `json:"timeout"`
}

type HeaderDefinition struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type RouteConfig struct {
	Version string    `json:"version"`
	Routes  []RouteV3 `json:"routes"`
}

type RouteV3 struct {
	Destination RouteDestination `json:"destination"`
	Rules       []Rule           `json:"rules"`
}

type RouteDestination struct {
	Cluster        string         `json:"cluster"`
	TlsSupported   bool           `json:"tlsSupported"`
	Endpoint       string         `json:"endpoint"`
	TlsEndpoint    string         `json:"tlsEndpoint"`
	HttpVersion    *int32         `json:"httpVersion"`
	TlsConfigName  string         `json:"tlsConfigName" `
	CircuitBreaker CircuitBreaker `json:"circuitBreaker" `
	TcpKeepalive   *TcpKeepalive  `json:"tcpKeepalive" `
}

type Rule struct {
	Match           RouteMatch         `json:"match"`
	PrefixRewrite   string             `json:"prefixRewrite"`
	HostRewrite     string             `json:"hostRewrite"`
	AddHeaders      []HeaderDefinition `json:"addHeaders"`
	RemoveHeaders   []string           `json:"removeHeaders"`
	Allowed         *bool              `json:"allowed"`
	Timeout         *int64             `json:"timeout"`
	IdleTimeout     *int64             `json:"idleTimeout"`
	StatefulSession *StatefulSession   `json:"statefulSession" yaml:"statefulSession"`
	RateLimit       string             `json:"rateLimit"`
	Deny            *bool              `json:"deny"`
	LuaFilter       string             `json:"luaFilter"`
}

type RouteMatch struct {
	Prefix         string          `json:"prefix"`
	Regexp         string          `json:"regExp"`
	Path           string          `json:"path"`
	HeaderMatchers []HeaderMatcher `json:"headers"`
}

type ActiveDCsV3 struct {
	Protocol       string                     `json:"protocol"`
	HttpPort       *int32                     `json:"httpPort"`
	HttpsPort      *int32                     `json:"httpsPort"`
	PublicGwHosts  []string                   `json:"publicGwHosts"`
	PrivateGwHosts []string                   `json:"privateGwHosts"`
	HealthCheck    *ActiveDCsHealthCheckV3    `json:"healthCheck"`
	RetryPolicy    *ActiveDCsRetryPolicyV3    `json:"retryPolicy"`
	CommonLbConfig *ActiveDCsCommonLbConfigV3 `json:"commonLbConfig"`
}

func (a *ActiveDCsV3) String() string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("ActiveDCsV3{protocol='%s',httpPort=%s,httpsPort=%s,publicGwHosts=%v,privateGwHosts=%v,healthCheck=%s,retryPolicy=%s,commonLbConfig=%s}",
		a.Protocol, toString(a.HttpPort), toString(a.HttpsPort), a.PublicGwHosts, a.PrivateGwHosts, a.HealthCheck.String(), a.RetryPolicy.String(), a.CommonLbConfig.String())
}

type ActiveDCsHealthCheckV3 struct {
	Timeout            int64   `json:"timeout"`
	Interval           int64   `json:"interval"`
	NoTrafficInterval  *int64  `json:"noTrafficInterval"`
	UnhealthyThreshold *uint32 `json:"unhealthyThreshold"`
	UnhealthyInterval  *int64  `json:"unhealthyInterval"`
	HealthyThreshold   *uint32 `json:"healthyThreshold"`
}

func (a *ActiveDCsHealthCheckV3) String() string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{timeout=%d,interval=%d,noTrafficInterval=%s,unhealthyThreshold=%s,unhealthyInterval=%s,healthyThreshold=%s}",
		a.Timeout, a.Interval, toString(a.NoTrafficInterval), toString(a.UnhealthyThreshold), toString(a.UnhealthyInterval), toString(a.HealthyThreshold))
}

type ActiveDCsRetryPolicyV3 struct {
	RetryOn              string                   `json:"retryOn"`
	NumRetries           uint32                   `json:"numRetries"`
	PerTryTimeout        *int64                   `json:"perTryTimeout"`
	RetryBackOff         *ActiveDCsRetryBackOffV3 `json:"retryBackOff"`
	RetriableStatusCodes []uint32                 `json:"retriableStatusCodes"`
}

func (a *ActiveDCsRetryPolicyV3) String() string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{retryOn='%s',numRetries=%d,perTryTimeout=%s,retryBackOff=%s,retriableStatusCodes=%v}",
		a.RetryOn, a.NumRetries, toString(a.PerTryTimeout), a.RetryBackOff.String(), a.RetriableStatusCodes)
}

type ActiveDCsRetryBackOffV3 struct {
	BaseInterval int64 `json:"baseInterval"`
	MaxInterval  int64 `json:"maxInterval"`
}

func (a *ActiveDCsRetryBackOffV3) String() string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{baseInterval='%d',maxInterval=%d}", a.BaseInterval, a.MaxInterval)
}

type ActiveDCsCommonLbConfigV3 struct {
	HealthyPanicThreshold float64 `json:"healthyPanicThreshold"`
}

func (a *ActiveDCsCommonLbConfigV3) String() string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{healthyPanicThreshold='%f'}", a.HealthyPanicThreshold)
}

func toString(value interface{}) string {
	var str string
	switch v := value.(type) {
	case *int32:
		if v == nil {
			return "<nil>"
		}
		str = fmt.Sprintf("%d", *v)
	case *uint32:
		if v == nil {
			return "<nil>"
		}
		str = fmt.Sprintf("%d", *v)
	case *int64:
		if v == nil {
			return "<nil>"
		}
		str = fmt.Sprintf("%d", *v)
	default:
		if v == nil {
			return "<nil>"
		}
		str = fmt.Sprintf("%v", v)
	}
	return str
}

type Tls struct {
	Enabled    bool   `json:"enabled"`
	Insecure   bool   `json:"insecure"`
	TrustedCA  string `json:"trustedCA"`
	ClientCert string `json:"clientCert"`
	PrivateKey string `json:"privateKey"`
	SNI        string `json:"sni"`
}

type TlsConfig struct {
	Name               string   `json:"name"`
	TrustedForGateways []string `json:"trustedForGateways"`
	Tls                *Tls     `json:"tls"`
	Overridden         bool     `json:"overridden"`
}

type WasmFilter struct {
	Name          string                   `json:"name"`
	URL           string                   `json:"url"`
	SHA256        string                   `json:"sha256"`
	TlsConfigName string                   `json:"tlsConfigName"`
	Timeout       int64                    `json:"timeout"`
	Params        []map[string]interface{} `json:"params"`
}

type LuaFilter struct {
	Name          string                   `json:"name"`
    LuaScript     string                   `json:"luaScript"`
}

//swagger:model StatefulSession
type StatefulSession struct {
	Version   string   `json:"version" yaml:"version"`
	Namespace string   `json:"namespace" yaml:"namespace"`
	Cluster   string   `json:"cluster" yaml:"cluster"`
	Hostname  string   `json:"hostname" yaml:"hostname"`
	Gateways  []string `json:"gateways" yaml:"gateways"`
	Port      *int     `json:"port" yaml:"port"`
	Enabled   *bool    `json:"enabled" yaml:"enabled"`
	Cookie    *Cookie  `json:"cookie" yaml:"cookie"`
	// Route is RO field to return in GET StatefulSession response. Ignored in requests.
	Route      *RouteMatcher `json:"route" yaml:"route"`
	Overridden bool          `json:"overridden"`
}

func (r *StatefulSession) IsDeleteRequest() bool {
	return r.Cookie == nil && r.IsEnabled()
}

func (r *StatefulSession) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

func (r *StatefulSession) ToRouteStatefulSession(gateway string) *domain.StatefulSession {
	if r.IsDeleteRequest() {
		return nil
	}

	statefulSession := &domain.StatefulSession{Gateways: []string{gateway}}
	if r.Cookie != nil {
		statefulSession.CookieName = r.Cookie.Name
		if r.Cookie.Path.Valid {
			statefulSession.CookiePath = r.Cookie.Path.String
		}
		statefulSession.CookieTtl = r.Cookie.Ttl
	}
	statefulSession.Enabled = r.Enabled == nil || *r.Enabled
	return statefulSession
}

func (r *StatefulSession) Clone() *StatefulSession {
	clone := *r
	if r.Cookie != nil {
		clone.Cookie = &Cookie{
			Name: r.Cookie.Name,
			Path: r.Cookie.Path,
		}
		if r.Cookie.Ttl != nil {
			ttlVal := *r.Cookie.Ttl
			clone.Cookie.Ttl = &ttlVal
		}
	}
	if r.Enabled != nil {
		enabledVal := *r.Enabled
		clone.Enabled = &enabledVal
	}
	if r.Port != nil {
		portVal := *r.Port
		clone.Port = &portVal
	}
	return &clone
}

//swagger:model RateLimit
type RateLimit struct {
	Name                  string `json:"name"`
	LimitRequestPerSecond int    `json:"limitRequestPerSecond"`
	Priority              string `json:"priority"`
	Overridden            bool   `json:"overridden"`
}

//swagger:model GatewayDeclaration
type GatewayDeclaration struct {
	Name              string             `json:"name"`
	GatewayType       domain.GatewayType `json:"gatewayType"`
	AllowVirtualHosts *bool              `json:"allowVirtualHosts"`
	Exists            *bool              `json:"exists,omitempty"`
	Overridden        bool               `json:"overridden"`
}

func (r GatewayDeclaration) IsDeleteRequest() bool {
	return r.Exists != nil && !*r.Exists
}

//swagger:model ClusterKeepAliveReq
type ClusterKeepAliveReq struct {
	ClusterKey   string        `json:"clusterKey"`
	TcpKeepalive *TcpKeepalive `json:"tcpKeepalive"`
}
