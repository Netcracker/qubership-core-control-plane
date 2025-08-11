package cluster

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/dns"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/provider"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	tlsUtil "github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/tls"
	"os"
	"strings"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	tlsV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	extHttp "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	v31 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	etype "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"google.golang.org/protobuf/types/known/durationpb"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("EnvoyConfigBuilder#common")
}

type ClusterBuilder interface {
	BuildCluster(nodeGroup string, domainCluster *domain.Cluster, routeProperties *common.RouteProperties) (*cluster.Cluster, error)
}

type DefaultClusterBuilder struct {
	BaseClusterBuilder
}

func NewDefaultClusterBuilder(dao dao.Repository, routeProperties *common.RouteProperties) *DefaultClusterBuilder {
	builder := &DefaultClusterBuilder{}
	baseBuilder := BaseClusterBuilder{
		dao:                      dao,
		routeProperties:          routeProperties,
		enrichUpstreamTlsContext: nil,
	}
	builder.BaseClusterBuilder = baseBuilder
	return builder
}

type BaseClusterBuilder struct {
	dao                      dao.Repository
	routeProperties          *common.RouteProperties
	enrichUpstreamTlsContext func(*tlsV3.UpstreamTlsContext)
}

func (builder *BaseClusterBuilder) loadClusterRelations(cluster *domain.Cluster) (*domain.Cluster, error) {
	clusterEndpoints, err := builder.dao.FindEndpointsByClusterId(cluster.Id)
	if err != nil {
		logger.Errorf("Failed to load cluster %v endpoints using DAO: %v", cluster.Name, err)
		return nil, err
	}
	cluster.Endpoints = clusterEndpoints
	for _, clusterEndpoint := range cluster.Endpoints {
		endpointVersion, err := builder.dao.FindDeploymentVersion(clusterEndpoint.DeploymentVersion)
		if err != nil {
			logger.Errorf("Failed to load endpoint %v deployment version using DAO: %v", clusterEndpoint.Id, err)
			return nil, err
		}
		clusterEndpoint.DeploymentVersionVal = endpointVersion
	}
	// load healthChecks
	healthChecks, err := builder.dao.FindHealthChecksByClusterId(cluster.Id)
	if err != nil {
		logger.Errorf("Failed to load cluster %v healthChecks using DAO: %v", cluster.Name, err)
		return nil, err
	}
	cluster.HealthChecks = healthChecks

	//load tcp keepalive
	if cluster.TcpKeepaliveId != 0 {
		tcpKeepalive, err := builder.dao.FindTcpKeepaliveById(cluster.TcpKeepaliveId)
		if err != nil {
			logger.Errorf("Failed to load cluster %v tp keepalives using DAO: %v", cluster.Name, err)
			return nil, err
		}
		cluster.TcpKeepalive = tcpKeepalive
	}

	//load TLS config
	tlsConfig, err := builder.dao.FindTlsConfigById(cluster.TLSId)
	if err != nil {
		logger.Errorf("Failed to load cluster %v tlsConfig using DAO: %v", cluster.Name, err)
		return nil, err
	}

	if tlsConfig != nil {
		logger.Debug("Loaded cluster-wide tls config for cluster %s: %+v", cluster.Name, *tlsConfig)
	} else {
		logger.Debug("There is no cluster-wide tls config for cluster %s", cluster.Name)
	}

	cluster.TLS = tlsConfig

	//load CircuitBreaker
	circuitBreaker, err := builder.dao.FindCircuitBreakerById(cluster.CircuitBreakerId)
	if err != nil {
		logger.Errorf("Failed to load cluster %v circuitBreaker using DAO: %v", cluster.Name, err)
		return nil, err
	}

	if circuitBreaker != nil {
		//load Threshold
		threshold, err := builder.dao.FindThresholdById(circuitBreaker.ThresholdId)
		if err != nil {
			logger.Errorf("Failed to load cluster %v threshold using DAO: %v", cluster.Name, err)
			return nil, err
		}

		circuitBreaker.Threshold = threshold
	}

	cluster.CircuitBreaker = circuitBreaker

	return cluster, nil
}

