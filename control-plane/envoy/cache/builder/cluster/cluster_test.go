package cluster

import (
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/provider"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	mock_provider "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/services/provider"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"os"
	"strings"
	"testing"

	clusterV3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	tlsV3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func TestBuildMatchTypedSubjectAltNames(t *testing.T) {
	const CLUSTER_NAME = "testClusterName"
	const TRUSTED_CA = firstCaCert
	const CLIENT_CERT = firstClientCert
	const PRIVATE_KEY = firstPrivateKey
	const TLS_ENABLED = true
	const TLS_ID = 2356

	defaultTlsConfig := &domain.TlsConfig{
		BaseModel:  bun.BaseModel{},
		Id:         TLS_ID,
		NodeGroups: nil,
		Name:       CLUSTER_NAME + "-tls",
		Enabled:    TLS_ENABLED,
		TrustedCA:  TRUSTED_CA,
		ClientCert: CLIENT_CERT,
		PrivateKey: PRIVATE_KEY,
		SNI:        "",
	}
	tests := []struct {
		name string
		//endpoints     []*domain.Endpoint
		cluster       *domain.Cluster
		enableTls     string
		expectedNames []string
	}{
		{
			name: "withTls",
			cluster: &domain.Cluster{
				Name: CLUSTER_NAME,
				TLS:  defaultTlsConfig,
				Endpoints: []*domain.Endpoint{
					{
						Address: "control-plane",
					},
					{
						Address: "test-service",
					},
				},
			},
			enableTls:     "true",
			expectedNames: []string{"control-plane", "test-service"},
		},
		{
			name: "withoutTls",
			cluster: &domain.Cluster{
				Name: CLUSTER_NAME,
				TLS:  defaultTlsConfig,
				Endpoints: []*domain.Endpoint{
					{
						Address: "control-plane",
					},
					{
						Address: "test-service",
					},
				},
			},
			enableTls:     "false",
			expectedNames: []string{},
		},
		{
			name: "withTlsAnd0Endpoints",
			cluster: &domain.Cluster{
				Name:      CLUSTER_NAME,
				TLS:       defaultTlsConfig,
				Endpoints: []*domain.Endpoint{},
			},
			enableTls:     "true",
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("INTERNAL_TLS_ENABLED", tt.enableTls)
			defer disableTls()
			configloader.Init(configloader.EnvPropertySource())
			tlsmode.SetUpTlsProperties()

			subjectAltNameMatcher := buildMatchTypedSubjectAltNames(tt.cluster)
			assert.Equal(t, len(tt.expectedNames), len(subjectAltNameMatcher))
			for _, matcher := range subjectAltNameMatcher {
				assert.Equal(t, tlsV3.SubjectAltNameMatcher_DNS, matcher.SanType)
				assert.True(t, util.SliceContainsElement(tt.expectedNames, matcher.Matcher.GetExact()))
			}
		})
	}
}

func disableTls() {
	os.Unsetenv("INTERNAL_TLS_ENABLED")
	configloader.Init(configloader.EnvPropertySource())
	tlsmode.SetUpTlsProperties()
}

func TestClusterBuilder_aggregateGlobalTlsConfig_all_disabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	tlsService := mock_provider.NewMockTlsService(ctrl)
	tlsService.EXPECT().GetGlobalTlsConfigs(gomock.Any(), "nodeGroup").Return([]*domain.TlsConfig{
		{
			Name:    "test1",
			Enabled: false,
		},
		{
			Name:    "test2",
			Enabled: false,
		},
	}, nil)
	provider.Init(tlsService)

	builder := NewDefaultClusterBuilder(nil, nil)
	config, err := builder.aggregateGlobalTlsConfig("nodeGroup", buildHttpsCluster("test"))
	assert.Nil(t, err)
	assert.Nil(t, config)
}

func TestClusterBuilder_aggregateGlobalTlsConfig_merge(t *testing.T) {
	_ = os.Setenv("SNI_PROPAGATION_ENABLED", "true")
	defer os.Unsetenv("SNI_PROPAGATION_ENABLED")
	ctrl := gomock.NewController(t)
	tlsService := mock_provider.NewMockTlsService(ctrl)
	tlsService.EXPECT().GetGlobalTlsConfigs(gomock.Any(), "nodeGroup").Return([]*domain.TlsConfig{
		{
			Name:       "test1",
			Enabled:    true,
			TrustedCA:  firstCaCert,
			ClientCert: firstClientCert,
			PrivateKey: firstPrivateKey,
		},
		{
			Name:       "test2",
			Enabled:    true,
			TrustedCA:  secondCaCert,
			ClientCert: secondClientCert,
			PrivateKey: secondPrivateKey,
		},
	}, nil)
	provider.Init(tlsService)

	builder := NewDefaultClusterBuilder(nil, nil)
	config, err := builder.aggregateGlobalTlsConfig("nodeGroup", buildHttpsCluster("test"))
	assert.Nil(t, err)
	assert.NotNil(t, config)
	assert.Contains(t, config.TrustedCA, firstCaCert)
	assert.Contains(t, config.ClientCert, firstClientCert)
	assert.Contains(t, config.PrivateKey, firstPrivateKey)
	assert.Contains(t, config.TrustedCA, secondCaCert)
	assert.Contains(t, config.ClientCert, secondClientCert)
	assert.Contains(t, config.PrivateKey, secondPrivateKey)
	assert.Equal(t, "test.net", config.SNI)
}

