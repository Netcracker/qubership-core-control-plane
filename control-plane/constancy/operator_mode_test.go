package constancy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperatorModeEnabled(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"false", false},
		{"", false},
		{"1", false},
		{"yes", false},
		{" true", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv(OperatorModeEnvVar, tc.value)
			assert.Equal(t, tc.want, OperatorModeEnabled())
		})
	}
}

func TestOperatorModeEnabled_Unset(t *testing.T) {
	t.Setenv(OperatorModeEnvVar, "true")
	assert.NoError(t, os.Unsetenv(OperatorModeEnvVar))
	assert.False(t, OperatorModeEnabled())
}
