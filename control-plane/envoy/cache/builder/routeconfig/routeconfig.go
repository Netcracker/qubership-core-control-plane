package routeconfig

import (
	"errors"
	"sort"
	"time"

	eroute "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_type_matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/cluster"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("EnvoyConfigBuilder#route")
}

//go:generate mockgen -source=routeconfig.go -destination=../../../../test/mock/envoy/cache/builder/routeconfig/stub_routeconfig.go -package=mock_routeconfig
type VirtualHostBuilder interface {
	BuildVirtualHosts(routeConfig *domain.RouteConfiguration) ([]*eroute.VirtualHost, error)
}

type RouteBuilder interface {
	BuildRoutes(routes []*domain.Route) ([]*eroute.Route, map[string]*domain.RateLimit, error)
	BuildRoute(route *domain.Route) (*eroute.Route, *domain.RateLimit, error)
}

type RouteBuilderImpl struct {
	dao                     dao.Repository
	props                   *common.EnvoyProxyProperties
	routePreparer           RoutePreparer
	facade                  bool
	hostRewrite             bool
	responseHeadersToRemove []string
}

func NewRouteBuilder(dao dao.Repository, props *common.EnvoyProxyProperties, routePreparer RoutePreparer) *RouteBuilderImpl {
	//responseHeadersToRemove := []string{"server"}
	return &RouteBuilderImpl{dao: dao, props: props, routePreparer: routePreparer}
}

func NewMeshRouteBuilder(dao dao.Repository, props *common.EnvoyProxyProperties, routePreparer RoutePreparer) *RouteBuilderImpl {
	builder := NewRouteBuilder(dao, props, routePreparer)
	builder.facade = true
	return builder
}

func NewEgressRouteBuilder(dao dao.Repository, props *common.EnvoyProxyProperties, routePreparer RoutePreparer) *RouteBuilderImpl {
	builder := NewRouteBuilder(dao, props, routePreparer)
	builder.facade = true
	builder.hostRewrite = true
	return builder
}

func (builder *RouteBuilderImpl) loadRouteRelations(routes []*domain.Route) ([]*domain.Route, error) {
	for _, route := range routes {
		ver, err := builder.dao.FindDeploymentVersion(route.DeploymentVersion)
		if err != nil {
			logger.Errorf("Failed to load route %v deployment version using DAO: %v", route.Id, err)
			return nil, err
		}
		route.DeploymentVersionVal = ver
		headerMatchers, err := builder.dao.FindHeaderMatcherByRouteId(route.Id)
		if err != nil {
			logger.Errorf("Failed to load route %v header matchers using DAO: %v", route.Id, err)
			return nil, err
		}
		route.HeaderMatchers = headerMatchers
		hashPolicies, err := builder.dao.FindHashPolicyByRouteId(route.Id)
		if err != nil {
			logger.Errorf("Failed to load route %v hashPolicy using DAO: %v", route.Id, err)
			return nil, err
		}
		route.HashPolicies = hashPolicies
		retryPolicy, err := builder.dao.FindRetryPolicyByRouteId(route.Id)
		if err != nil {
			logger.Errorf("Failed to load route %v retryPolicy using DAO: %v", route.Id, err)
			return nil, err
		}
		route.RetryPolicy = retryPolicy
		if route.StatefulSessionId != 0 {
			statefulSessionCookie, err := builder.dao.FindStatefulSessionConfigById(route.StatefulSessionId)
			if err != nil {
				logger.Errorf("Failed to load route %v statefulSession using DAO:\n %v", route.Id, err)
				return nil, err
			}
			route.StatefulSession = statefulSessionCookie
		}
		if route.RateLimitId != "" {
			rateLimit, err := builder.dao.FindRateLimitByNameWithHighestPriority(route.RateLimitId)
			if err != nil {
				logger.Errorf("Failed to load route %v rateLimit using DAO:\n %v", route.Id, err)
				return nil, err
			}
			route.RateLimit = rateLimit
		}
		if route.LuaFilterName != "" {
			luaFilter, err := builder.dao.FindLuaFilterByName(route.LuaFilterName)
			if err != nil {
				logger.Errorf("Failed to load route %v lua filter using DAO:\n %v", route.Id, err)
				return nil, err
			}
			route.LuaFilter = luaFilter
		}
	}
	return routes, nil
}