func (builder *BaseClusterBuilder) BuildCluster(nodeGroup string, domainCluster *domain.Cluster, routeProperties *common.RouteProperties) (*cluster.Cluster, error) {
	logger.Debug("Building envoy cluster %s", domainCluster.Name)
	activeVersions, err := builder.dao.FindDeploymentVersionsByStage("ACTIVE")
	if err != nil {
		logger.Errorf("Failed to load active version using DAO: %v", err)
		return nil, err
	}
	activeVersion := activeVersions[0].Version
	domainCluster, err = builder.loadClusterRelations(domainCluster)
	if err != nil {
		return nil, err
	}
	aggregatedTlsConfig, err := builder.aggregateGlobalTlsConfig(nodeGroup, domainCluster)
	if err != nil {
		return nil, err
	}
	if aggregatedTlsConfig != nil {
		domainCluster.TLS = aggregatedTlsConfig
	}
	if len(domainCluster.Endpoints) == 1 {
		activeVersion = domainCluster.Endpoints[0].DeploymentVersion
	}

	var maxConnections int32 = 1048576
	if domainCluster.CircuitBreaker != nil && domainCluster.CircuitBreaker.Threshold != nil && domainCluster.CircuitBreaker.Threshold.MaxConnections != 0 {
		maxConnections = domainCluster.CircuitBreaker.Threshold.MaxConnections
	}

	c := &cluster.Cluster{
		Name:                 ReplaceDotsByUnderscore(domainCluster.Name),
		DnsLookupFamily:      dns.DefaultLookupFamily.Get(),
		ConnectTimeout:       durationpb.New(routeProperties.GetTimeout()),
		LbPolicy:             cluster.Cluster_LbPolicy(cluster.Cluster_LbPolicy_value[domainCluster.LbPolicy]),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_DiscoveryType(cluster.Cluster_DiscoveryType_value[domainCluster.DiscoveryType])},
		LoadAssignment:       buildClusterLoadAssignment(domainCluster),
		CircuitBreakers: &cluster.CircuitBreakers{
			Thresholds: []*cluster.CircuitBreakers_Thresholds{
				{
					Priority:           core.RoutingPriority_DEFAULT,
					MaxConnections:     &wrappers.UInt32Value{Value: uint32(maxConnections)},
					MaxRequests:        &wrappers.UInt32Value{Value: 1048576},
					MaxPendingRequests: &wrappers.UInt32Value{Value: 1048576},
					MaxRetries:         &wrappers.UInt32Value{Value: 1048576},
				},
				{
					Priority:           core.RoutingPriority_HIGH,
					MaxConnections:     &wrappers.UInt32Value{Value: uint32(maxConnections)},
					MaxRequests:        &wrappers.UInt32Value{Value: 1048576},
					MaxPendingRequests: &wrappers.UInt32Value{Value: 1048576},
					MaxRetries:         &wrappers.UInt32Value{Value: 1048576},
				},
			},
		},
	}
	if c.Name != domain.ExtAuthClusterName {
		c.LbSubsetConfig = &cluster.Cluster_LbSubsetConfig{
			FallbackPolicy: cluster.Cluster_LbSubsetConfig_DEFAULT_SUBSET,
			DefaultSubset: &structpb.Struct{Fields: map[string]*structpb.Value{
				"version": {Kind: &structpb.Value_StringValue{StringValue: activeVersion}},
			}},
			SubsetSelectors: []*cluster.Cluster_LbSubsetConfig_LbSubsetSelector{{Keys: []string{"version"}}},
		}
	} else {
		c.OutlierDetection = &cluster.OutlierDetection{
			Consecutive_5Xx: &wrappers.UInt32Value{Value: 1},
		}
	}
	var httpProtocolOptions *any.Any
	if *domainCluster.HttpVersion == 2 {
		httpProtocolOptions, err = ptypes.MarshalAny(
			&extHttp.HttpProtocolOptions{
				UpstreamProtocolOptions: &extHttp.HttpProtocolOptions_ExplicitHttpConfig_{
					ExplicitHttpConfig: &extHttp.HttpProtocolOptions_ExplicitHttpConfig{
						ProtocolConfig: &extHttp.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
							Http2ProtocolOptions: &core.Http2ProtocolOptions{},
						},
					},
				},
			},
		)
		if err != nil {
			return nil, err
		}
	} else {
		httpProtocolOptions, err = ptypes.MarshalAny(
			&extHttp.HttpProtocolOptions{
				UpstreamProtocolOptions: &extHttp.HttpProtocolOptions_ExplicitHttpConfig_{
					ExplicitHttpConfig: &extHttp.HttpProtocolOptions_ExplicitHttpConfig{
						ProtocolConfig: &extHttp.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{
							HttpProtocolOptions: &core.Http1ProtocolOptions{
								AllowChunkedLength: true,
							},
						},
					},
				},
			},
		)
		if err != nil {
			return nil, err
		}
	}
	c.TypedExtensionProtocolOptions = map[string]*any.Any{
		"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": httpProtocolOptions,
	}
	if domainCluster.HealthChecks != nil {
		c.HealthChecks = buildHealthChecks(domainCluster.HealthChecks)
	}
	if domainCluster.TcpKeepalive != nil {
		c.UpstreamConnectionOptions = &cluster.UpstreamConnectionOptions{
			TcpKeepalive: &core.TcpKeepalive{
				KeepaliveProbes:   &wrappers.UInt32Value{Value: uint32(domainCluster.TcpKeepalive.Probes)},
				KeepaliveTime:     &wrappers.UInt32Value{Value: uint32(domainCluster.TcpKeepalive.Time)},
				KeepaliveInterval: &wrappers.UInt32Value{Value: uint32(domainCluster.TcpKeepalive.Interval)},
			},
		}
	}
	if domainCluster.CommonLbConfig != nil {
		c.CommonLbConfig = buildCommonLbConfig(domainCluster.CommonLbConfig)
	}
	if domainCluster.DnsResolvers != nil {
		c.DnsResolvers = buildDnsResolvers(domainCluster.DnsResolvers)
	}
	if aggregatedTlsConfig != nil && aggregatedTlsConfig.Enabled {
		logger.Debugf("Building envoy cluster %s tls configuration %s", domainCluster.Name, aggregatedTlsConfig.Name)

		upstreamTlsContext := buildUpstreamTlsContext(domainCluster)
		if builder.enrichUpstreamTlsContext != nil {
			builder.enrichUpstreamTlsContext(upstreamTlsContext)
		}
		if err = buildTransportSocket(c, upstreamTlsContext); err != nil {
			return c, err
		}
	} else {
		logger.Debugf("Envoy cluster %s tls configuration is nil or disabled", domainCluster.Name)
	}
	if domainCluster.MaxRequestsPerConnection != 0 {
		c.MaxRequestsPerConnection = &wrappers.UInt32Value{Value: uint32(domainCluster.MaxRequestsPerConnection)}
	}
	return c, nil
}

