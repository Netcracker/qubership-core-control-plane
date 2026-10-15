package lib

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/podsecrets-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/stretchr/testify/assert"
)

// providerTypes names each source by its provider type, so a mismatch prints both lists in full.
func providerTypes(sources []*configloader.PropertySource) []string {
	types := make([]string, len(sources))
	for i, source := range sources {
		types[i] = fmt.Sprintf("%T", source.Provider)
	}
	return types
}

func TestConfigPropertySources_LegacyModeAddsPodSecrets(t *testing.T) {
	withPodSecrets := podsecrets.AddPodSecretsPropertySource(configloader.BasePropertySources())

	assert.Equal(t, providerTypes(withPodSecrets), providerTypes(configPropertySources(false)))
}

func TestConfigPropertySources_DbaasOperatorModeKeepsBaseSources(t *testing.T) {
	assert.Equal(t, providerTypes(configloader.BasePropertySources()), providerTypes(configPropertySources(true)))
}

func TestStartPodSecretsWatcher(t *testing.T) {
	logger = logging.GetLogger("server")

	t.Run("not in DBaaS Operator mode", func(t *testing.T) {
		t.Setenv(podsecrets.EnvSecretsDir, t.TempDir())

		stop := startPodSecretsWatcher(false)

		if assert.NotNil(t, stop, "the watcher must run outside DBaaS Operator mode") {
			stop()
		}
	})

	t.Run("in DBaaS Operator mode", func(t *testing.T) {
		t.Setenv(podsecrets.EnvSecretsDir, t.TempDir())

		assert.Nil(t, startPodSecretsWatcher(true), "no watcher may run in DBaaS Operator mode")
	})

	t.Run("without the pod-secrets directory", func(t *testing.T) {
		t.Setenv(podsecrets.EnvSecretsDir, filepath.Join(t.TempDir(), "missing"))

		assert.Nil(t, startPodSecretsWatcher(false))
	})
}