func (builder *RouteBuilderImpl) BuildRoutes(routes []*domain.Route) ([]*eroute.Route, map[string]*domain.RateLimit, error) {
	routes, err := builder.loadRouteRelations(routes)
	if err != nil {
		return nil, nil, err
	}
	preparedRoutes := builder.routePreparer.Prepare(routes)
	result := make([]*eroute.Route, 0, len(preparedRoutes))
	rateLimits := make(map[string]*domain.RateLimit)
	for _, route := range preparedRoutes {
		if route.DeploymentVersionVal.Stage != "ARCHIVED" {
			envoyRoute, rateLimit, err := builder.BuildRoute(route)
			if err != nil {
				return nil, nil, err
			}
			if envoyRoute != nil {
				result = append(result, envoyRoute)
				if rateLimit != nil {
					if _, alreadyContains := rateLimits[rateLimit.Name]; !alreadyContains {
						rateLimits[rateLimit.Name] = rateLimit.Clone()
					}
				}
			}

		}
	}
	return result, rateLimits, nil
}

func (builder *RouteBuilderImpl) BuildRoute(route *domain.Route) (*eroute.Route, *domain.RateLimit, error) {
	envoyRoute := &eroute.Route{
		Match: &eroute.RouteMatch{},
	}

	if route.Prefix != "" {
		envoyRoute.Match.PathSpecifier = &eroute.RouteMatch_Prefix{Prefix: route.Prefix}
	}
	if route.Path != "" {
		envoyRoute.Match.PathSpecifier = &eroute.RouteMatch_Path{Path: route.Path}
	}
	if route.Regexp != "" {
		envoyRoute.Match.PathSpecifier = &eroute.RouteMatch_SafeRegex{
			SafeRegex: &envoy_type_matcher.RegexMatcher{
				Regex: route.Regexp,
			},
		}
	}

	if envoyRoute.Match.PathSpecifier == nil {
		return nil, nil, nil
	}

	if len(route.RequestHeadersToAdd) != 0 {
		envoyRoute.RequestHeadersToAdd = buildHeaderOptions(route.RequestHeadersToAdd)
	}

	if len(route.RequestHeadersToRemove) != 0 {
		envoyRoute.RequestHeadersToRemove = route.RequestHeadersToRemove
	}

	envoyRoute.ResponseHeadersToRemove = builder.responseHeadersToRemove

	if route.HeaderMatchers != nil {
		envoyHeaderMatchers, err := builder.buildHeaderMatchers(route.HeaderMatchers)
		if err != nil {
			return nil, nil, err
		}
		envoyRoute.Match.Headers = envoyHeaderMatchers
	}

	if route.DirectResponseCode != 0 {
		envoyRoute.Action = &eroute.Route_DirectResponse{DirectResponse: &eroute.DirectResponseAction{Status: route.DirectResponseCode}}
	} else {
		envoyRoute.Action = &eroute.Route_Route{
			Route: &eroute.RouteAction{
				ClusterSpecifier: &eroute.RouteAction_Cluster{Cluster: cluster.ReplaceDotsByUnderscore(route.ClusterName)},
			},
		}
		if route.HashPolicies != nil && len(route.HashPolicies) > 0 {
			hashPolicies := make([]*eroute.RouteAction_HashPolicy, 0, len(route.HashPolicies))
			route.HashPolicies = builder.sortHashPolicies(route.HashPolicies)
			for _, hashPolicy := range route.HashPolicies {
				envoyHashPolicy, err := builder.buildRouteHashPolicy(hashPolicy)
				if err != nil {
					return nil, nil, err
				}
				hashPolicies = append(hashPolicies, envoyHashPolicy)
			}
			envoyRoute.GetRoute().HashPolicy = hashPolicies
		}
		if route.StatefulSession != nil {
			statefulSession, err := common.BuildStatefulSessionPerRoute(route.StatefulSession)
			if err != nil {
				return nil, nil, err
			}
			envoyRoute.TypedPerFilterConfig = map[string]*any.Any{
				"envoy.filters.http.stateful_session": statefulSession,
			}
		}
		if route.RetryPolicy != nil {
			retryPolicy, err := builder.buildRouteRetryPolicy(route.RetryPolicy)
			if err != nil {
				return nil, nil, err
			}
			envoyRoute.GetRoute().RetryPolicy = retryPolicy
		}
		if route.PrefixRewrite != "" {
			envoyRoute.GetRoute().PrefixRewrite = route.PrefixRewrite
		}
		if route.RegexpRewrite != "" {
			envoyRoute.GetRoute().RegexRewrite = &envoy_type_matcher.RegexMatchAndSubstitute{
				Pattern: &envoy_type_matcher.RegexMatcher{
					Regex: route.Regexp,
				},
				Substitution: route.RegexpRewrite,
			}
		}

		if route.HostRewriteLiteral != "" {
			envoyRoute.GetRoute().HostRewriteSpecifier = &eroute.RouteAction_HostRewriteLiteral{HostRewriteLiteral: route.HostRewriteLiteral}
		} else {
			if !builder.facade || builder.hostRewrite {
				if route.HostAutoRewrite.Valid {
					envoyRoute.GetRoute().HostRewriteSpecifier = &eroute.RouteAction_AutoHostRewrite{
						AutoHostRewrite: &wrappers.BoolValue{Value: route.HostAutoRewrite.Bool},
					}
				}
				if route.HostRewrite != "" {
					envoyRoute.GetRoute().HostRewriteSpecifier = &eroute.RouteAction_HostRewriteLiteral{HostRewriteLiteral: route.HostRewrite}
				}
			}
		}

		if route.Timeout.Valid && route.Timeout.Int64 >= 0 {
			envoyRoute.GetRoute().Timeout = durationpb.New(time.Duration(route.Timeout.Int64) * time.Millisecond)
		} else {
			envoyRoute.GetRoute().Timeout = durationpb.New(builder.props.Routes.GetTimeout())
		}
		if route.IdleTimeout.Valid && route.IdleTimeout.Int64 >= 0 {
			envoyRoute.GetRoute().IdleTimeout = durationpb.New(time.Duration(route.IdleTimeout.Int64) * time.Millisecond)
		}

		if route.RateLimit != nil {
			envoyRoute.GetRoute().RateLimits = builder.buildRateLimitAction(route.RateLimitId)
		}
		if route.LuaFilter != nil {
			luaConfig, err := common.BuildLuaFilterPerRoute(route.LuaFilter)
			if err != nil {
				return nil, nil, err
			}
			envoyRoute.TypedPerFilterConfig = map[string]*anypb.Any{
				"envoy.filters.http.lua": luaConfig,
			}
		}
	}
	return envoyRoute, route.RateLimit, nil
}

