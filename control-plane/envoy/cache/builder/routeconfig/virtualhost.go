package routeconfig

import (
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	common_ratelimitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/common/ratelimit/v3"
	corsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/cors/v3"
	ext_authz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	local_ratelimitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	envoy_type_matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"google.golang.org/protobuf/types/known/anypb"
	"math"
	"strings"
)

//go:generate mockgen -source=virtualhost.go -destination=../../../../test/mock/envoy/cache/builder/routeconfig/stub_virtualhost.go -package=mock_routeconfig
type VersionAliasesProvider interface {
	GetVersionAliases() string
}

func buildTypedPerFilterConfig(virtualHost *domain.VirtualHost, routeRateLimits map[string]*domain.RateLimit, buildExtAuthz func(virtualHost *domain.VirtualHost) (*any.Any, error), corsPolicy *any.Any) (map[string]*any.Any, error) {
	filterConfigs := make(map[string]*any.Any, 2)
	extAuthz, err := buildExtAuthz(virtualHost)
	if err != nil {
		return nil, err
	}
	if extAuthz != nil {
		filterConfigs["envoy.filters.http.ext_authz"] = extAuthz
	}
	if virtualHost.RateLimit != nil || len(routeRateLimits) != 0 {
		virtualHostRateLimitFilter, err := buildRateLimitFilter(virtualHost.Name, virtualHost.RateLimit, routeRateLimits)
		if err != nil {
			return nil, err
		}
		filterConfigs["envoy.filters.http.local_ratelimit"] = virtualHostRateLimitFilter
	}

	if corsPolicy != nil {
		filterConfigs["envoy.filters.http.cors"] = corsPolicy
	}

	return filterConfigs, nil
}

func buildHeaderOptions(headers []domain.Header) []*core.HeaderValueOption {
	headerValueOptions := make([]*core.HeaderValueOption, len(headers))
	for index, header := range headers {
		headerValueOption := &core.HeaderValueOption{
			Header: &core.HeaderValue{Key: header.Name, Value: header.Value},
		}
		headerValueOptions[index] = headerValueOption
	}
	return headerValueOptions
}

func buildCustomExtAuthzPerRoute(dao dao.Repository, entityService entity.ServiceInterface, virtualHost *domain.VirtualHost) (*any.Any, error) {
	listeners, err := entityService.FindListenersByRouteConfiguration(dao, virtualHost.RouteConfiguration)
	contextExtensions := make(map[string]string)
	for _, listener := range listeners {
		extAuthzFilter, err := dao.FindExtAuthzFilterByNodeGroup(listener.NodeGroupId)
		if err != nil {
			logger.Errorf("Failed to load extAuthzFilter by listener id %d while building custom ExtAuthzPerRoute filter:\n %v", listener.Id, err)
			return nil, err
		}
		if extAuthzFilter == nil {
			continue
		}
		for key, val := range extAuthzFilter.ContextExtensions {
			contextExtensions[key] = val
		}
	}

	if len(contextExtensions) == 0 {
		return nil, nil
	}

	extAuthzPerRoute := &ext_authz.ExtAuthzPerRoute{
		Override: &ext_authz.ExtAuthzPerRoute_CheckSettings{
			CheckSettings: &ext_authz.CheckSettings{
				ContextExtensions: contextExtensions,
			},
		},
	}
	marshalledExtAuthz, err := ptypes.MarshalAny(extAuthzPerRoute)
	if err != nil {
		logger.Errorf("routeconfig: failed to marshal ExtAuthzPerRoute config to protobuf Any")
		return nil, err
	}
	return marshalledExtAuthz, nil
}

