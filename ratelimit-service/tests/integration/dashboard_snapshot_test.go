//go:build integration
// +build integration

package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSnapshot_GrafanaDashboard guards against accidental removal of key
// metrics or panels from the Grafana dashboard JSON shipped with the chart.
//
// This is intentionally NOT a byte-for-byte snapshot — Grafana edits to
// layout/positions are common and should not break the build. We pin only
// the metric names referenced by panels, because those drive the downstream
// alerts that ops depends on.
//
func TestSnapshot_GrafanaDashboard(t *testing.T) {
	path := repoFile(t, "helm-charts/dashboards/ratelimit-service.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", path)

	var dash map[string]any
	require.NoError(t, json.Unmarshal(data, &dash),
		"dashboard JSON must be valid")

	panels, ok := dash["panels"].([]any)
	require.True(t, ok, "dashboard must have a 'panels' array")
	assert.GreaterOrEqual(t, len(panels), 1, "expect at least one panel")

	// Pin the metric names that downstream alerts and ops rely on.
	requiredMetrics := []string{
		"ratelimit_checks_total",
		"ratelimit_violating_users_total",
		"ratelimit_redis_operations_total",
		"ratelimit_config_reloads_total",
	}
	body := string(data)
	for _, m := range requiredMetrics {
		assert.True(t, strings.Contains(body, m),
			"dashboard must reference metric %q (used by alerts/ops)", m)
	}
}

// TestSnapshot_PrometheusRule guards against accidental removal of key alerts
// from values.yaml (where the PrometheusRule alerts are defined and then
// templated into helm-charts/templates/prometheus_rule.yaml).
func TestSnapshot_PrometheusRule(t *testing.T) {
	path := repoFile(t, "helm-charts/values.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	body := string(data)
	requiredAlerts := []string{
		"RateLimitHighRejectionRate",
		"RateLimitViolatingUsersHigh",
		"RateLimitOperatorDown",
		"RateLimitRedisDown",
	}
	for _, a := range requiredAlerts {
		assert.True(t, strings.Contains(body, a),
			"values.yaml must define alert %q", a)
	}
}

// repoFile returns the absolute path to a file relative to the module root,
// using the location of this test file as anchor.
func repoFile(t *testing.T, relPath string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// thisFile = .../ratelimit-service/tests/integration/dashboard_snapshot_test.go
	// module root = two levels up from tests/integration/
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(root, relPath)
}