func (builder *RouteBuilderImpl) buildRateLimitAction(rateLimitName string) []*eroute.RateLimit {
	return []*eroute.RateLimit{{
		
		Actions: []*eroute.RateLimit_Action{
			{
				ActionSpecifier: &eroute.RateLimit_Action_GenericKey_{
					GenericKey: &eroute.RateLimit_Action_GenericKey{
						DescriptorValue: rateLimitName,
						DescriptorKey:   "rate_limit_name",
					},
				},
			},
		},
	}}
}

func (builder *RouteBuilderImpl) sortHashPolicies(hashPolicy []*domain.HashPolicy) []*domain.HashPolicy {
	sort.SliceStable(hashPolicy, func(i, j int) bool {
		return hashPolicy[i].Terminal.Bool == true
	})
	return hashPolicy
}

func (builder *RouteBuilderImpl) buildHeaderMatchers(headerMatchers []*domain.HeaderMatcher) ([]*eroute.HeaderMatcher, error) {
	envoyHeaderMatchers := make([]*eroute.HeaderMatcher, 0, len(headerMatchers))
	for _, headerMatcher := range headerMatchers {
		routeHeaderMatcher := &eroute.HeaderMatcher{Name: headerMatcher.Name, InvertMatch: headerMatcher.InvertMatch}
		switch {
		case headerMatcher.SuffixMatch != "":
			routeHeaderMatcher.HeaderMatchSpecifier = &eroute.HeaderMatcher_SuffixMatch{SuffixMatch: headerMatcher.SuffixMatch}
		case headerMatcher.SafeRegexMatch != "":
			routeHeaderMatcher.HeaderMatchSpecifier = &eroute.HeaderMatcher_SafeRegexMatch{SafeRegexMatch: &envoy_type_matcher.RegexMatcher{Regex: headerMatcher.SafeRegexMatch}}
		case headerMatcher.RangeMatch.Start.Valid || headerMatcher.RangeMatch.End.Valid:
			routeHeaderMatcher.HeaderMatchSpecifier = &eroute.HeaderMatcher_RangeMatch{RangeMatch: &envoy_type.Int64Range{Start: headerMatcher.RangeMatch.Start.Int64, End: headerMatcher.RangeMatch.End.Int64}}
		case headerMatcher.PresentMatch.Valid:
			routeHeaderMatcher.HeaderMatchSpecifier = &eroute.HeaderMatcher_PresentMatch{PresentMatch: headerMatcher.PresentMatch.Bool}
		case headerMatcher.PrefixMatch != "":
			routeHeaderMatcher.HeaderMatchSpecifier = &eroute.HeaderMatcher_PrefixMatch{PrefixMatch: headerMatcher.PrefixMatch}
		case headerMatcher.ExactMatch != "":
			routeHeaderMatcher.HeaderMatchSpecifier = &eroute.HeaderMatcher_StringMatch{
				StringMatch: &envoy_type_matcher.StringMatcher{
					MatchPattern: &envoy_type_matcher.StringMatcher_Exact{
						Exact: headerMatcher.ExactMatch,
					},
				}}
		default:
			routeHeaderMatcher.HeaderMatchSpecifier = &eroute.HeaderMatcher_PresentMatch{PresentMatch: true}
		}
		envoyHeaderMatchers = append(envoyHeaderMatchers, routeHeaderMatcher)
	}
	return envoyHeaderMatchers, nil
}

