package listener

import (
	"encoding/json"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoyConfigTraceV3 "github.com/envoyproxy/go-control-plane/envoy/config/trace/v3"
	fileaccesslog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	corsV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/cors/v3"
	extauthz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	h2m "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/header_to_metadata/v3"
	local_ratelimitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	routerV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	wasmFiltersV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	tlsInspectorV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	wasmV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	etype "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	luaFiltersV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"strconv"
	"strings"
	"time"
)

const accessLogPath = "/dev/stdout"
const accessLogFormat = "[%START_TIME(%FT%T.%3f)%] [INFO] [request_id=%REQ(X-REQUEST-ID)%] [tenant_id=%REQ(Tenant)%] [thread=-] [class=-] envoyLog \n" +
	"Forward Request: \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" \n" +
	"Forward To: \"%UPSTREAM_HOST% %UPSTREAM_CLUSTER%\" \n" +
	"Forward For: \"%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT%\"\n" +
	"Forward Address: \"%DOWNSTREAM_REMOTE_ADDRESS%\"\n" +
	"Response: \"%RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT% %DURATION%ms\"\n" +
	"X-FORWARDED-FOR: \"%REQ(X-FORWARDED-FOR)%\" X-FORWARDED-PROTO: \"%REQ(X-FORWARDED-PROTO)%\"\n" +
	"Access-Control-Allow-Headers: %RESP(Access-Control-Allow-Headers)% ORIGIN: %REQ(Origin)%\n" +
	"Access-Control-Allow-Origin: %RESP(Access-Control-Allow-Origin)%\n"
const listenerTlsPort = 8443

var logger logging.Logger

func init() {
	logger = logging.GetLogger("EnvoyConfigBuilder#listener")
}

//go:generate mockgen -source=listener.go -destination=../../../../test/mock/envoy/cache/builder/listener/stub_listener.go -package=mock_listener
type ListenerBuilder interface {
	BuildListener(listener *domain.Listener, namespaceMapping string, withTls bool) (*v3listener.Listener, error)
}

type BaseListenerBuilder struct {
	defaultHost       string
	statPrefix        string
	tracingProperties *common.TracingProperties
	enrichConnManager func(connManager *hcm.HttpConnectionManager, namespaceMapping string) error
	enrichListener    func(*v3listener.Listener) error
}

func (b BaseListenerBuilder) BuildListener(originalListener *domain.Listener, namespaceMapping string, withTls bool) (*v3listener.Listener, error) {
	connManager, err := b.buildHttpConnectionManager(originalListener, namespaceMapping)
	if err != nil {
		return nil, err
	}
	listener, err := newListener(connManager, originalListener, withTls)
	if err != nil {
		return nil, err
	}
	err = b.enrichListener(listener)
	return listener, err
}

