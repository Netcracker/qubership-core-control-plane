package db

import (
	"context"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security.DummyToken{})
	os.Exit(m.Run())
}

// Test error cases
// Required to suppress coverage requirement

func TestNewDBProvider(t *testing.T) {
	os.Setenv("microservice.namespace", "test")
	os.Setenv("microservice.name", "control-plane")
	configloader.Init(configloader.EnvPropertySource())
	dbaasPool := dbaasbase.NewDbaaSPool()
	provider, err := NewDBProvider(dbaasPool)
	assert.Nil(t, err)
	assert.NotNil(t, provider)
}

func TestGetBunDB(t *testing.T) {
	configureUnavailableDbaaS(t)
	os.Setenv("microservice.namespace", "test")
	os.Setenv("microservice.name", "control-plane")
	configloader.Init(configloader.EnvPropertySource())
	dbaasPool := dbaasbase.NewDbaaSPool()
	provider, err := NewDBProvider(dbaasPool)
	assert.Nil(t, err)

	_, err = provider.GetDB(context.Background())
	assert.NotNil(t, err)
}

func TestGetConn(t *testing.T) {
	configureUnavailableDbaaS(t)
	os.Setenv("microservice.namespace", "test")
	os.Setenv("microservice.name", "control-plane")
	configloader.Init(configloader.EnvPropertySource())
	dbaasPool := dbaasbase.NewDbaaSPool()
	provider, err := NewDBProvider(dbaasPool)
	assert.Nil(t, err)

	_, err = provider.GetConn(context.Background())
	assert.NotNil(t, err)
}

func TestCreateControlPlaneServiceClassifier_UsesConfiguredServiceName(t *testing.T) {
	os.Setenv("microservice.namespace", "test")
	os.Setenv("microservice.name", "custom-control-plane")
	defer os.Setenv("microservice.name", "control-plane")
	configloader.Init(configloader.EnvPropertySource())

	classifier := createControlPlaneServiceClassifier(context.Background())

	assert.Equal(t, map[string]interface{}{
		"namespace":        "test",
		"microserviceName": "custom-control-plane",
		"scope":            "service",
	}, classifier)
}

// Error-path tests use a local DBaaS endpoint rather than cluster DNS and retries.
func configureUnavailableDbaaS(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "DBaaS unavailable for this test", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)
	t.Setenv("dbaas.agent", server.URL)
	t.Setenv("M2M_AUTH_MODE", "legacy")
}
