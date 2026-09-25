package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigPropertySources_LegacyModeAddsPodSecrets(t *testing.T) {
	legacy := configPropertySources(false)
	operator := configPropertySources(true)

	assert.Len(t, legacy, len(operator)+1, "legacy mode must add exactly the pod-secrets source")
}

func TestConfigPropertySources_OperatorModeKeepsBaseSources(t *testing.T) {
	operator := configPropertySources(true)
	legacy := configPropertySources(false)

	for i := range operator {
		assert.IsType(t, legacy[i].Provider, operator[i].Provider, "source %d", i)
	}
}
