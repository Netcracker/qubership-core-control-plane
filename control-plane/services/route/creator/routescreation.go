package creator

import (
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/business/format"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/routes"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"strings"
)

const (
	ProhibitedRouteMarker  = "false"
	DefaultTimeoutSpec     = -1
	DefaultIdleTimeoutSpec = -1
)

var (
	logger = logging.GetLogger("config-server-migration")
)

type RouteEntry struct {
	from           string
	to             string
	namespace      string
	timeout        int64
	idleTimeout    int64
	headerMatchers []*domain.HeaderMatcher
}

type RouteRequest struct {
	microserviceUrl string
	isAllowed       bool
}

type RouteEntityRequest struct {
	*RouteRequest
	routes []*RouteEntry
}

func (r RouteEntityRequest) String() string {
	return fmt.Sprintf("RouteEntityRequest{microserviceUrl='%s',isAllowed=%t", r.microserviceUrl, r.isAllowed)
}

func NewRouteEntityRequest(key *RouteRequest, routes []*RouteEntry) *RouteEntityRequest {
	return &RouteEntityRequest{
		RouteRequest: key,
		routes:       routes,
	}
}

func NewRouteEntry(from string, to string, namespace string, timeout int64, idleTimeout int64, headerMatchers []*domain.HeaderMatcher) *RouteEntry {
	return &RouteEntry{
		from:           from,
		to:             to,
		namespace:      namespace,
		timeout:        timeout,
		idleTimeout:    idleTimeout,
		headerMatchers: headerMatchers,
	}
}

func NewRouteRequest(microserviceUrl string, isAllowed bool) RouteRequest {
	return RouteRequest{
		microserviceUrl: microserviceUrl,
		isAllowed:       isAllowed,
	}
}

func (entry *RouteEntry) GetHeaderMatchers() []*domain.HeaderMatcher {
	return entry.headerMatchers
}

func (entry *RouteEntry) GetFrom() string {
	return entry.from
}

func (entry *RouteEntry) GetTo() string {
	return entry.to
}

func (entry *RouteEntry) GetTimeout() int64 {
	return entry.timeout
}

func (entry *RouteEntry) GetIdleTimeout() int64 {
	return entry.idleTimeout
}

func (entry *RouteEntry) GetNamespace() string {
	return entry.namespace
}

func (entry *RouteEntry) IsProhibited() bool {
	return entry.to == ProhibitedRouteMarker
}

func (entry *RouteEntry) String() string {
	return fmt.Sprintf("RouteEntry{from=%s,to=%s,timeout=%d,namespace=%s}", entry.from, entry.to, entry.timeout, entry.namespace)
}

func (r *RouteEntityRequest) GetRoutes() []*RouteEntry {
	return r.routes
}

func (r *RouteEntityRequest) AddRoute(routeEntry *RouteEntry) {
	r.routes = append(r.routes, routeEntry)
}

func (r *RouteEntityRequest) GetMicroserviceUrl() string {
	return r.microserviceUrl
}

func (r *RouteEntityRequest) IsAllowed() bool {
	return r.isAllowed
}

func (entry *RouteEntry) ConfigureAllowedRoute(route *domain.Route) *domain.Route {
	routeFromAddress := format.NewRouteFromAddress(entry.GetFrom())
	routeToAddress := format.NewRouteToAddress(entry)
	prefixRewrite := routeToAddress.GetPrefixRewrite()
	regexpRewrite := routeToAddress.GetRegexpRewrite()
	if route.Prefix != "" && routeFromAddress.RouteFromPrefix != prefixRewrite && routeFromAddress.RouteFromPrefix != prefixRewrite+"/" {
		route.PrefixRewrite = prefixRewrite
	}
	if route.Regexp != "" {
		route.RegexpRewrite = strings.ReplaceAll(regexpRewrite, "\\", "")

		variablesFrom := collectPathVariables(entry.GetFrom())
		var variablesTo []pathVariable
		if entry.GetTo() == "" {
			variablesTo = make([]pathVariable, 0, len(variablesFrom))
			variablesTo = append(variablesTo, variablesFrom...)
		} else {
			variablesTo = collectPathVariables(entry.GetTo())
		}
		route.RegexpRewrite = fillRegexRewriteWithVars(route.RegexpRewrite, variablesFrom, variablesTo)

		// configure route suffix
		for strings.HasSuffix(route.RegexpRewrite, "/") { // remove trailing slash
			route.RegexpRewrite = route.RegexpRewrite[:len(route.RegexpRewrite)-1]
		}
		route.RegexpRewrite = fmt.Sprintf("%s\\%v", route.RegexpRewrite, len(variablesTo)+1)
	}
	return route
}

