package constancy

import (
	"os"
	"testing"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
)

var legacyCredentialEnv = []string{"PG_HOST", "PG_PORT", "PG_DB", "PG_USER", "PG_PASSWD", "PG_TLS", "PG_ROLE"}

// withLegacyCredentials exposes exactly the given pg.* properties, the way the pod-secrets
// property source would, and reloads the configuration. Keys missing from values are unset rather
// than set to "", so the pg.tls and pg.role defaults still apply. The environment and the global
// configuration are restored when the test ends.
func withLegacyCredentials(t *testing.T, values map[string]string) {
	t.Helper()
	for _, key := range legacyCredentialEnv {
		previous, wasSet := os.LookupEnv(key)
		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv(key, previous)
			} else {
				_ = os.Unsetenv(key)
			}
		})
		if value, ok := values[key]; ok {
			assert.NoError(t, os.Setenv(key, value))
		} else {
			assert.NoError(t, os.Unsetenv(key))
		}
	}
	// Registered last so it runs first, then the environment is restored; reload once more after
	// that so later tests see the original configuration.
	t.Cleanup(func() { configloader.Init(configloader.EnvPropertySource()) })
	configloader.Init(configloader.EnvPropertySource())
}

func TestNewPostgresStorageConfigurator_RequiresCredentialsWithoutOperatorSecret(t *testing.T) {
	withLegacyCredentials(t, nil)

	cfg, err := newPostgresStorageConfigurator(true)

	assert.Nil(t, cfg)
	assert.EqualError(t, err, "can't find property pg.host")
}

func TestNewPostgresStorageConfigurator_ReportsFirstMissingProperty(t *testing.T) {
	withLegacyCredentials(t, map[string]string{"PG_HOST": "pg-host"})

	_, err := newPostgresStorageConfigurator(true)

	assert.EqualError(t, err, "can't find property pg.port")
}

func TestNewPostgresStorageConfigurator_ToleratesMissingCredentialsWithOperatorSecret(t *testing.T) {
	withLegacyCredentials(t, nil)

	cfg, err := newPostgresStorageConfigurator(false)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.GetDBHost())
	assert.Equal(t, "false", cfg.GetDBTls())
	assert.Equal(t, "admin", cfg.GetDBRole())
}

func TestNewPostgresStorageConfigurator_ReadsLegacyCredentials(t *testing.T) {
	withLegacyCredentials(t, map[string]string{
		"PG_HOST":   "pg-host",
		"PG_PORT":   "5432",
		"PG_DB":     "control-plane",
		"PG_USER":   "cp-user",
		"PG_PASSWD": "cp-password",
		"PG_TLS":    "true",
		"PG_ROLE":   "rw",
	})

	cfg, err := newPostgresStorageConfigurator(true)

	assert.NoError(t, err)
	assert.Equal(t, "pg-host", cfg.GetDBHost())
	assert.Equal(t, "5432", cfg.GetDBPort())
	assert.Equal(t, "control-plane", cfg.GetDBName())
	assert.Equal(t, "cp-user", cfg.GetDBUserName())
	assert.Equal(t, "cp-password", cfg.GetDBPassword())
	assert.Equal(t, "true", cfg.GetDBTls())
	assert.Equal(t, "rw", cfg.GetDBRole())
}