// ReplaceDotsByUnderscore
// This method replaces dots with underscores in an accepted argument.
// This needs for cluster name because of Envoy has an issue
// https://github.com/envoyproxy/envoy/issues/5239
func ReplaceDotsByUnderscore(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func isDefaultTlsConfig(cluster *domain.Cluster) bool {
	currentClusterName := cluster.Name
	clusterNameIndex := strings.Index(currentClusterName, "||")
	if clusterNameIndex > 0 {
		currentClusterName = currentClusterName[:clusterNameIndex]
	}
	return cluster.TLS.Name == currentClusterName+"-tls"
}

func buildMatchTypedSubjectAltNames(cluster *domain.Cluster) []*tlsV3.SubjectAltNameMatcher {
	if tlsmode.GetMode() == tlsmode.Disabled || !isDefaultTlsConfig(cluster) {
		return nil
	}

	var dnsNames []string
	for _, domainEndpoint := range cluster.Endpoints {
		endpointAddr := domainEndpoint.Address
		dnsNames = append(dnsNames, endpointAddr)
	}

	var matchTypedSubjectAltNames []*tlsV3.SubjectAltNameMatcher
	for _, name := range dnsNames {
		matchTypedSubjectAltNames = append(matchTypedSubjectAltNames, &tlsV3.SubjectAltNameMatcher{
			SanType: tlsV3.SubjectAltNameMatcher_DNS,
			Matcher: &v31.StringMatcher{
				MatchPattern: &v31.StringMatcher_Exact{
					Exact: name,
				},
			},
		})
	}
	return matchTypedSubjectAltNames
}

func buildTransportSocket(cluster *cluster.Cluster, upstreamTlsContext *tlsV3.UpstreamTlsContext) error {
	transportSocket, err := ptypes.MarshalAny(upstreamTlsContext)
	if err != nil {
		logger.Errorf("Error during marshalling UpstreamTlsContext: \n%v", err)
		return err
	}
	cluster.TransportSocket = &core.TransportSocket{
		Name: wellknown.TransportSocketTls,
		ConfigType: &core.TransportSocket_TypedConfig{
			TypedConfig: transportSocket,
		},
	}
	return nil
}

func buildUpstreamTlsContext(cluster *domain.Cluster) *tlsV3.UpstreamTlsContext {
	commonTlsContext := &tlsV3.CommonTlsContext{
		ValidationContextType: &tlsV3.CommonTlsContext_ValidationContext{
			ValidationContext: &tlsV3.CertificateValidationContext{
				TrustedCa:                 buildTrustedCA(cluster.Name, cluster.TLS.TrustedCA, cluster.TLS.Insecure),
				TrustChainVerification:    buildTrustChainVerification(cluster.TLS.Insecure),
				MatchTypedSubjectAltNames: buildMatchTypedSubjectAltNames(cluster),
			},
		},
		TlsCertificates: []*tlsV3.TlsCertificate{},
	}

	if cluster.TLS.ClientCert != "" {
		commonTlsContext.TlsCertificates = append(commonTlsContext.TlsCertificates, &tlsV3.TlsCertificate{
			CertificateChain: common.BuildInlineStringDataSource(cluster.TLS.ClientCert),
			PrivateKey:       common.BuildInlineStringDataSource(cluster.TLS.PrivateKey),
			Password:         nil,
		})
	}

	if tlsmode.GetMode() == tlsmode.Preferred {
		commonTlsContext.TlsCertificates = append(commonTlsContext.TlsCertificates, &tlsV3.TlsCertificate{
			CertificateChain: common.BuildFilenameDataSource(tlsmode.GatewayCertificatesFilePath() + "/tls.crt"),
			PrivateKey:       common.BuildFilenameDataSource(tlsmode.GatewayCertificatesFilePath() + "/tls.key"),
			Password:         nil,
		})
	}

	return &tlsV3.UpstreamTlsContext{Sni: cluster.TLS.SNI, CommonTlsContext: commonTlsContext}
}

func (builder *BaseClusterBuilder) aggregateGlobalTlsConfig(nodeGroup string, cluster *domain.Cluster) (*domain.TlsConfig, error) {
	propagateSniEnv, exists := os.LookupEnv("SNI_PROPAGATION_ENABLED")
	propagateSni := exists && strings.EqualFold(strings.TrimSpace(propagateSniEnv), "true")

	var clusterEndpoint *domain.Endpoint
	if len(cluster.Endpoints) != 0 {
		clusterEndpoint = cluster.Endpoints[0]
	}

	if cluster.TLS != nil {
		if validated, err := tlsUtil.TryToDecodePemAndParseX509Certificates(cluster.TLS.TrustedCA); err != nil {
			logger.Warnf("Trusted CA partially or fully excluded from configuration due to validation error for certificates: %v", err)
			cluster.TLS.TrustedCA = validated
		}

		if validated, err := tlsUtil.TryToValidateX509PrivateKeyClientCertPair(cluster.TLS.PrivateKey, cluster.TLS.ClientCert); err != nil {
			if validated == "" {
				logger.Warnf("Private key and Client certificate excluded from configuration due to validation error for key pair:\n %v", err)
				cluster.TLS.PrivateKey = ""
				cluster.TLS.ClientCert = ""
			} else {
				logger.Warnf("Client certificate partially excluded from configuration due to validation error for key pair:\n %v ", err)
				cluster.TLS.ClientCert = validated
			}

		}
	}

	if clusterEndpoint == nil || (clusterEndpoint.Port != 443 && clusterEndpoint.Protocol != "https") {
		logger.Debug("There is no TLS endpoint for cluster %s", cluster.Name)
		return cluster.TLS, nil
	}
	enabledTlsConfigs := make([]*domain.TlsConfig, 0)
	tlsConfigs, err := provider.GetTlsService().GetGlobalTlsConfigs(cluster, nodeGroup)
	if err != nil {
		return nil, err
	}
	for _, tlsConfig := range tlsConfigs {
		logger.Debug("Processing global TLS config for cluster %s in %s: %v", cluster.Name, nodeGroup, tlsConfig.Name)
		if tlsConfig.Enabled {
			enabledTlsConfigs = append(enabledTlsConfigs, tlsConfig)
			logger.Debug("Added global TLS config for cluster %s: %v", cluster.Name, tlsConfig.Name)
		}
	}
	if len(enabledTlsConfigs) == 0 {
		return cluster.TLS, nil
	}

	aggregatedTlsConfig := domain.TlsConfig{
		NodeGroups: make([]*domain.NodeGroup, 0),
		Name:       "aggregatedTlsConfig-" + cluster.Name,
		Enabled:    true,
		Insecure:   false,
	}
	for _, tlsConfig := range enabledTlsConfigs {
		aggregatedTlsConfig.SNI = tlsConfig.SNI

		if tlsConfig.Insecure { // if at least one gateway-level tls config is insecure, that consider the whole gateway-level configuration insecure
			logger.Debugf("Gateway-level configuration for cluster is insecure")
			aggregatedTlsConfig.Insecure = true
			aggregatedTlsConfig.TrustedCA = ""
			aggregatedTlsConfig.ClientCert = ""
			aggregatedTlsConfig.PrivateKey = ""
			break
		}
		validated, tErr := tlsUtil.TryToDecodePemAndParseX509Certificates(tlsConfig.TrustedCA)
		if tErr != nil {
			logger.Warnf("Trusted CA can't be added to aggregated config: %v", tErr)
		}
		if validated != "" {
			aggregatedTlsConfig.TrustedCA = strings.Join([]string{aggregatedTlsConfig.TrustedCA, validated}, "\n")
		}

		validated, cErr := tlsUtil.TryToValidateX509PrivateKeyClientCertPair(tlsConfig.PrivateKey, tlsConfig.ClientCert)
		if cErr != nil {
			logger.Warnf("Client certificate or Private key can't be added to aggregated config: %v", cErr)
		}
		if tlsConfig.PrivateKey != "" && validated != "" {
			aggregatedTlsConfig.ClientCert = strings.Join([]string{aggregatedTlsConfig.ClientCert, validated}, "\n")
			aggregatedTlsConfig.PrivateKey = strings.Join([]string{aggregatedTlsConfig.PrivateKey, tlsConfig.PrivateKey}, "\n")
		}

	}
	if propagateSni && aggregatedTlsConfig.SNI == "" {
		aggregatedTlsConfig.SNI = cluster.Endpoints[0].Address
		logger.Debugf("SNI with value [%s] will be propagated for cluster [%s]", cluster.Endpoints[0].Address, cluster.Name)
	}
	if cluster.TLS != nil { // cluster TLS config has priority over global TLS, unless it is default TLS config for mTLS
		if cluster.TLS.SNI != "" {
			aggregatedTlsConfig.SNI = cluster.TLS.SNI
			logger.Debugf("SNI with value [%s] will finally be used for cluster [%s]", cluster.TLS.SNI, cluster.Name)
		}
		clusterFamilyName := clusterkey.DefaultClusterKeyGenerator.ExtractFamilyName(cluster.Name)
		if cluster.TLS.Name == clusterFamilyName+"-tls" { // this is default mTLS configuration, it has lower priority then gateway-level config
			mergeTlsWithGatewayPriority(&aggregatedTlsConfig, cluster.TLS, cluster.Name)
		} else {
			mergeTlsWithClusterPriority(&aggregatedTlsConfig, cluster.TLS, cluster.Name)
		}
	}
	return &aggregatedTlsConfig, nil
}

func mergeTlsWithClusterPriority(aggregatedTlsConfig, clusterTlsConfig *domain.TlsConfig, clusterName string) {
	logger.Info("Cluster-wide TLS config will override gateway-wide configuration for the cluster %s", clusterName)
	aggregatedTlsConfig.Enabled = clusterTlsConfig.Enabled
	aggregatedTlsConfig.Insecure = clusterTlsConfig.Insecure
	if aggregatedTlsConfig.Enabled && !aggregatedTlsConfig.Insecure {
		if clusterTlsConfig.TrustedCA != "" {
			aggregatedTlsConfig.TrustedCA = strings.Join([]string{aggregatedTlsConfig.TrustedCA, clusterTlsConfig.TrustedCA}, "\n")
		}
		if clusterTlsConfig.ClientCert != "" {
			aggregatedTlsConfig.ClientCert = strings.Join([]string{aggregatedTlsConfig.ClientCert, clusterTlsConfig.ClientCert}, "\n")
			aggregatedTlsConfig.PrivateKey = strings.Join([]string{aggregatedTlsConfig.PrivateKey, clusterTlsConfig.PrivateKey}, "\n")
		}
	}
}

func mergeTlsWithGatewayPriority(aggregatedTlsConfig, clusterTlsConfig *domain.TlsConfig, clusterName string) {
	logger.Info("Gateway-wide TLS config will override cluster-wide configuration for the cluster %s", clusterName)
	if !aggregatedTlsConfig.Insecure {
		if clusterTlsConfig.TrustedCA != "" {
			aggregatedTlsConfig.TrustedCA = strings.Join([]string{aggregatedTlsConfig.TrustedCA, clusterTlsConfig.TrustedCA}, "\n")
		}
		if clusterTlsConfig.ClientCert != "" {
			aggregatedTlsConfig.ClientCert = strings.Join([]string{aggregatedTlsConfig.ClientCert, clusterTlsConfig.ClientCert}, "\n")
			aggregatedTlsConfig.PrivateKey = strings.Join([]string{aggregatedTlsConfig.PrivateKey, clusterTlsConfig.PrivateKey}, "\n")
		}
	}
}

func buildTrustChainVerification(insecure bool) tlsV3.CertificateValidationContext_TrustChainVerification {
	if insecure {
		return tlsV3.CertificateValidationContext_ACCEPT_UNTRUSTED
	} else {
		return tlsV3.CertificateValidationContext_VERIFY_TRUST_CHAIN
	}
}

func buildTrustedCA(clusterName, customCA string, insecure bool) *core.DataSource {
	customCA = strings.TrimSpace(customCA)
	if len(customCA) == 0 {
		if tlsmode.GetMode() == tlsmode.Disabled {
			logger.Debugf("Core TLS is disabled, using /etc/ssl/certs/ca-certificates.crt for cluster %s", clusterName)
			return &core.DataSource{Specifier: &core.DataSource_Filename{Filename: "/etc/ssl/certs/ca-certificates.crt"}}
		} else {
			logger.Debugf("Core TLS is enabled, using ca.crt for cluster %s", clusterName)
			return &core.DataSource{Specifier: &core.DataSource_Filename{Filename: tlsmode.GatewayCertificatesFilePath() + "/ca.crt"}}
		}
	} else {
		logger.Debugf("Adding custom CA for cluster %s", clusterName)
		return &core.DataSource{Specifier: &core.DataSource_InlineString{InlineString: customCA}}
	}
}

func buildClusterLoadAssignment(cluster *domain.Cluster) *endpoint.ClusterLoadAssignment {
	return &endpoint.ClusterLoadAssignment{
		ClusterName: ReplaceDotsByUnderscore(cluster.Name),
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: createLbEndpoints(cluster.Name, cluster.Endpoints),
		}},
	}
}