func (builder *RouteBuilderImpl) buildRouteHashPolicy(hashPolicy *domain.HashPolicy) (*eroute.RouteAction_HashPolicy, error) {
	result := &eroute.RouteAction_HashPolicy{Terminal: hashPolicy.Terminal.Bool}
	if hashPolicy.HeaderName != "" {
		result.PolicySpecifier = &eroute.RouteAction_HashPolicy_Header_{
			Header: &eroute.RouteAction_HashPolicy_Header{HeaderName: hashPolicy.HeaderName},
		}
	} else if hashPolicy.CookieName != "" {
		if !hashPolicy.CookieTTL.Valid {
			result.PolicySpecifier = &eroute.RouteAction_HashPolicy_Cookie_{
				Cookie: &eroute.RouteAction_HashPolicy_Cookie{
					Name: hashPolicy.CookieName,
					Path: hashPolicy.CookiePath,
				},
			}
		} else {
			result.PolicySpecifier = &eroute.RouteAction_HashPolicy_Cookie_{
				Cookie: &eroute.RouteAction_HashPolicy_Cookie{
					Name: hashPolicy.CookieName,
					Ttl:  &duration.Duration{Seconds: hashPolicy.CookieTTL.Int64},
					Path: hashPolicy.CookiePath,
				},
			}
		}
	} else if hashPolicy.QueryParamName != "" {
		result.PolicySpecifier = &eroute.RouteAction_HashPolicy_QueryParameter_{
			QueryParameter: &eroute.RouteAction_HashPolicy_QueryParameter{
				Name: hashPolicy.QueryParamName,
			},
		}
	} else if hashPolicy.QueryParamSourceIP.Valid {
		result.PolicySpecifier = &eroute.RouteAction_HashPolicy_ConnectionProperties_{
			ConnectionProperties: &eroute.RouteAction_HashPolicy_ConnectionProperties{
				SourceIp: hashPolicy.QueryParamSourceIP.Bool,
			},
		}
	} else {
		return nil, errors.New("routeconfig: can't create route action hash policy without header, cookie, queryParameter or connectionProperties settings")
	}
	return result, nil
}

func (builder *RouteBuilderImpl) buildRouteRetryPolicy(retryPolicy *domain.RetryPolicy) (*eroute.RetryPolicy, error) {
	result := &eroute.RetryPolicy{
		RetryOn:                       retryPolicy.RetryOn,
		NumRetries:                    &wrappers.UInt32Value{Value: retryPolicy.NumRetries},
		PerTryTimeout:                 util.MillisToDuration(retryPolicy.PerTryTimeout),
		RetryPriority:                 nil,
		RetryHostPredicate:            nil,
		HostSelectionRetryMaxAttempts: retryPolicy.HostSelectionRetryMaxAttempts,
		RetriableStatusCodes:          retryPolicy.RetriableStatusCodes,
	}
	if retryPolicy.RetryBackOff != nil {
		result.RetryBackOff = &eroute.RetryPolicy_RetryBackOff{
			BaseInterval: util.MillisToDuration(retryPolicy.RetryBackOff.BaseInterval),
			MaxInterval:  util.MillisToDuration(retryPolicy.RetryBackOff.MaxInterval),
		}
	}
	return result, nil
}