func TestClusterBuilder_aggregateGlobalTlsConfig_propagate_sni_false(t *testing.T) {
	_ = os.Setenv("SNI_PROPAGATION_ENABLED", "false")
	defer os.Unsetenv("SNI_PROPAGATION_ENABLED")
	ctrl := gomock.NewController(t)
	tlsService := mock_provider.NewMockTlsService(ctrl)
	tlsService.EXPECT().GetGlobalTlsConfigs(gomock.Any(), "nodeGroup").Return([]*domain.TlsConfig{
		{
			Name:       "test1",
			Enabled:    true,
			TrustedCA:  firstCaCert,
			ClientCert: firstClientCert,
			PrivateKey: firstPrivateKey,
		},
	}, nil)
	provider.Init(tlsService)

	builder := NewDefaultClusterBuilder(nil, nil)
	cluster := buildHttpsCluster("test")
	cluster.TLS = &domain.TlsConfig{
		Name:     "cluster-tls",
		Enabled:  true,
		Insecure: false,
	}
	config, err := builder.aggregateGlobalTlsConfig("nodeGroup", cluster)
	assert.Nil(t, err)
	assert.NotEqual(t, cluster.Endpoints[0].Address, config.SNI)
}
func TestClusterBuilder_aggregateGlobalTlsConfig_cluster_has_priority(t *testing.T) {
	_ = os.Setenv("SNI_PROPAGATION_ENABLED", "true")
	defer os.Unsetenv("SNI_PROPAGATION_ENABLED")
	ctrl := gomock.NewController(t)
	tlsService := mock_provider.NewMockTlsService(ctrl)
	tlsService.EXPECT().GetGlobalTlsConfigs(gomock.Any(), "test1").Return([]*domain.TlsConfig{
		{
			Name:       "test1",
			Enabled:    true,
			Insecure:   false,
			TrustedCA:  firstCaCert,
			ClientCert: firstClientCert,
			PrivateKey: firstPrivateKey,
		},
	}, nil)
	provider.Init(tlsService)

	builder := NewDefaultClusterBuilder(nil, nil)
	cluster := buildHttpsCluster("test")
	cluster.TLS = &domain.TlsConfig{
		Name:     "cluster-tls",
		Enabled:  true,
		Insecure: true,
	}
	config, err := builder.aggregateGlobalTlsConfig("test1", cluster)
	assert.Nil(t, err)
	assert.NotNil(t, config)
	assert.Contains(t, config.TrustedCA, firstCaCert)
	assert.Contains(t, config.ClientCert, firstClientCert)
	assert.Contains(t, config.PrivateKey, firstPrivateKey)
	assert.True(t, config.Insecure)
	assert.Equal(t, "test.net", config.SNI)
}

func TestClusterBuilder_mergeTlsWithClusterOrGatewayPriority(t *testing.T) {

	cluster := buildHttpsCluster("test-tls")
	cluster.TLS = &domain.TlsConfig{
		Name:       "cluster-tls",
		Enabled:    true,
		Insecure:   true,
		TrustedCA:  firstCaCert,
		ClientCert: firstClientCert,
		PrivateKey: firstPrivateKey,
		SNI:        "not_default",
	}
	aggregatedTlsConfig := domain.TlsConfig{
		NodeGroups: make([]*domain.NodeGroup, 0),
		Name:       "aggregatedTlsConfig-" + cluster.Name,
		Enabled:    false,
		Insecure:   false,
	}
	mergeTlsWithClusterPriority(&aggregatedTlsConfig, cluster.TLS, cluster.Name)
	assert.True(t, aggregatedTlsConfig.Insecure)
	assert.NotContains(t, aggregatedTlsConfig.TrustedCA, firstCaCert)
	assert.NotContains(t, aggregatedTlsConfig.ClientCert, firstClientCert)
	assert.NotContains(t, aggregatedTlsConfig.PrivateKey, firstPrivateKey)

	aggregatedTlsConfig = domain.TlsConfig{
		NodeGroups: make([]*domain.NodeGroup, 0),
		Name:       "aggregatedTlsConfig-" + cluster.Name,
		Enabled:    false,
		Insecure:   false,
	}
	mergeTlsWithGatewayPriority(&aggregatedTlsConfig, cluster.TLS, cluster.Name)
	assert.False(t, aggregatedTlsConfig.Insecure)
	assert.Contains(t, aggregatedTlsConfig.TrustedCA, firstCaCert)
	assert.Contains(t, aggregatedTlsConfig.ClientCert, firstClientCert)
	assert.Contains(t, aggregatedTlsConfig.PrivateKey, firstPrivateKey)
}

