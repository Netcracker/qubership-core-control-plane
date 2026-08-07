package grpc

import (
	"os"
	"testing"

	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
)

func TestClusterHash_ID_stripsNamespaceSuffix(t *testing.T) {
	os.Setenv("MICROSERVICE_NAMESPACE", "tenant-a")
	defer os.Unsetenv("MICROSERVICE_NAMESPACE")
	configloader.Init(configloader.EnvPropertySource())

	hash := ClusterHash{}
	assert.Equal(t, "public-gateway-service", hash.ID(&v3core.Node{Cluster: "public-gateway-service-tenant-a"}))
	assert.Equal(t, "egress-gateway", hash.ID(&v3core.Node{Cluster: "egress-gateway-tenant-a"}))
	assert.Equal(t, "public-gateway-service", hash.ID(&v3core.Node{Cluster: "public-gateway-service"}))
	assert.Equal(t, "", hash.ID(nil))
}

func TestClusterHash_ID_withoutNamespaceEnv(t *testing.T) {
	os.Unsetenv("MICROSERVICE_NAMESPACE")
	os.Unsetenv("CLOUD_NAMESPACE")
	os.Unsetenv("microservice.namespace")
	configloader.Init(configloader.EnvPropertySource())

	hash := ClusterHash{}
	assert.Equal(t, "public-gateway-service-tenant-a", hash.ID(&v3core.Node{Cluster: "public-gateway-service-tenant-a"}))
}

func TestClusterHash_ID_ignoresPlaceholderNamespace(t *testing.T) {
	os.Setenv("MICROSERVICE_NAMESPACE", "unknown")
	defer os.Unsetenv("MICROSERVICE_NAMESPACE")
	configloader.Init(configloader.EnvPropertySource())

	hash := ClusterHash{}
	assert.Equal(t, "public-gateway-service-unknown", hash.ID(&v3core.Node{Cluster: "public-gateway-service-unknown"}))
}

func TestClusterHash_ID_fallsBackToCloudNamespace(t *testing.T) {
	os.Unsetenv("MICROSERVICE_NAMESPACE")
	os.Setenv("CLOUD_NAMESPACE", "tenant-b")
	defer os.Unsetenv("CLOUD_NAMESPACE")
	configloader.Init(configloader.EnvPropertySource())

	hash := ClusterHash{}
	assert.Equal(t, "public-gateway-service", hash.ID(&v3core.Node{Cluster: "public-gateway-service-tenant-b"}))
}