func createLbEndpoints(clusterName string, endpoints []*domain.Endpoint) []*endpoint.LbEndpoint {
	result := make([]*endpoint.LbEndpoint, 0)
	for _, clusterEndpoint := range endpoints {
		if clusterEndpoint.DeploymentVersionVal.Stage == "ARCHIVED" {
			continue
		}
		endpointAddr := clusterEndpoint.Address

		if !strings.Contains(endpointAddr, msaddr.LocalDevNamespacePostfix) {
			namespace := msaddr.CurrentNamespaceAsString()
			endpointAddrParts := strings.Split(endpointAddr, ".")
			//endpoint address doesn't have namespace
			if len(endpointAddrParts) == 1 && (namespace != msaddr.LocalNamespace) {
				endpointAddr = endpointAddr + "." + namespace + ".svc.cluster.local"
			}
			logger.Infof("Endpoint address in cluster: %s", endpointAddr)
		}

		lbEndpoint := &endpoint.LbEndpoint{
			HostIdentifier: &endpoint.LbEndpoint_Endpoint{
				Endpoint: &endpoint.Endpoint{
					Address: common.BuildSocketAddr(endpointAddr, uint32(clusterEndpoint.Port)),
				},
			},
		}
		if clusterName != domain.ExtAuthClusterName {
			lbEndpoint.Metadata = &core.Metadata{
				FilterMetadata: map[string]*structpb.Struct{
					"envoy.lb": {
						Fields: map[string]*structpb.Value{
							"version": {Kind: &structpb.Value_StringValue{StringValue: clusterEndpoint.DeploymentVersion}},
						},
					},
				},
			}
		}
		if clusterEndpoint.Hostname != "" {
			endpointIdentifier := lbEndpoint.HostIdentifier.(*endpoint.LbEndpoint_Endpoint)
			endpointIdentifier.Endpoint.Hostname = clusterEndpoint.Hostname
			endpointIdentifier.Endpoint.HealthCheckConfig = &endpoint.Endpoint_HealthCheckConfig{Hostname: clusterEndpoint.Hostname}
		}
		result = append(result, lbEndpoint)
	}
	return result
}

