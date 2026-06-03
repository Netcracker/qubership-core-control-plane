//go:build cloud_e2e
// +build cloud_e2e

package cloud_e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"ratelimit-service/pkg/utils"
	"ratelimit-service/tests/helpers"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	gatewayPort = "8080"
	operatorPort = "8082"
	metricsPort  = "9090"
)

var namespace = utils.GetEnv("NAMESPACE", "core-1-core")

func TestCloudE2E_RateLimitThroughGateway(t *testing.T) {
	t.Log("=== Setting up cloud E2E test ===")

	ctx := context.Background()

	// Port-forwards (gateway, operator, redis, metrics) are provided by the Makefile
	// via port-forward.sh --profile=cloud --start.

	kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err, "building k8s rest config")
	clientset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err, "creating k8s clientset")

	gatewayURL := fmt.Sprintf("http://localhost:%s", gatewayPort)
	operatorURL := fmt.Sprintf("http://localhost:%s", operatorPort)
	metricsURL := fmt.Sprintf("http://localhost:%s", metricsPort)
	userID := "cloud-e2e-user"

	t.Log("\n=== Cleaning up existing keys for cloud-e2e-user ===")
	cleanupRedisKeys("cloud-e2e-user")

	t.Log("\n=== Adding rate limit rule via ConfigMap: 2 requests per minute ===")

	configYAML := `
domain: auth_limit
separator: "|"
descriptors:
  - key: user_id
    value_regex: "cloud-e2e-user"
    rate_limit:
      unit: minute
      requests_per_unit: 2
    algorithm: fixed_window
    priority: 100
`
	helpers.SetRules(ctx, t, clientset, namespace, "cloud-e2e-rttg", configYAML)

	// Force immediate reconciliation.
	_, _ = http.Post(operatorURL+"/api/v1/config/reload", "application/json", nil)
	time.Sleep(3 * time.Second)
	t.Log("Rate limit rule added")

	t.Log("\n=== Testing custom rate limit (2 requests per minute) ===")

	// First 2 requests should be allowed
	for i := 0; i < 2; i++ {
		statusCode, err := sendGatewayRequest(gatewayURL, "/test", userID)
		require.NoError(t, err)
		t.Logf("Request %d: HTTP %d", i+1, statusCode)
		assert.Equal(t, 200, statusCode, "First 2 requests should be allowed")
		time.Sleep(100 * time.Millisecond)
	}

	// Third request should be rate limited
	statusCode, err := sendGatewayRequest(gatewayURL, "/test", userID)
	require.NoError(t, err)
	t.Logf("Request 3: HTTP %d", statusCode)
	assert.Equal(t, 429, statusCode, "Third request should be rate limited")

	t.Log("\n=== Getting violating users (before reset) ===")

	violatingUsersJSON, err := getViolatingUsersRaw(operatorURL)
	require.NoError(t, err)
	t.Logf("Violating users API response:\n%s", violatingUsersJSON)

	t.Log("\n=== Testing rate limit reset ===")

	err = resetUserRateLimit(operatorURL, userID)
	require.NoError(t, err)
	t.Log("Rate limits reset")

	time.Sleep(2 * time.Second)

	// After reset, request should be allowed immediately
	statusCode, err = sendGatewayRequest(gatewayURL, "/test", userID)
	require.NoError(t, err)
	t.Logf("Request after reset: HTTP %d", statusCode)
	assert.Equal(t, 200, statusCode, "Request after reset should be allowed")

	time.Sleep(2 * time.Second)

	t.Log("\n=== Getting violating users (after reset) ===")

	violatingUsersAfter, err := getViolatingUsers(operatorURL)
	require.NoError(t, err)
	t.Logf("Violating users after reset: %v", violatingUsersAfter)

	// Note: Reset might take a moment to propagate
	if len(violatingUsersAfter) > 0 {
		t.Logf("Warning: Violating users still present after reset: %v", violatingUsersAfter)
	}

	t.Log("\n=== Testing Redis keys ===")

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	defer rdb.Close()

	keys, err := rdb.Keys(context.Background(), "*").Result()
	require.NoError(t, err)
	t.Logf("Redis keys found: %d", len(keys))
	for _, key := range keys {
		val, _ := rdb.Get(context.Background(), key).Result()
		t.Logf("  Key: %s = %s", key, val)
	}

	t.Log("\n=== Testing metrics (optional) ===")

	metrics, err := getMetrics(metricsURL)
	if err != nil {
		t.Logf("Warning: Metrics endpoint not available: %v", err)
	} else {
		t.Logf("Metrics sample:\n%s", metrics[:min(500, len(metrics))])
	}

	// ConfigMap cleanup is handled by helpers.SetRules t.Cleanup.
	t.Log("\n=== Cloud E2E test completed successfully! ===")
}

