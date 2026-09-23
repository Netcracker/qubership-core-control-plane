package constancy

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/db"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
)

// withStorageStubs replaces the constructors NewConfiguredStorage chooses between, so the choice can
// be covered without a database, and restores them when the test ends.
func withStorageStubs(t *testing.T, operator func(context.Context) *StorageImpl,
	legacy func(context.Context, Configurator) *StorageImpl,
	configure func() (*PostgresStorageConfigurator, error)) {
	t.Helper()
	previousOperator, previousLegacy, previousConfigure := operatorStorage, legacyStorage, configureLegacy
	t.Cleanup(func() {
		operatorStorage, legacyStorage, configureLegacy = previousOperator, previousLegacy, previousConfigure
	})
	operatorStorage, legacyStorage, configureLegacy = operator, legacy, configure
}

func TestNewConfiguredStorage_UsesTheOperatorSecret(t *testing.T) {
	t.Setenv(OperatorModeEnvVar, "true")
	operatorCalls, legacyCalls, configureCalls := 0, 0, 0
	expected := &StorageImpl{}
	withStorageStubs(t,
		func(context.Context) *StorageImpl { operatorCalls++; return expected },
		func(context.Context, Configurator) *StorageImpl { legacyCalls++; return nil },
		func() (*PostgresStorageConfigurator, error) { configureCalls++; return nil, nil })

	storage := NewConfiguredStorage(context.Background())

	assert.Same(t, expected, storage)
	assert.Equal(t, 1, operatorCalls)
	// The legacy credentials are neither read nor required in operator mode.
	assert.Zero(t, legacyCalls)
	assert.Zero(t, configureCalls)
}

func TestNewConfiguredStorage_UsesTheLegacyCredentials(t *testing.T) {
	t.Setenv(OperatorModeEnvVar, "false")
	operatorCalls := 0
	expected := &StorageImpl{}
	configured := &PostgresStorageConfigurator{dbHost: "pg-host"}
	var got Configurator
	withStorageStubs(t,
		func(context.Context) *StorageImpl { operatorCalls++; return nil },
		func(_ context.Context, cfg Configurator) *StorageImpl { got = cfg; return expected },
		func() (*PostgresStorageConfigurator, error) { return configured, nil })

	storage := NewConfiguredStorage(context.Background())

	assert.Same(t, expected, storage)
	assert.Same(t, configured, got)
	assert.Zero(t, operatorCalls)
}

func TestNewConfiguredStorage_PanicsWithoutLegacyCredentials(t *testing.T) {
	t.Setenv(OperatorModeEnvVar, "false")
	withStorageStubs(t,
		func(context.Context) *StorageImpl { return nil },
		func(context.Context, Configurator) *StorageImpl { return nil },
		func() (*PostgresStorageConfigurator, error) { return nil, errors.New("can't find property pg.host") })

	assert.PanicsWithError(t, "can't find property pg.host", func() {
		NewConfiguredStorage(context.Background())
	})
}

// The pools themselves are built without contacting a database, so both providers can be exercised.
func TestDbProviders(t *testing.T) {
	serviceloader.Register(1, &security.DummyToken{})
	assert.NoError(t, os.Setenv("microservice.namespace", "test"))
	t.Cleanup(func() { _ = os.Unsetenv("microservice.namespace") })
	configloader.Init(configloader.EnvPropertySource())

	t.Run("operator", func(t *testing.T) {
		provider, err := operatorDbProvider(context.Background())()

		assert.NoError(t, err)
		assert.NotNil(t, provider)
	})

	t.Run("legacy", func(t *testing.T) {
		cfg := &PostgresStorageConfigurator{dbHost: "pg-host", dbPort: "5432", dbName: "cp",
			dbUserName: "user", dbPassword: "password", dbRole: "admin"}

		provider, err := legacyDbProvider(cfg)()

		assert.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

// withStorageSeam replaces the shared constructor, so NewStorage and NewOperatorStorage can be
// covered without a database. It records the provider each was given, without calling it.
func withStorageSeam(t *testing.T, built *StorageImpl) *func() (db.DBProvider, error) {
	t.Helper()
	var given func() (db.DBProvider, error)
	previous := newStorage
	t.Cleanup(func() { newStorage = previous })
	newStorage = func(_ context.Context, getProvider func() (db.DBProvider, error)) *StorageImpl {
		given = getProvider
		return built
	}
	return &given
}

func TestNewOperatorStorage(t *testing.T) {
	expected := &StorageImpl{}
	given := withStorageSeam(t, expected)

	storage := NewOperatorStorage(context.Background())

	assert.Same(t, expected, storage)
	assert.NotNil(t, *given)
}

func TestNewStorage_UsesTheConfiguredCredentials(t *testing.T) {
	expected := &StorageImpl{}
	given := withStorageSeam(t, expected)
	cfg := &PostgresStorageConfigurator{dbHost: "pg-host", dbPort: "5432", dbName: "cp",
		dbUserName: "user", dbPassword: "password", dbRole: "admin"}

	storage := NewStorage(context.Background(), cfg)

	assert.Same(t, expected, storage)
	assert.NotNil(t, *given)
}
