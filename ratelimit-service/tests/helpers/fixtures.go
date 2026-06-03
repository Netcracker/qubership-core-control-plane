package helpers

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// FixturePath returns the absolute path to a fixture file under tests/fixtures/.
// It is robust to the test's working directory because it derives the project
// root from the location of this source file.
func FixturePath(t *testing.T, name string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")

	// thisFile == .../ratelimit-service/tests/helpers/fixtures.go
	// project root == .../ratelimit-service
	helpersDir := filepath.Dir(thisFile)
	testsDir := filepath.Dir(helpersDir)
	return filepath.Join(testsDir, "fixtures", name)
}

// LoadFixture reads a fixture file from tests/fixtures/ as a string.
func LoadFixture(t *testing.T, name string) string {
	t.Helper()

	path := FixturePath(t, name)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading fixture %s", path)
	return string(data)
}
