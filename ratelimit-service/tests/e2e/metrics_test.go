// tests/e2e/metrics_test.go
//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"ratelimit-service/pkg/utils"
	"ratelimit-service/tests/e2e/setup"
	"ratelimit-service/tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	metricsPort = "9090"
)

func TestE2E_MetricsEndpoint(t *testing.T) {
	t.Log("=== Setting up metrics test environment ===")

	kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")

	// Redis port-forward (6379) is provided by the Makefile via port-forward.sh --profile=local --start.
	operator, err := setup.NewLocalOperator(kubeconfig)
	require.NoError(t, err)
	require.NoError(t, operator.Start(context.Background(), operatorPort))
	defer operator.Stop()

	time.Sleep(5 * time.Second)

	metricsURL := fmt.Sprintf("http://localhost:%s/metrics", metricsPort)

	t.Log("\n=== Testing metrics endpoint availability ===")

	var resp *http.Response
	for i := 0; i < 5; i++ {
		resp, err = http.Get(metricsURL)
		if err == nil {
			break
		}
		t.Logf("Attempt %d: metrics endpoint not ready yet, waiting...", i+1)
		time.Sleep(2 * time.Second)
	}

	require.NoError(t, err, "Metrics endpoint should be available")
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Metrics endpoint should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metricsStr := string(body)
	t.Logf("Metrics endpoint accessible, response length: %d bytes", len(metricsStr))

	expectedMetrics := []string{
		"ratelimit_violating_users_total",
		"ratelimit_active_limits_total",
		"ratelimit_redis_operations_total",
		"ratelimit_config_reloads_total",
		"ratelimit_redis_keys_total",
		"ratelimit_redis_memory_bytes",
		"ratelimit_redis_connected_clients",
		"ratelimit_redis_hit_rate",
	}

	missingMetrics := []string{}
	for _, metric := range expectedMetrics {
		if !assert.Contains(t, metricsStr, metric, "Should contain metric: %s", metric) {
			missingMetrics = append(missingMetrics, metric)
		}
	}

	if len(missingMetrics) > 0 {
		t.Logf("Missing metrics: %v", missingMetrics)
		preview := metricsStr
		if len(preview) > 500 {
			preview = preview[:500]
		}
		t.Logf("Metrics preview: %s", preview)
	}

	t.Log("Metrics endpoint test completed")
}

func TestE2E_MetricsCollection(t *testing.T) {
	t.Log("=== Testing metrics collection ===")

	ctx := context.Background()
	kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")

	// Build k8s clientset for ConfigMap management.
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	testNs := utils.GetEnv("NAMESPACE", "core-1-core")

	// Redis port-forward (6379) is provided by the Makefile.
	operator, err := setup.NewLocalOperator(kubeconfig)
	require.NoError(t, err)
	require.NoError(t, operator.Start(ctx, operatorPort))
	defer operator.Stop()

	time.Sleep(5 * time.Second)

	operatorURL := fmt.Sprintf("http://localhost:%s", operatorPort)
	metricsURL := fmt.Sprintf("http://localhost:%s/metrics", metricsPort)
	userID := "metrics-test-user"

	t.Log("\n=== Adding rate limit rule via ConfigMap ===")

	configYAML := `
domain: auth_limit
separator: "|"
descriptors:
  - key: user_id
    value_regex: "metrics-test-user"
    rate_limit:
      unit: minute
      requests_per_unit: 2
    algorithm: fixed_window
    priority: 100
`
	helpers.SetRules(ctx, t, clientset, testNs, "e2e-metrics-test", configYAML)

	// Force immediate reconciliation.
	_, _ = http.Post(operatorURL+"/api/v1/config/reload", "application/json", nil)
	time.Sleep(500 * time.Millisecond)
	t.Log("Rule added via ConfigMap")

	t.Log("\n=== Making rate limit requests ===")

	for i := 0; i < 5; i++ {
		checkReq := map[string]interface{}{
			"components": map[string]string{
				"path":    "/test",
				"user_id": userID,
			},
		}
		body, _ := json.Marshal(checkReq)
		resp, err := http.Post(operatorURL+"/api/v1/ratelimit/check", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Logf("Request %d failed: %v", i+1, err)
			continue
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			if allowed, ok := result["allowed"].(bool); ok {
				t.Logf("Request %d: allowed=%v", i+1, allowed)
			}
		}
		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("\n=== Checking metrics after requests ===")

	time.Sleep(3 * time.Second)

	resp, err := http.Get(metricsURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	metricsStr := string(bodyBytes)

	preview := metricsStr
	if len(preview) > 800 {
		preview = preview[:800]
	}
	t.Logf("Metrics collected (first 800 chars):\n%s", preview)

	if !assert.Contains(t, metricsStr, "ratelimit_active_limits_total", "Should have active limits") {
		t.Log("Note: Some metrics may be zero if no rate limits were triggered")
	}

	t.Log("Metrics collection test completed")
	// ConfigMap cleanup is handled by helpers.SetRules t.Cleanup.
}

func TestE2E_MetricsServerOnly(t *testing.T) {
	t.Log("=== Testing metrics server only ===")

	kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")

	// Redis port-forward (6379) is provided by the Makefile.
	operator, err := setup.NewLocalOperator(kubeconfig)
	require.NoError(t, err)
	require.NoError(t, operator.Start(context.Background(), operatorPort))
	defer operator.Stop()

	time.Sleep(5 * time.Second)

	metricsURL := fmt.Sprintf("http://localhost:%s/metrics", metricsPort)

	for i := 0; i < 3; i++ {
		resp, err := http.Get(metricsURL)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)

		metricsStr := string(body)
		t.Logf("Attempt %d: Got %d bytes of metrics", i+1, len(metricsStr))

		assert.Contains(t, metricsStr, "ratelimit_redis_connected_clients", "Should have Redis clients metric")
		assert.Contains(t, metricsStr, "ratelimit_redis_hit_rate", "Should have Redis hit rate metric")

		time.Sleep(2 * time.Second)
	}

	t.Log("Metrics server test completed")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