func (b BaseListenerBuilder) buildHttpConnectionManager(originalListener *domain.Listener, namespaceMapping string) (*hcm.HttpConnectionManager, error) {
	connManager, err := buildBaseHttpConnectionManager(originalListener.RouteConfigurationName, b.defaultHost, b.statPrefix)
	if err != nil {
		logger.Errorf("Failed to build facade listener due to error in HttpConnectionManager building: %v", err)
		return nil, err
	}

	if originalListener.ExtAuthzFilter != nil {
		if err := addExtAuthzFilter(connManager, originalListener.ExtAuthzFilter); err != nil {
			return nil, err
		}
	}

	if err := addHeaderToMetadataFilter(connManager); err != nil {
		logger.Errorf("Failed to build facade listener due to error in adding header to metadata filter: %v", err)
		return nil, err
	}
	if err := addTracing(connManager, b.tracingProperties); err != nil {
		return nil, err
	}
	if len(originalListener.WasmFilters) != 0 {
		for _, wf := range originalListener.WasmFilters {
			err = addWASMFilter(connManager, &wf)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := addDisabledLuaFilter(connManager); err != nil {
		return nil, err
	}

	if err := addStatefulSessionFilter(connManager); err != nil {
		return nil, err
	}
	if err := addDisabledRateLimit(connManager); err != nil {
		return nil, err
	}

	if err := b.enrichConnManager(connManager, namespaceMapping); err != nil {
		logger.Errorf("Error modifying http connection manager by concrete builder:\n %v", err)
		return nil, err
	}

	if err := doTerminalStep(connManager); err != nil {
		logger.Errorf("Error in listener builder terminal step:\n %v", err)
		return nil, err
	}
	return connManager, nil
}

func buildBaseHttpConnectionManager(listenerRouteConfigName, defaultHost, statPrefix string) (*hcm.HttpConnectionManager, error) {
	// access log service configuration
	alsConfig := &fileaccesslog.FileAccessLog{
		Path: accessLogPath,
		AccessLogFormat: &fileaccesslog.FileAccessLog_LogFormat{
			LogFormat: &core.SubstitutionFormatString{
				Format: &core.SubstitutionFormatString_TextFormatSource{
					TextFormatSource: &core.DataSource{
						Specifier: &core.DataSource_InlineString{
							InlineString: accessLogFormat,
						},
					},
				},
			},
		},
	}
	alsConfigPbst, err := ptypes.MarshalAny(alsConfig)
	if err != nil {
		return nil, err
	}

	manager := &hcm.HttpConnectionManager{
		ServerHeaderTransformation: hcm.HttpConnectionManager_PASS_THROUGH, // According to security requirements gateway must remove "server" response header whether it is set by envoy or upstream service
		HttpFilters:                make([]*hcm.HttpFilter, 0),
		MergeSlashes:               true,
		HttpProtocolOptions: &core.Http1ProtocolOptions{
			AcceptHttp_10:         true,
			DefaultHostForHttp_10: defaultHost,
		},
		UpgradeConfigs: []*hcm.HttpConnectionManager_UpgradeConfig{{
			UpgradeType: "websocket",
		}},
		AccessLog: []*accesslog.AccessLog{{
			Name:       "envoy.access_loggers.file",
			ConfigType: &accesslog.AccessLog_TypedConfig{TypedConfig: alsConfigPbst},
		}},
		StatPrefix: statPrefix,
		CodecType:  hcm.HttpConnectionManager_AUTO,
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource: &core.ConfigSource{
					ResourceApiVersion: core.ApiVersion_V3,
					ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
						ApiConfigSource: &core.ApiConfigSource{
							ApiType:             core.ApiConfigSource_GRPC,
							TransportApiVersion: core.ApiVersion_V3,
							GrpcServices: []*core.GrpcService{{
								TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
									EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: "xds_cluster"},
								},
							}},
						},
					},
				},
				RouteConfigName: listenerRouteConfigName,
			},
		},
	}
	return manager, nil
}

func addTracing(httpConnManager *hcm.HttpConnectionManager, tracingProps *common.TracingProperties) error {
	if tracingProps.Enabled {
		zipkinConfig, err := ptypes.MarshalAny(&envoyConfigTraceV3.ZipkinConfig{
			CollectorCluster:         tracingProps.ZipkinCollectorCluster,
			CollectorEndpoint:        tracingProps.ZipkinCollectorEndpoint,
			CollectorEndpointVersion: envoyConfigTraceV3.ZipkinConfig_HTTP_JSON,
		})
		if err != nil {
			return err
		}
		httpConnManager.Tracing = &hcm.HttpConnectionManager_Tracing{
			OverallSampling: &etype.Percent{Value: tracingProps.TracingSamplerProbabilisticValue},
			Provider: &envoyConfigTraceV3.Tracing_Http{
				Name: "envoy.tracers.zipkin",
				ConfigType: &envoyConfigTraceV3.Tracing_Http_TypedConfig{
					TypedConfig: zipkinConfig,
				},
			},
		}
	}
	return nil
}

// addEnvoyRouterFilter adds "envoy.filters.http.router" filter to the HttpConnectionManager, must be the last filter in the chain.
func addEnvoyRouterFilter(httpConnManager *hcm.HttpConnectionManager) error {
	marshalledConfig, err := ptypes.MarshalAny(&routerV3.Router{})
	if err != nil {
		return err
	}
	filter := hcm.HttpFilter{
		Name:       wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledConfig},
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &filter)
	return nil
}

func doTerminalStep(httpConnManager *hcm.HttpConnectionManager) error {
	/*fixed Error: envoy.router must be the terminal http filter.*/
	return addEnvoyRouterFilter(httpConnManager)
}

