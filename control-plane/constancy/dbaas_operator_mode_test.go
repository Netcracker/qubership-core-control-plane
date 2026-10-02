package constancy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDbaasOperatorModeEnabled(t *testing.T) {
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
			t.Setenv(DbaasOperatorModeEnvVar, tc.value)
			assert.Equal(t, tc.want, DbaasOperatorModeEnabled())
		})
	}
}

func TestDbaasOperatorModeEnabled_Unset(t *testing.T) {
	t.Setenv(DbaasOperatorModeEnvVar, "true")
	assert.NoError(t, os.Unsetenv(DbaasOperatorModeEnvVar))
	assert.False(t, DbaasOperatorModeEnabled())
}
