package listener

import (
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	gzip "github.com/envoyproxy/go-control-plane/envoy/extensions/compression/gzip/compressor/v3"
	compressor "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/compressor/v3"
	grpcstats "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_stats/v3"
	lua "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const gatewayDefaultHost = "gateway"
const gatewayStatPrefix = "ingress_http"

type GatewayListenerBuilder struct {
	BaseListenerBuilder
	properties *common.EnvoyProxyProperties
}

func NewGatewayListenerBuilder(properties *common.EnvoyProxyProperties) *GatewayListenerBuilder {
	b := GatewayListenerBuilder{properties: properties}
	baseBuilder := BaseListenerBuilder{
		defaultHost:       gatewayDefaultHost,
		statPrefix:        gatewayStatPrefix,
		tracingProperties: properties.Tracing,
		enrichConnManager: b.enrichConnManager,
		enrichListener:    b.enrichListener,
	}
	b.BaseListenerBuilder = baseBuilder
	return &b
}

func (builder *GatewayListenerBuilder) enrichConnManager(connManager *hcm.HttpConnectionManager, namespaceMapping string) error {
	if err := addCorsFilter(connManager); err != nil {
		return err
	}
	if err := addGrpcStatsFilter(connManager); err != nil {
		return err
	}
	if err := addNamespaceHeaderFilter(connManager, namespaceMapping); err != nil {
		return err
	}
	if err := 	addUuidHeaderFilter(connManager); err != nil {
		return err
	}

	return addCompression(connManager, builder.properties.Compression)
}

func (builder *GatewayListenerBuilder) enrichListener(listener *listener.Listener) error {
	listener.PerConnectionBufferLimitBytes = builder.properties.Connection.GetUInt32PerConnectionBufferLimitMBytes()
	return nil
}

func addCompression(connManager *hcm.HttpConnectionManager, properties *common.CompressionProperties) error {
	if properties.Enabled {
		gzipConfig, err := ptypes.MarshalAny(&gzip.Gzip{})
		if err != nil {
			return err
		}
		marshalled, err := ptypes.MarshalAny(&compressor.Compressor{
			CompressorLibrary: &core.TypedExtensionConfig{
				Name:        "envoy.compression.gzip.compressor",
				TypedConfig: gzipConfig,
			},
		})
		if err != nil {
			return err
		}
		connManager.HttpFilters = append(connManager.HttpFilters, &hcm.HttpFilter{
			Name:       "envoy.filters.http.compressor",
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalled},
		})
	}
	return nil
}

func addUuidHeaderFilter(connManager *hcm.HttpConnectionManager) error {
		luaScript := "function envoy_on_request(request_handle)\n" +
			"  local path = request_handle:headers():get(\":path\")\n" +
			"  local match = string.match(path, \"([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\")\n" +
			"    if match then\n" +
			"        request_handle:headers():add(\"x-uuid\", match)\n" +
			"  end\n" +
			"end"
		luaConfig := &lua.Lua{
			InlineCode: luaScript,
		}
		marshalled, err := ptypes.MarshalAny(luaConfig)
		if err != nil {
			return err
		}
		connManager.HttpFilters = append(connManager.HttpFilters, &hcm.HttpFilter{
			Name:       wellknown.Lua,
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalled},
		})
	return nil
}

func addNamespaceHeaderFilter(connManager *hcm.HttpConnectionManager, namespaceMapping string) error {
	if len(namespaceMapping) > 0 {
		luaScript := namespaceMapping + "\n" +
			"function envoy_on_request(request_handle)\n" +
			"  tenant_id = request_handle:headers():get(\"Tenant\")\n" +
			"  if tenantsNS[tenant_id] ~= nil then\n" +
			"    request_handle:headers():add(\"namespace\", tenantsNS[tenant_id])\n" +
			"  end\n" +
			"end"
		luaConfig := &lua.Lua{
			InlineCode: luaScript,
		}
		marshalled, err := ptypes.MarshalAny(luaConfig)
		if err != nil {
			return err
		}
		connManager.HttpFilters = append(connManager.HttpFilters, &hcm.HttpFilter{
			Name:       wellknown.Lua,
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalled},
		})
	}
	return nil
}

func addGrpcStatsFilter(connManager *hcm.HttpConnectionManager) error {
	filter := &grpcstats.FilterConfig{
		EmitFilterState: false,
		PerMethodStatSpecifier: &grpcstats.FilterConfig_StatsForAllMethods{
			StatsForAllMethods: wrapperspb.Bool(true),
		},
		EnableUpstreamStats: true,
	}
	marshalledFilter, err := ptypes.MarshalAny(filter)
	if err != nil {
		return err
	}
	connManager.HttpFilters = append(connManager.HttpFilters, &hcm.HttpFilter{
		Name:       wellknown.HTTPGRPCStats,
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledFilter},
	})
	return nil
}