func buildRateLimitFilter(vHostName string, vHostRateLimit *domain.RateLimit, routeRateLimits map[string]*domain.RateLimit) (*any.Any, error) {
	var rateLimitName string
	if vHostRateLimit == nil {
		rateLimitName = vHostName + "-ratelimit"
	} else {
		rateLimitName = vHostRateLimit.Name
	}
	localRateLimit := &local_ratelimitv3.LocalRateLimit{
		StatPrefix: rateLimitName + "-stat",
		FilterEnabled: &core.RuntimeFractionalPercent{
			DefaultValue: &envoy_type.FractionalPercent{
				Numerator:   100,
				Denominator: envoy_type.FractionalPercent_HUNDRED,
			},
			RuntimeKey: rateLimitName + "_enabled",
		},
		FilterEnforced: &core.RuntimeFractionalPercent{
			DefaultValue: &envoy_type.FractionalPercent{
				Numerator:   100,
				Denominator: envoy_type.FractionalPercent_HUNDRED,
			},
			RuntimeKey: rateLimitName + "_enforced",
		},
		Descriptors: make([]*common_ratelimitv3.LocalRateLimitDescriptor, 0, len(routeRateLimits)),
	}
	if vHostRateLimit != nil {
		localRateLimit.TokenBucket = &envoy_type.TokenBucket{
			MaxTokens:     vHostRateLimit.LimitRequestsPerSecond,
			TokensPerFill: &wrappers.UInt32Value{Value: vHostRateLimit.LimitRequestsPerSecond},
			FillInterval:  util.MillisToDuration(1000),
		}
	} else {
		localRateLimit.TokenBucket = &envoy_type.TokenBucket{
			MaxTokens:     math.MaxUint32,
			TokensPerFill: &wrappers.UInt32Value{Value: math.MaxUint32},
			FillInterval:  util.MillisToDuration(1000),
		}
	}
	for _, routeRateLimit := range routeRateLimits {
		localRateLimit.Descriptors = append(localRateLimit.Descriptors, &common_ratelimitv3.LocalRateLimitDescriptor{
			Entries: []*common_ratelimitv3.RateLimitDescriptor_Entry{
				{
					Key:   "rate_limit_name",
					Value: routeRateLimit.Name,
				},
			},
			TokenBucket: &envoy_type.TokenBucket{
				MaxTokens:     routeRateLimit.LimitRequestsPerSecond,
				TokensPerFill: &wrappers.UInt32Value{Value: routeRateLimit.LimitRequestsPerSecond},
				FillInterval:  util.MillisToDuration(1000),
			},
		})
	}
	marshalledFilter, err := anypb.New(localRateLimit)
	if err != nil {
		logger.Errorf("routeconfig: failed to marshal LocalRateLimit config to protobuf Any")
		return nil, err
	}
	return marshalledFilter, nil
}

func convertOrigins(origins string) []*envoy_type_matcher.StringMatcher {
	result := make([]*envoy_type_matcher.StringMatcher, 0)
	for _, origin := range strings.Split(origins, ",") {
		trimmedString := strings.TrimSpace(origin)
		if len(trimmedString) > 0 {
			processedOrigin := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(trimmedString,
				".", "\\."),
				"*", "(?:.*)"),
				":(?:.*)", "(?:(?::\\d*))*")
			strMatcher := &envoy_type_matcher.StringMatcher{
				MatchPattern: &envoy_type_matcher.StringMatcher_SafeRegex{
					SafeRegex: &envoy_type_matcher.RegexMatcher{
						Regex: processedOrigin,
					},
				},
			}
			result = append(result, strMatcher)
		}
	}
	return result
}

func buildCorsPolicy(virtualHost *domain.VirtualHost, vhBuilder *GatewayVirtualHostBuilder) (*any.Any, error) {
	corsPolicy := &corsv3.CorsPolicy{
		MaxAge:                 vhBuilder.maxAge,
		AllowOriginStringMatch: vhBuilder.originStringMatchers,
		AllowCredentials:       &wrappers.BoolValue{Value: true},
		AllowHeaders:           vhBuilder.allowedHeaders,
		AllowMethods:           "OPTIONS, HEAD, GET, PUT, POST, DELETE, PATCH",
		FilterEnabled: &core.RuntimeFractionalPercent{
			DefaultValue: &envoy_type.FractionalPercent{
				Numerator:   100,
				Denominator: envoy_type.FractionalPercent_HUNDRED,
			},
			RuntimeKey: "cors." + virtualHost.Name + ".enabled",
		},
		ShadowEnabled: &core.RuntimeFractionalPercent{
			DefaultValue: &envoy_type.FractionalPercent{
				Numerator:   0,
				Denominator: envoy_type.FractionalPercent_HUNDRED,
			},
			RuntimeKey: "cors." + virtualHost.Name + ".shadow_enabled",
		},
	}
	marshalledCorsPolicy, err := anypb.New(corsPolicy)
	if err != nil {
		logger.Errorf("failed to marshal corsPolicy config to protobuf Any")
		return nil, err
	}
	return marshalledCorsPolicy, nil
}
