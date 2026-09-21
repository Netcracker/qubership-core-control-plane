package constancy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model/rest"
	"github.com/stretchr/testify/assert"
)

func writeSecretDir(t *testing.T, basePath, secretName, metadata string) {
	t.Helper()
	dir := filepath.Join(basePath, secretName)
	assert.NoError(t, os.MkdirAll(dir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(metadata), 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "connectionProperties.json"), []byte(`{"url":"postgresql://pg:5432/db"}`), 0o644))
}

func TestHasMountedDbaasSecret_MissingDirectory(t *testing.T) {
	assert.False(t, hasMountedDbaasSecret(filepath.Join(t.TempDir(), "absent")))
}

func TestHasMountedDbaasSecret_EmptyDirectory(t *testing.T) {
	assert.False(t, hasMountedDbaasSecret(t.TempDir()))
}

func TestHasMountedDbaasSecret_IgnoresPlainFiles(t *testing.T) {
	basePath := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(basePath, "metadata.json"), []byte(`{}`), 0o644))
	assert.False(t, hasMountedDbaasSecret(basePath))
}

func TestHasMountedDbaasSecret_IgnoresDirectoryWithoutMetadata(t *testing.T) {
	basePath := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(basePath, "some-secret"), 0o755))
	assert.False(t, hasMountedDbaasSecret(basePath))
}

func TestHasMountedDbaasSecret_IgnoresEmptyMetadata(t *testing.T) {
	basePath := t.TempDir()
	writeSecretDir(t, basePath, "control-plane-postgresql-service-default-credentials", "")
	assert.False(t, hasMountedDbaasSecret(basePath))
}

func TestHasMountedDbaasSecret_DetectsOperatorSecret(t *testing.T) {
	basePath := t.TempDir()
	writeSecretDir(t, basePath, "control-plane-postgresql-service-default-credentials",
		`{"classifier":{"microserviceName":"control-plane","namespace":"cp-ns","scope":"service"},"type":"postgresql"}`)
	assert.True(t, hasMountedDbaasSecret(basePath))
}

func testLocalDbProvider(mountedSecretsPath string) *DbaasAggregatorLogicalDbProvider {
	return &DbaasAggregatorLogicalDbProvider{
		host:               "pg-host",
		port:               "5432",
		username:           "cp-user",
		password:           "cp-password",
		database:           "control-plane",
		tls:                "false",
		role:               "admin",
		mountedSecretsPath: mountedSecretsPath,
	}
}

func TestLocalProviderServesPodSecretsWithoutMountedSecret(t *testing.T) {
	provider := testLocalDbProvider(t.TempDir())
	classifier := map[string]interface{}{"microserviceName": "control-plane", "namespace": "cp-ns", "scope": "service"}

	logicalDb, err := provider.GetOrCreateDb("postgresql", classifier, rest.BaseDbParams{})
	assert.NoError(t, err)
	assert.NotNil(t, logicalDb)
	assert.Equal(t, "cp-user", logicalDb.ConnectionProperties["username"])
	assert.Equal(t, "postgresql://pg-host:5432/control-plane", logicalDb.ConnectionProperties["url"])

	connection, err := provider.GetConnection("postgresql", classifier, rest.BaseDbParams{})
	assert.NoError(t, err)
	assert.Equal(t, "cp-password", connection["password"])
}

func TestLocalProviderDefersToMountedSecret(t *testing.T) {
	basePath := t.TempDir()
	writeSecretDir(t, basePath, "control-plane-postgresql-service-default-credentials",
		`{"classifier":{"microserviceName":"control-plane","namespace":"cp-ns","scope":"service"},"type":"postgresql"}`)
	provider := testLocalDbProvider(basePath)
	classifier := map[string]interface{}{"microserviceName": "control-plane", "namespace": "cp-ns", "scope": "service"}

	// A nil result with a nil error is how the base client is told to try the next provider,
	// which is its own mounted-secret provider, and only then the DBaaS REST API.
	logicalDb, err := provider.GetOrCreateDb("postgresql", classifier, rest.BaseDbParams{})
	assert.NoError(t, err)
	assert.Nil(t, logicalDb)

	connection, err := provider.GetConnection("postgresql", classifier, rest.BaseDbParams{})
	assert.NoError(t, err)
	assert.Nil(t, connection)
}

func TestNewDbaasAggregatorLogicalDbProviderUsesDefaultMountPath(t *testing.T) {
	cfg := &PostgresStorageConfigurator{dbHost: "pg-host", dbPort: "5432", dbName: "control-plane",
		dbUserName: "cp-user", dbPassword: "cp-password", dbTls: "false", dbRole: "admin"}
	assert.Equal(t, dbaasMountedSecretsPath, NewDbaasAggregatorLogicalDbProvider(cfg).mountedSecretsPath)
}
