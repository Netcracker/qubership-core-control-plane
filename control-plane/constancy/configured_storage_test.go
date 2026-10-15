package constancy

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
)

func legacyCredentials() *PostgresStorageConfigurator {
	return &PostgresStorageConfigurator{dbHost: "pg-host", dbPort: "5432", dbName: "cp",
		dbUserName: "user", dbPassword: "password", dbRole: "admin"}
}

// The pool must have no application provider in DBaaS Operator mode: a provider there answers before
// the base client looks at the mounted DBaaS Operator Secret, so the Secret would never be read.
func TestLogicalDbProviders_NoneInDbaasOperatorMode(t *testing.T) {
	configureCalls := 0

	providers, err := logicalDbProviders(context.Background(), true, func() (*PostgresStorageConfigurator, error) {
		configureCalls++
		return legacyCredentials(), nil
	})

	assert.NoError(t, err)
	assert.Empty(t, providers)
	assert.Zero(t, configureCalls, "DBaaS Operator mode must not read the legacy credentials")
}

func TestLogicalDbProviders_AggregatorProviderInLegacyMode(t *testing.T) {
	providers, err := logicalDbProviders(context.Background(), false, func() (*PostgresStorageConfigurator, error) {
		return legacyCredentials(), nil
	})

	assert.NoError(t, err)
	if assert.Len(t, providers, 1) {
		assert.Equal(t, NewDbaasAggregatorLogicalDbProvider(legacyCredentials()), providers[0])
	}
}

func TestLogicalDbProviders_ReportsMissingLegacyCredentials(t *testing.T) {
	_, err := logicalDbProviders(context.Background(), false, func() (*PostgresStorageConfigurator, error) {
		return nil, errors.New("can't find property pg.host")
	})

	assert.EqualError(t, err, "can't find property pg.host")
}

func TestLogicalDbProviders_RejectsEmptyLegacyCredentials(t *testing.T) {
	_, err := logicalDbProviders(context.Background(), false, func() (*PostgresStorageConfigurator, error) {
		cfg := legacyCredentials()
		cfg.dbRole = ""
		return cfg, nil
	})

	assert.EqualError(t, err, "database name, username, password, role or address must not be empty")
}

// The pool is built without contacting a database, in both modes.
func TestNewDbProvider(t *testing.T) {
	serviceloader.Register(1, &security.DummyToken{})
	assert.NoError(t, os.Setenv("microservice.namespace", "test"))
	t.Cleanup(func() { _ = os.Unsetenv("microservice.namespace") })
	configloader.Init(configloader.EnvPropertySource())
	configure := func() (*PostgresStorageConfigurator, error) { return legacyCredentials(), nil }

	for _, dbaasOperatorMode := range []bool{true, false} {
		provider, err := newDbProvider(context.Background(), dbaasOperatorMode, configure)

		assert.NoError(t, err, "DBaaS Operator mode %t", dbaasOperatorMode)
		assert.NotNil(t, provider, "DBaaS Operator mode %t", dbaasOperatorMode)
	}
}

func TestNewDbProvider_ReportsMissingLegacyCredentials(t *testing.T) {
	_, err := newDbProvider(context.Background(), false, func() (*PostgresStorageConfigurator, error) {
		return nil, errors.New("can't find property pg.host")
	})

	assert.EqualError(t, err, "can't find property pg.host")
}

// Without DBaaS Operator mode and without pg.* properties, startup stops before it connects.
func TestNewConfiguredStorage_PanicsWithoutLegacyCredentials(t *testing.T) {
	t.Setenv(DbaasOperatorModeEnvVar, "false")

	assert.PanicsWithError(t, "can't find property pg.host", func() {
		NewConfiguredStorage(context.Background())
	})
}