func (entry *RouteEntry) ConfigureProhibitedRoute(route *domain.Route, isDenied bool) *domain.Route {
	route.PrefixRewrite = ""
	route.RegexpRewrite = ""
	route.DirectResponseCode = 404
	route.Deny = isDenied
	return route
}

func (entry *RouteEntry) IsValidRoute() bool {
	if routes.IsListedInForbiddenRoutes(entry.GetFrom()) {
		logger.Errorf("The request contains a forbidden route: %v", entry.GetFrom())
		return false
	}

	if !entry.IsProhibited() {
		routeFrom := format.NewRouteFromAddress(entry.GetFrom())
		var routeTo *format.RouteFromAddress
		if entry.GetTo() != "" {
			routeTo = format.NewRouteFromAddress(format.DefaultRouteFormatter.GetFromWithoutVariable(entry.GetTo()))
		} else {
			routeTo = format.NewRouteFromAddress(format.DefaultRouteFormatter.GetFromWithoutVariable(entry.GetFrom()))
		}

		return routeFrom.IsValidUrlPath() && routeTo.IsValidUrlPath()
	}
	return routes.IsValidFromUrlPath(entry.GetFrom())
}

func (entry *RouteEntry) CreateRoute(virtualHostId int32, fromPrefix, endpointAddr, clusterName string, timeout int64, idleTimeout int64,
	deploymentVersion, initialDeploymentVersion string,
	headerMatchers []*domain.HeaderMatcher, requestHeadersToAdd []domain.Header, requestHeadersToRemove []string) *domain.Route {
	routeFromAddr := format.NewRouteFromAddress(fromPrefix)
	namespace := msaddr.NewNamespace(entry.namespace)
	route := domain.Route{
		VirtualHostId:            virtualHostId,
		Uuid:                     uuid.New().String(),
		Prefix:                   routeFromAddr.RouteFromPrefix,
		Regexp:                   routeFromAddr.RouteFromRegex,
		HostRewrite:              endpointAddr,
		ClusterName:              clusterName,
		Timeout:                  GetNullIntTimeout(timeout),
		IdleTimeout:              GetNullIntTimeout(idleTimeout),
		HeaderMatchers:           headerMatchers,
		DeploymentVersion:        deploymentVersion,
		InitialDeploymentVersion: initialDeploymentVersion,
		RequestHeadersToAdd:      requestHeadersToAdd,
		RequestHeadersToRemove:   requestHeadersToRemove,
		Version:                  1,
	}

	if !namespace.IsCurrentNamespace() {
		namespaceHeaderMatcher := &domain.HeaderMatcher{
			Name:       "namespace",
			Version:    1,
			ExactMatch: namespace.Namespace,
			Route:      &route,
		}
		route.HeaderMatchers = append(route.HeaderMatchers, namespaceHeaderMatcher)
	}
	return &route
}

func GetInt64Timeout(timeoutPointer *int64) int64 {
	if timeoutPointer == nil {
		return DefaultTimeoutSpec
	}
	return *timeoutPointer
}

func GetNullIntTimeout(timeout int64) domain.NullInt {
	if timeout == DefaultTimeoutSpec {
		return domain.NullInt{NullInt64: sql.NullInt64{}}
	}
	return domain.NullInt{NullInt64: sql.NullInt64{Int64: timeout, Valid: true}}
}

// collectPathVariables returns path variables in order they appear in the provided URL.
func collectPathVariables(url string) []pathVariable {
	variablesTotal := strings.Count(url, "{")
	res := make([]pathVariable, variablesTotal)
	for i := 0; i < variablesTotal; i++ {
		variableName := url[strings.Index(url, "{")+1 : strings.Index(url, "}")]
		res[i] = pathVariable{
			name:        variableName,
			orderNumber: i + 1, // +1 since variable numbers start from 1 in envoy regexp rewrite
		}
		url = strings.Replace(url, "{", "", 1)
		url = strings.Replace(url, "}", "", 1)
	}
	return res
}

type pathVariable struct {
	name        string
	orderNumber int
}

func fillRegexRewriteWithVars(regexpRewrite string, variablesFrom, variablesTo []pathVariable) string {
	for _, varTo := range variablesTo {
		// find first variable with the same name in variablesFrom
		var idxFrom int
		var varFrom pathVariable
		for idxFrom, varFrom = range variablesFrom {
			if varTo.name == varFrom.name {
				break
			}
		}

		v := fmt.Sprintf("\\%v", varFrom.orderNumber)
		regexpRewrite = strings.Replace(regexpRewrite, ".*", v, 1)

		// remove this variable from variablesFrom slice so we will not take it again if variable name is duplicated
		copy(variablesFrom[idxFrom:], variablesFrom[idxFrom+1:])
		variablesFrom = variablesFrom[:len(variablesFrom)-1]
	}
	return regexpRewrite
}