func buildHealthChecks(healthChecks []*domain.HealthCheck) []*core.HealthCheck {
	if healthChecks == nil {
		return nil
	}
	result := make([]*core.HealthCheck, len(healthChecks))
	for i, healthCheckConfig := range healthChecks {
		healthCheck := &core.HealthCheck{
			Timeout:                      util.MillisToDuration(healthCheckConfig.Timeout),
			Interval:                     util.MillisToDuration(healthCheckConfig.Interval),
			InitialJitter:                util.MillisToDuration(healthCheckConfig.InitialJitter),
			IntervalJitter:               util.MillisToDuration(healthCheckConfig.IntervalJitter),
			IntervalJitterPercent:        healthCheckConfig.IntervalJitterPercent,
			UnhealthyThreshold:           &wrappers.UInt32Value{Value: healthCheckConfig.UnhealthyThreshold},
			HealthyThreshold:             &wrappers.UInt32Value{Value: healthCheckConfig.HealthyThreshold},
			ReuseConnection:              &wrappers.BoolValue{Value: healthCheckConfig.ReuseConnection},
			NoTrafficInterval:            util.MillisToDuration(healthCheckConfig.NoTrafficInterval),
			UnhealthyInterval:            util.MillisToDuration(healthCheckConfig.UnhealthyInterval),
			UnhealthyEdgeInterval:        util.MillisToDuration(healthCheckConfig.UnhealthyEdgeInterval),
			HealthyEdgeInterval:          util.MillisToDuration(healthCheckConfig.HealthyEdgeInterval),
			EventLogPath:                 healthCheckConfig.EventLogPath,
			AlwaysLogHealthCheckFailures: healthCheckConfig.AlwaysLogHealthCheckFailures,
		}
		if healthCheckConfig.HttpHealthCheck != nil {
			httpHealthCheckConfig := healthCheckConfig.HttpHealthCheck
			healthCheck.HealthChecker = &core.HealthCheck_HttpHealthCheck_{
				HttpHealthCheck: &core.HealthCheck_HttpHealthCheck{
					Host:                   httpHealthCheckConfig.Host,
					Path:                   httpHealthCheckConfig.Path,
					RequestHeadersToAdd:    buildHeaderOptions(httpHealthCheckConfig.RequestHeadersToAdd),
					RequestHeadersToRemove: httpHealthCheckConfig.RequestHeadersToRemove,
					ExpectedStatuses:       buildRanges(httpHealthCheckConfig.ExpectedStatuses),
					CodecClientType:        etype.CodecClientType(etype.CodecClientType_value[httpHealthCheckConfig.CodecClientType]),
				},
			}
		}
		if healthCheckConfig.TlsOptions != nil {
			tlsOptionsConfig := healthCheckConfig.TlsOptions
			healthCheck.TlsOptions = &core.HealthCheck_TlsOptions{
				AlpnProtocols: tlsOptionsConfig.AlpnProtocols,
			}
		}
		result[i] = healthCheck
	}
	return result
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

func buildRanges(rangeMatches []domain.RangeMatch) []*etype.Int64Range {
	if rangeMatches == nil {
		return nil
	}
	result := make([]*etype.Int64Range, len(rangeMatches))
	for index, rangeMatch := range rangeMatches {
		item := &etype.Int64Range{}
		if rangeMatch.Start.Valid {
			item.Start = rangeMatch.Start.Int64
		}
		if rangeMatch.End.Valid {
			item.End = rangeMatch.End.Int64
		}
		result[index] = item
	}
	return result
}

func buildCommonLbConfig(commonLbConfig *domain.CommonLbConfig) *cluster.Cluster_CommonLbConfig {
	result := &cluster.Cluster_CommonLbConfig{
		HealthyPanicThreshold:           &etype.Percent{Value: commonLbConfig.HealthyPanicThreshold},
		LocalityConfigSpecifier:         nil,
		UpdateMergeWindow:               nil,
		IgnoreNewHostsUntilFirstHc:      commonLbConfig.IgnoreNewHostsUntilFirstHc,
		CloseConnectionsOnHostSetChange: commonLbConfig.CloseConnectionsOnHostSetChange,
	}
	if commonLbConfig.ConsistentHashingLbConfig != nil {
		consistentHashingLbConfig := commonLbConfig.ConsistentHashingLbConfig
		result.ConsistentHashingLbConfig = &cluster.Cluster_CommonLbConfig_ConsistentHashingLbConfig{
			UseHostnameForHashing: consistentHashingLbConfig.UseHostnameForHashing,
		}
	}
	return result
}

func buildDnsResolvers(dnsResolvers []domain.DnsResolver) []*core.Address {
	var result []*core.Address
	for _, dnsResolver := range dnsResolvers {
		// if socket_address specified
		if dnsResolver.SocketAddress != nil {
			var protocol core.SocketAddress_Protocol
			if dnsResolver.SocketAddress.Protocol == "UDP" {
				protocol = core.SocketAddress_UDP
			} else {
				protocol = core.SocketAddress_TCP
			}
			socketAddress := &core.Address_SocketAddress{SocketAddress: &core.SocketAddress{
				Address: dnsResolver.SocketAddress.Address,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: dnsResolver.SocketAddress.Port,
				},
				Protocol:   protocol,
				Ipv4Compat: dnsResolver.SocketAddress.IPv4_compat,
			}}
			result = append(result, &core.Address{Address: socketAddress})
		} else {
			// nor Pipe or EnvoyInternalAddress are supported yet
		}
	}
	return result
}