func TestClusterBuilder_aggregateGlobalTlsConfig_for_https_only(t *testing.T) {
	ctrl := gomock.NewController(t)
	tlsService := mock_provider.NewMockTlsService(ctrl)
	tlsService.EXPECT().GetGlobalTlsConfigs(gomock.Any(), "nodeGroup").AnyTimes().Return([]*domain.TlsConfig{
		{
			Name:       "test1",
			Enabled:    true,
			Insecure:   false,
			TrustedCA:  firstCaCert,
			ClientCert: firstClientCert,
			PrivateKey: firstPrivateKey,
		},
	}, nil)
	provider.Init(tlsService)
	tests := []struct {
		port    int32
		proto   string
		wantRes bool
	}{
		{port: 80, proto: "http", wantRes: false},
		{port: 443, proto: "http", wantRes: true},
		{port: 443, proto: "https", wantRes: true},
		{port: 80, proto: "https", wantRes: true},
	}
	builder := NewDefaultClusterBuilder(nil, nil)
	cluster := domain.NewCluster("test", false)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%d:%s", test.port, test.proto), func(t *testing.T) {
			cluster.Endpoints = []*domain.Endpoint{
				{
					Port:     test.port,
					Protocol: test.proto,
				},
			}
			config, _ := builder.aggregateGlobalTlsConfig("nodeGroup", cluster)
			assert.Equalf(t, test.wantRes, config != nil, "TestClusterBuilder_aggregateGlobalTlsConfig_for_https_only(%d:%s)", test.port, test.proto)
		})
	}
}

func buildHttpsCluster(name string) *domain.Cluster {
	cluster := domain.NewCluster(name, false)
	cluster.Endpoints = []*domain.Endpoint{
		{
			Address:  "test.net",
			Port:     443,
			Protocol: "https",
		},
	}
	return cluster
}

func getEgressClusterBuilder(dao dao.Repository, routeProperties *common.RouteProperties) ClusterBuilder {
	return NewEgressClusterBuilder(dao, routeProperties)
}

func getDefaultClusterBuilder(dao dao.Repository, routeProperties *common.RouteProperties) ClusterBuilder {
	return NewDefaultClusterBuilder(dao, routeProperties)
}