func addWASMFilter(httpConnManager *hcm.HttpConnectionManager, filterToAdd *domain.WasmFilter) error {
	cluster, err := filterToAdd.Cluster()
	if err != nil {
		return err
	}
	paramsJson, err := json.Marshal(filterToAdd.Params)
	if err != nil {
		return err
	}
	value := wrapperspb.StringValue{Value: string(paramsJson)}
	configurationAsAny, err := ptypes.MarshalAny(&value)
	if err != nil {
		return err
	}
	wasmConfig := wasmV3.PluginConfig{
		Name:   filterToAdd.Name,
		RootId: filterToAdd.Name + "_root_id",
		Vm: &wasmV3.PluginConfig_VmConfig{
			VmConfig: &wasmV3.VmConfig{
				VmId:    filterToAdd.Name + "_vm_id",
				Runtime: "envoy.wasm.runtime.v8",
				Code: &core.AsyncDataSource{Specifier: &core.AsyncDataSource_Remote{Remote: &core.RemoteDataSource{
					HttpUri: &core.HttpUri{
						Uri:              filterToAdd.URL,
						HttpUpstreamType: &core.HttpUri_Cluster{Cluster: strings.ReplaceAll(cluster, ".", "_")},
						Timeout: &duration.Duration{
							Seconds: filterToAdd.Timeout,
						},
					},
					Sha256: filterToAdd.SHA256,
					RetryPolicy: &core.RetryPolicy{
						NumRetries: &wrappers.UInt32Value{Value: uint32(300)},
					},
				}}},
				AllowPrecompiled: true,
			},
		},
		Configuration: configurationAsAny,
		FailOpen:      true,
	}
	wasm := wasmFiltersV3.Wasm{Config: &wasmConfig}
	marshalledConfig, err := ptypes.MarshalAny(&wasm)
	if err != nil {
		return err
	}
	filter := hcm.HttpFilter{
		Name:       "envoy.filters.http.wasm",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledConfig},
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &filter)
	return nil
}

func addLuaFilter(httpConnManager *hcm.HttpConnectionManager, filterToAdd *domain.LuaFilter) error {
	luaConfig := &luaFiltersV3.Lua{
		InlineCode: filterToAdd.LuaScript,
	}
	marshalled, err := ptypes.MarshalAny(luaConfig)
	if err != nil {
		return err
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &hcm.HttpFilter{
		Name:       wellknown.Lua,
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalled},
	})
return nil
}

func addCorsFilter(httpConnManager *hcm.HttpConnectionManager) error {
	marshalledConfig, err := ptypes.MarshalAny(&corsV3.Cors{})
	if err != nil {
		return err
	}
	filter := hcm.HttpFilter{
		Name:       wellknown.CORS,
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledConfig},
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &filter)
	return nil
}

func addHeaderToMetadataFilter(httpConnManager *hcm.HttpConnectionManager) error {
	filterConfig := h2m.Config{
		RequestRules: []*h2m.Config_Rule{{
			Header: "x-version",
			OnHeaderPresent: &h2m.Config_KeyValuePair{
				MetadataNamespace: "envoy.lb",
				Key:               "version",
				Type:              h2m.Config_STRING,
			},
			Remove: false,
		}},
	}
	marshalledConfig, err := ptypes.MarshalAny(&filterConfig)
	if err != nil {
		return err
	}
	filter := hcm.HttpFilter{
		Name:       "envoy.filters.http.header_to_metadata",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledConfig},
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &filter)
	return nil
}

func addStatefulSessionFilter(httpConnManager *hcm.HttpConnectionManager) error {
	config, err := common.BuildCookieBasedSessionFilterForListener()
	if err != nil {
		return err
	}
	filter := hcm.HttpFilter{
		Name:       "envoy.filters.http.stateful_session",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: config},
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &filter)
	return nil
}

func addDisabledRateLimit(httpConnManager *hcm.HttpConnectionManager) error {
	localRateLimit := &local_ratelimitv3.LocalRateLimit{
		StatPrefix: httpConnManager.StatPrefix + "ratelimit",
	}
	marshalledFilter, err := ptypes.MarshalAny(localRateLimit)
	if err != nil {
		logger.Errorf("routeconfig: failed to marshal LocalRateLimit config to protobuf Any")
		return err
	}
	filter := hcm.HttpFilter{
		Name:       "envoy.filters.http.local_ratelimit",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledFilter},
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &filter)
	return nil
}