func TestCloudE2E_TwoUsersRateLimit(t *testing.T) {
	t.Log("=== Setting up Two Users Rate Limit Test ===")

	ctx := context.Background()

	// Port-forwards (gateway, operator, redis) are provided by the Makefile
	// via port-forward.sh --profile=cloud --start.

	kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err, "building k8s rest config")
	clientset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err, "creating k8s clientset")

	gatewayURL := fmt.Sprintf("http://localhost:%s", gatewayPort)
	operatorURL := fmt.Sprintf("http://localhost:%s", operatorPort)
	goodUser := "good-user"
	badUser := "bad-user"

	t.Log("\n=== Adding rate limit rules via ConfigMap ===")

	// Both users get explicit rules at priority=100 to override any cluster-level
	// ratelimit-config rules (typically priority≤50). A catch-all at low priority
	// is avoided because cluster rules would win and make assertions non-deterministic.
	configYAML := `
domain: auth_limit
separator: "|"
descriptors:
  - key: user_id
    value_regex: "bad-user"
    rate_limit:
      unit: minute
      requests_per_unit: 30
    algorithm: fixed_window
    priority: 100
  - key: user_id
    value_regex: "good-user"
    rate_limit:
      unit: minute
      requests_per_unit: 1000
    algorithm: fixed_window
    priority: 100
`
	helpers.SetRules(ctx, t, clientset, namespace, "cloud-e2e-two-users", configYAML)

	// Force immediate reconciliation.
	_, _ = http.Post(operatorURL+"/api/v1/config/reload", "application/json", nil)
	time.Sleep(3 * time.Second)
	t.Log("Rate limit rules added (bad-user=30/min, good-user=1000/min)")

	t.Log("\n=== Verifying rule applied ===")

	checkResp, err := http.Post(operatorURL+"/api/v1/ratelimit/check", "application/json",
		bytes.NewBuffer([]byte(`{"components":{"path":"/test","user_id":"bad-user"}}`)))
	if err == nil {
		var result map[string]interface{}
		json.NewDecoder(checkResp.Body).Decode(&result)
		t.Logf("Bad user limit: %v", result["limit"])
		checkResp.Body.Close()
	}

	checkResp, err = http.Post(operatorURL+"/api/v1/ratelimit/check", "application/json",
		bytes.NewBuffer([]byte(`{"components":{"path":"/test","user_id":"good-user"}}`)))
	if err == nil {
		var result map[string]interface{}
		json.NewDecoder(checkResp.Body).Decode(&result)
		t.Logf("Good user limit: %v", result["limit"])
		checkResp.Body.Close()
	}

	t.Log("\n=== Testing GOOD user (should have high limit) ===")

	goodSuccess := 0
	goodLimited := 0
	requests := 50

	for i := 0; i < requests; i++ {
		statusCode, err := sendGatewayRequest(gatewayURL, "/test", goodUser)
		require.NoError(t, err)
		if statusCode == 200 {
			goodSuccess++
		} else if statusCode == 429 {
			goodLimited++
		}
		if (i+1)%10 == 0 {
			t.Logf("Good user progress: %d/%d requests", i+1, requests)
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Logf("\nGood user results: %d OK, %d Rate Limited out of %d requests", goodSuccess, goodLimited, requests)

	// Good user should have very few rate limits (less than 5)
	assert.LessOrEqual(t, goodLimited, 5, "Good user should rarely be rate limited (got %d)", goodLimited)

	t.Log("\n=== Testing BAD user (rate limited after ~30 requests) ===")

	badSuccess := 0
	badLimited := 0

	for i := 0; i < requests; i++ {
		statusCode, err := sendGatewayRequest(gatewayURL, "/test", badUser)
		require.NoError(t, err)
		if statusCode == 200 {
			badSuccess++
		} else if statusCode == 429 {
			badLimited++
			if badLimited == 1 {
				t.Logf("Bad user rate limited started at request #%d", i+1)
			}
		}

		if (i+1)%10 == 0 {
			t.Logf("Bad user progress: %d/%d requests", i+1, requests)
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Logf("\nBad user results: %d OK, %d Rate Limited out of %d requests", badSuccess, badLimited, requests)

	// Expected: ~30 OK, ~20 Limited
	expectedLimited := requests - 30
	if expectedLimited < 0 {
		expectedLimited = 0
	}

	t.Logf("Expected bad user limited requests: ~%d (50 total - 30 limit)", expectedLimited)
	t.Logf("Actual bad user limited requests: %d", badLimited)

	// Allow some tolerance (within 10 requests due to network delays)
	assert.InDelta(t, expectedLimited, badLimited, 10,
		"Bad user should be rate limited around %d requests (got %d)", expectedLimited, badLimited)

	t.Log("\n=== Isolation Check ===")
	if goodLimited <= 5 && badLimited > 15 {
		t.Log("PASS: Rate limits are properly isolated per user")
		t.Logf("   Good user: %d limited, Bad user: %d limited", goodLimited, badLimited)
	} else if goodLimited > 5 {
		t.Log("WARNING: Good user was also rate limited - isolation may be compromised")
		t.Logf("   Good user: %d limited, Bad user: %d limited", goodLimited, badLimited)
	}

	t.Log("\n=== Getting violating users ===")

	violatingUsers, err := getViolatingUsers(operatorURL)
	require.NoError(t, err)
	t.Logf("Violating users: %v", violatingUsers)

	t.Log("\n=== Resetting rate limits for bad user ===")

	err = resetUserRateLimit(operatorURL, badUser)
	require.NoError(t, err)
	t.Log("Rate limits reset for bad user")

	time.Sleep(2 * time.Second)

	t.Log("\n=== Verifying reset ===")

	resetSuccess := 0
	for i := 0; i < 10; i++ {
		statusCode, err := sendGatewayRequest(gatewayURL, "/test", badUser)
		require.NoError(t, err)
		if statusCode == 200 {
			resetSuccess++
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Bad user after reset: %d/10 requests successful", resetSuccess)
	assert.Greater(t, resetSuccess, 8, "After reset, bad user should be able to make requests again")

	// ConfigMap cleanup is handled by helpers.SetRules t.Cleanup.
	t.Log("\n=== Two Users Rate Limit Test completed successfully! ===")
}

// Helper functions

func sendGatewayRequest(gatewayURL, endpoint, userID string) (int, error) {
	url := gatewayURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("x-user-id", userID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func getViolatingUsersRaw(apiURL string) (string, error) {
	resp, err := http.Get(apiURL + "/api/v1/users/violating")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
		return string(body), nil
	}

	return prettyJSON.String(), nil
}

func getViolatingUsers(apiURL string) ([]string, error) {
	resp, err := http.Get(apiURL + "/api/v1/users/violating")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		ViolatingUsers []struct {
			UserID string `json:"user_id"`
		} `json:"violating_users"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	users := make([]string, len(result.ViolatingUsers))
	for i, u := range result.ViolatingUsers {
		users[i] = u.UserID
	}
	return users, nil
}

func resetUserRateLimit(apiURL, userID string) error {
	url := fmt.Sprintf("%s/api/v1/users/%s/reset", apiURL, userID)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reset failed with status: %d", resp.StatusCode)
	}
	return nil
}

func cleanupRedisKeys(userID string) {
	cmd := exec.Command("kubectl", "exec", "-n", namespace, "deployment/redis", "--",
		"redis-cli", "KEYS", "*"+userID+"*")
	output, _ := cmd.Output()
	keys := strings.Split(string(output), "\n")

	for _, key := range keys {
		if key != "" {
			delCmd := exec.Command("kubectl", "exec", "-n", namespace, "deployment/redis", "--",
				"redis-cli", "DEL", key)
			delCmd.Run()
		}
	}
}

func getMetrics(apiURL string) (string, error) {
	metricsURL := fmt.Sprintf("http://localhost:%s/metrics", metricsPort)
	resp, err := http.Get(metricsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(body), "\n")
	var ourMetrics []string

	for _, line := range lines {
		if strings.Contains(line, "ratelimit_") {
			ourMetrics = append(ourMetrics, line)
		}
	}

	if len(ourMetrics) == 0 {
		return "No rate limit metrics found", nil
	}

	return "=== RateLimit Metrics ===\n" +
		strings.Join(ourMetrics, "\n") +
		"\n=== End Metrics ===", nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