// Test BuildCLuster with trustCA, clientCert and PrivateKey
func TestBuildClusterWithMtls(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		name                string
		expectedTypedConfig string
		isSetEcdh           bool
		builderFunc         func(dao dao.Repository, routeProperties *common.RouteProperties) ClusterBuilder
	}{
		{
			name:                "TestEgressNodeGroupWithEcdhCurves",
			isSetEcdh:           true,
			expectedTypedConfig: fmt.Sprintf("[type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext]:{common_tls_context:{tls_params:{tls_maximum_protocol_version:TLSv1_3ecdh_curves:\"P-256\"ecdh_curves:\"P-384\"}tls_certificates:{certificate_chain:{inline_string:\"%s\"}private_key:{inline_string:\"%s\"}}validation_context:{trusted_ca:{inline_string:\"%s\"}}}}", firstClientCert, firstPrivateKey, firstCaCert),
			builderFunc:         getEgressClusterBuilder,
		},
		{
			name:                "TestEgressNodeGroupWithoutEcdhCurves",
			isSetEcdh:           false,
			expectedTypedConfig: fmt.Sprintf("[type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext]:{common_tls_context:{tls_params:{tls_maximum_protocol_version:TLSv1_3}tls_certificates:{certificate_chain:{inline_string:\"%s\"}private_key:{inline_string:\"%s\"}}validation_context:{trusted_ca:{inline_string:\"%s\"}}}}", firstClientCert, firstPrivateKey, firstCaCert),
			builderFunc:         getEgressClusterBuilder,
		},
		{
			name:                "TestDefaultNodeGroupWithEcdhCurves",
			isSetEcdh:           true,
			expectedTypedConfig: fmt.Sprintf("[type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext]:{common_tls_context:{tls_certificates:{certificate_chain:{inline_string:\"%s\"}private_key:{inline_string:\"%s\"}}validation_context:{trusted_ca:{inline_string:\"%s\"}}}}", firstClientCert, firstPrivateKey, firstCaCert),
			builderFunc:         getDefaultClusterBuilder,
		},
		{
			name:                "TestDefaultNodeGroupWithoutEcdhCurves",
			isSetEcdh:           false,
			expectedTypedConfig: fmt.Sprintf("[type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext]:{common_tls_context:{tls_certificates:{certificate_chain:{inline_string:\"%s\"}private_key:{inline_string:\"%s\"}}validation_context:{trusted_ca:{inline_string:\"%s\"}}}}", firstClientCert, firstPrivateKey, firstCaCert),
			builderFunc:         getDefaultClusterBuilder,
		},
	}

	const CLUSTER_NAME = "testClusterName"
	const TRUSTED_CA = firstCaCert
	const CLIENT_CERT = firstClientCert
	const PRIVATE_KEY = firstPrivateKey
	const TLS_ENABLED = true
	const TLS_CONFIG_NAME = "testTlsConfigName"
	const TLS_ID = 2356

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.isSetEcdh {
				_ = os.Setenv("ECDH_CURVES", "P-256,P-384")
				defer os.Unsetenv("ECDH_CURVES")
			}
			mockRepository := mock_dao.NewMockRepository(ctrl)
			clusterBuilder := test.builderFunc(mockRepository, &common.RouteProperties{})
			httpVersion := int32(1)
			tlsConfig := domain.TlsConfig{
				BaseModel:  bun.BaseModel{},
				Id:         TLS_ID,
				NodeGroups: nil,
				Name:       TLS_CONFIG_NAME,
				Enabled:    TLS_ENABLED,
				TrustedCA:  TRUSTED_CA,
				ClientCert: CLIENT_CERT,
				PrivateKey: PRIVATE_KEY,
				SNI:        "",
			}
			domainCluster := &domain.Cluster{
				Name:        CLUSTER_NAME,
				HttpVersion: &httpVersion,
				TLS:         &tlsConfig,
				ConnectionIdleTimeout: domain.NewNullInt(10),
			}

			mockRepository.EXPECT().FindDeploymentVersionsByStage(gomock.Any()).AnyTimes().Return([]*domain.DeploymentVersion{{}}, nil)
			mockRepository.EXPECT().FindEndpointsByClusterId(gomock.Any()).AnyTimes().Return([]*domain.Endpoint{
				{},
			}, nil)
			mockRepository.EXPECT().FindDeploymentVersion(gomock.Any()).AnyTimes().Return(&domain.DeploymentVersion{}, nil)
			mockRepository.EXPECT().FindHealthChecksByClusterId(gomock.Any()).AnyTimes().Return([]*domain.HealthCheck{
				{},
			}, nil)

			mockRepository.EXPECT().FindCircuitBreakerById(gomock.Any()).AnyTimes().Return(&domain.CircuitBreaker{}, nil)
			mockRepository.EXPECT().FindThresholdById(gomock.Any()).AnyTimes().Return(&domain.Threshold{}, nil)
			mockRepository.EXPECT().FindTlsConfigById(gomock.Any()).AnyTimes().Return(&tlsConfig, nil)

			cluster, _ := clusterBuilder.BuildCluster("nodeGroup", domainCluster, &common.RouteProperties{})

			actualTypedConfig := strings.ReplaceAll(cluster.TransportSocket.GetTypedConfig().String(), "\\n", "\n")
			assert.NotNil(t, cluster)
			assert.Equal(t, CLUSTER_NAME, cluster.Name)
			assert.Equal(t, clusterV3.Cluster_AUTO, cluster.DnsLookupFamily)
			assert.NotNil(t, cluster.TransportSocket.GetTypedConfig())
			assert.Equal(t, strings.ReplaceAll(test.expectedTypedConfig, " ", ""), strings.ReplaceAll(actualTypedConfig, " ", ""))
			assert.Equal(t, true, cluster.CommonHttpProtocolOptions.IdleTimeout.IsValid())
		})
	}
}

func TestClusterBuilderRemoveNotValidCerts(t *testing.T) {
	ctrl := gomock.NewController(t)
	tlsService := mock_provider.NewMockTlsService(ctrl)
	tlsService.EXPECT().GetGlobalTlsConfigs(gomock.Any(), "nodeGroup").Return([]*domain.TlsConfig{
		{
			Name:       "test1",
			Enabled:    true,
			TrustedCA:  badMiddleCaCert,
			ClientCert: invalidClientCertForMatch,
			PrivateKey: prKeyForMatch,
		},
	}, nil)
	provider.Init(tlsService)

	builder := NewDefaultClusterBuilder(nil, nil)
	config, err := builder.aggregateGlobalTlsConfig("nodeGroup", buildHttpsCluster("test"))
	assert.Nil(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "\n"+validatedCert+"\n", config.TrustedCA)
	assert.Contains(t, config.PrivateKey, prKeyForMatch)
	assert.Equal(t, "\n"+validClientCertForMatch+"\n", config.ClientCert)
}