func newListener(httpConnManager *hcm.HttpConnectionManager, originalListener *domain.Listener, withTls bool) (*v3listener.Listener, error) {
	name := originalListener.Name
	port, err := strconv.Atoi(originalListener.BindPort)
	if err != nil {
		return nil, err
	}
	if withTls {
		name += "-tls"
		if port == util.DefaultPort {
			port = listenerTlsPort
		}
	}

	pbst, err := ptypes.MarshalAny(httpConnManager)
	if err != nil {
		return nil, err
	}
	listener := v3listener.Listener{
		Name:    name,
		Address: common.BuildSocketAddr(originalListener.BindHost, uint32(port)),
		FilterChains: []*v3listener.FilterChain{{
			Filters: []*v3listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &v3listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}
	if withTls {
		return enrichListenerWithTlsFilters(&listener)
	}
	return &listener, nil
}

func enrichListenerWithTlsFilters(listener *v3listener.Listener) (*v3listener.Listener, error) {
	tlsInspectorMarshalled, err := ptypes.MarshalAny(&tlsInspectorV3.TlsInspector{})
	if err != nil {
		return nil, err
	}
	listener.ListenerFilters = []*v3listener.ListenerFilter{{
		Name:       wellknown.TlsInspector,
		ConfigType: &v3listener.ListenerFilter_TypedConfig{TypedConfig: tlsInspectorMarshalled},
	}}

	tlsContext := &tlsV3.DownstreamTlsContext{
		CommonTlsContext:         buildServerCommonTlsContext(),
		RequireClientCertificate: &wrappers.BoolValue{Value: false},
	}
	tlsContextMarshalled, err := ptypes.MarshalAny(tlsContext)
	if err != nil {
		return nil, err
	}
	listener.FilterChains[0].TransportSocket = &core.TransportSocket{
		Name: wellknown.TransportSocketTls,
		ConfigType: &core.TransportSocket_TypedConfig{
			TypedConfig: tlsContextMarshalled,
		},
	}
	return listener, nil
}

func buildServerCommonTlsContext() *tlsV3.CommonTlsContext {
	return &tlsV3.CommonTlsContext{
		AlpnProtocols: []string{"h2", "http/1.1"},
		TlsCertificates: []*tlsV3.TlsCertificate{{
			CertificateChain: common.BuildFilenameDataSource(tlsmode.GatewayCertificatesFilePath() + "/tls.crt"),
			PrivateKey:       common.BuildFilenameDataSource(tlsmode.GatewayCertificatesFilePath() + "/tls.key"),
			Password:         nil,
		}},
		ValidationContextType: &tlsV3.CommonTlsContext_ValidationContext{
			ValidationContext: &tlsV3.CertificateValidationContext{
				TrustedCa:               common.BuildFilenameDataSource(tlsmode.GatewayCertificatesFilePath() + "/ca.crt"),
				MatchSubjectAltNames:    nil,
				AllowExpiredCertificate: false,
				TrustChainVerification:  tlsV3.CertificateValidationContext_VERIFY_TRUST_CHAIN,
				OnlyVerifyLeafCertCrl:   false,
			},
		},
	}
}

func addExtAuthzFilter(connManager *hcm.HttpConnectionManager, filterSpec *domain.ExtAuthzFilter) error {
	extAuthzFilter := &extauthz.ExtAuthz{
		ClearRouteCache:     true,
		StatusOnError:       &etype.HttpStatus{Code: etype.StatusCode_NetworkAuthenticationRequired},
		TransportApiVersion: core.ApiVersion_V3,
		Services: &extauthz.ExtAuthz_GrpcService{
			GrpcService: &core.GrpcService{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: filterSpec.ClusterName}},
				Timeout:         ptypes.DurationProto(time.Duration(filterSpec.Timeout) * time.Millisecond),
			},
		},
	}
	marshalledExtAuthz, err := ptypes.MarshalAny(extAuthzFilter)
	if err != nil {
		return err
	}
	connManager.HttpFilters = append(connManager.HttpFilters, &hcm.HttpFilter{
		Name:       wellknown.HTTPExternalAuthorization,
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledExtAuthz},
	})
	return nil
}

func addDisabledLuaFilter(httpConnManager *hcm.HttpConnectionManager) error {
	defaultLuaFilter := &luaFiltersV3.Lua{
		StatPrefix: httpConnManager.StatPrefix + "lua",
	}
	marshalledFilter, err := ptypes.MarshalAny(defaultLuaFilter)
	if err != nil {
		logger.Errorf("routeconfig: failed to marshal DefaultLuaFilter config to protobuf Any")
		return err
	}
	filter := hcm.HttpFilter{
		Name:       "envoy.filters.http.lua",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: marshalledFilter},
	}
	httpConnManager.HttpFilters = append(httpConnManager.HttpFilters, &filter)
	return nil
}