package lib

import (
	"fmt"
	"testing"

	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/podsecrets-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
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
