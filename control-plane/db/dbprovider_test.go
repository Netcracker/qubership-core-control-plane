package db

import (
	"context"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security.DummyToken{})
	os.Exit(m.Run())
}

// Test error cases
// Required to suppress coverage requirement

func TestBuildServiceDbParams(t *testing.T) {
	params := buildServiceDbParams()
	assert.NotNil(t, params.Classifier)
	assert.Equal(t, "admin", params.BaseDbParams.Role)
}

func TestNewDBProvider(t *testing.T) {
	os.Setenv("microservice.namespace", "test")
	configloader.Init(configloader.EnvPropertySource())
	dbaasPool := dbaasbase.NewDbaaSPool()
	provider, err := NewDBProvider(dbaasPool)
	assert.Nil(t, err)
	assert.NotNil(t, provider)
}

func TestGetBunDB(t *testing.T) {
	os.Setenv("microservice.namespace", "test")
	configloader.Init(configloader.EnvPropertySource())
	dbaasPool := dbaasbase.NewDbaaSPool()
	provider, err := NewDBProvider(dbaasPool)
	assert.Nil(t, err)

	_, err = provider.GetDB(context.Background())
	assert.NotNil(t, err)
}

func TestGetConn(t *testing.T) {
	os.Setenv("microservice.namespace", "test")
	configloader.Init(configloader.EnvPropertySource())
	dbaasPool := dbaasbase.NewDbaaSPool()
	provider, err := NewDBProvider(dbaasPool)
	assert.Nil(t, err)

	_, err = provider.GetConn(context.Background())
	assert.NotNil(t, err)
}
