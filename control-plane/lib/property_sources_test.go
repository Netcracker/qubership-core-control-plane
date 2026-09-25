package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigPropertySources_LegacyModeAddsPodSecrets(t *testing.T) {
	legacy := configPropertySources(false)
	dbaasOperator := configPropertySources(true)

	assert.Len(t, legacy, len(dbaasOperator)+1, "legacy mode must add exactly the pod-secrets source")
}

func TestConfigPropertySources_DbaasOperatorModeKeepsBaseSources(t *testing.T) {
	dbaasOperator := configPropertySources(true)
	legacy := configPropertySources(false)

	for i := range dbaasOperator {
		assert.IsType(t, legacy[i].Provider, dbaasOperator[i].Provider, "source %d", i)
	}
}
