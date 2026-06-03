//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"ratelimit-service/pkg/utils"
	"ratelimit-service/tests/e2e/setup"
	"ratelimit-service/tests/helpers"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	operatorPort = "8083"
)

func TestE2E_RateLimitThroughOperatorAPI(t *testing.T) {
	ctx := context.Background()

	t.Log("=== Setting up test environment ===")

	kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")

	// Build k8s clientset for ConfigMap management.
	// Redis port-forward (6379) is provided by the Makefile via port-forward.sh --profile=local --start.
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err, "building k8s rest config from kubeconfig")
	clientset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err, "creating k8s clientset")

	// Namespace the operator will watch (must match NAMESPACE env var read by controller).
	testNs := utils.GetEnv("NAMESPACE", "core-1-core")
	os.Setenv("NAMESPACE", testNs)

	// Start local operator (connects to Redis at localhost:6379 — forwarded by Makefile).
	operator, err := setup.NewLocalOperator(kubeconfig)
	require.NoError(t, err)
	require.NoError(t, operator.Start(ctx, operatorPort))
	defer operator.Stop()

	time.Sleep(3 * time.Second)

	operatorURL := "http://localhost:" + operatorPort
	userID := "e2e-test-user"

	t.Log("\n=== Cleaning Redis before test ===")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	defer rdb.Close()

	keys, _ := rdb.Keys(ctx, "*"+userID+"*").Result()
	for _, key := range keys {
		rdb.Del(ctx, key)
	}
	t.Logf("Cleaned %d keys from Redis", len(keys))

	t.Log("\n=== Creating rules via ConfigMap ===")

	// Two explicit per-user rules, both at priority=100, so they always beat any
	// cluster-level ratelimit-config rules (typically priority≤50).
	// A catch-all fallback at low priority is intentionally avoided: its effective
	// limit would be determined by which higher-priority cluster rule matches first,
	// making the assertion non-deterministic across environments.
	configYAML := `
domain: auth_limit
separator: "|"
descriptors:
  - key: user_id
    value_regex: "e2e-test-user"
    rate_limit:
      unit: minute
      requests_per_unit: 2
    algorithm: fixed_window
    priority: 100
  - key: user_id
    value_regex: "other-test-user"
    rate_limit:
      unit: minute
      requests_per_unit: 100
    algorithm: fixed_window
    priority: 100
`
	helpers.SetRules(ctx, t, clientset, testNs, "e2e-rate-limit-test", configYAML)

	// Force immediate reconciliation rather than waiting for watcher debounce.
	_, _ = http.Post(operatorURL+"/api/v1/config/reload", "application/json", nil)
	time.Sleep(500 * time.Millisecond)

	t.Log("\n=== Testing rate limit for test user (limit=2/s) ===")

	for i := 0; i < 2; i++ {
		allowed, limit, remaining, err := checkRateLimit(operatorURL, "/test", userID)
		require.NoError(t, err)
		t.Logf("Request %d (test user): allowed=%v, limit=%d, remaining=%d", i+1, allowed, limit, remaining)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
		assert.Equal(t, 2, limit, "Limit should be 2 for test user")
	}

	allowed, _, remaining, err := checkRateLimit(operatorURL, "/test", userID)
	require.NoError(t, err)
	t.Logf("Request 3 (test user): allowed=%v, remaining=%d", allowed, remaining)
	assert.False(t, allowed, "Third request for test user should be rejected")

	t.Log("\n=== Testing other user with fallback rule ===")

	otherUserID := "other-test-user"
	allowed2, limit2, _, err := checkRateLimit(operatorURL, "/test", otherUserID)
	require.NoError(t, err)
	t.Logf("Other user: allowed=%v, limit=%d", allowed2, limit2)
	assert.True(t, allowed2, "Other user should be allowed (rate limits are per-user, not shared)")
	assert.Equal(t, 100, limit2, "Other user should have limit 100 from their dedicated rule (priority=100)")

	t.Log("\n=== Testing rate limit reset ===")

	err = resetUserRateLimit(operatorURL, userID)
	require.NoError(t, err)
	t.Log("Rate limits reset for test user")
	time.Sleep(1 * time.Second)

	allowed, _, _, err = checkRateLimit(operatorURL, "/test", userID)
	require.NoError(t, err)
	t.Logf("After reset: allowed=%v", allowed)
	assert.True(t, allowed, "Request after reset should be allowed")

	// ConfigMap cleanup is handled by helpers.SetRules t.Cleanup.
	t.Log("\n=== Cleaning up Redis ===")
	finalKeys, _ := rdb.Keys(ctx, "*").Result()
	for _, key := range finalKeys {
		if bytes.Contains([]byte(key), []byte(userID)) ||
			bytes.Contains([]byte(key), []byte(otherUserID)) {
			rdb.Del(ctx, key)
			t.Logf("Deleted key: %s", key)
		}
	}

	t.Log("\n=== E2E test completed successfully! ===")
}

// Helper functions

func checkRateLimit(apiURL, path, userID string) (allowed bool, limit int, remaining int, err error) {
	reqBody := map[string]interface{}{
		"components": map[string]string{
			"path":    path,
			"user_id": userID,
		},
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(apiURL+"/api/v1/ratelimit/check", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return false, 0, 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, 0, 0, err
	}

	allowed, ok := result["allowed"].(bool)
	if !ok {
		return false, 0, 0, fmt.Errorf("invalid response format")
	}

	if l, ok := result["limit"].(float64); ok {
		limit = int(l)
	}
	if r, ok := result["remaining"].(float64); ok {
		remaining = int(r)
	}

	return allowed, limit, remaining, nil
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

// TestHelperFunctions tests helper utilities that need no external dependencies.
func TestHelperFunctions(t *testing.T) {
	t.Run("ValidatePattern", func(t *testing.T) {
		tests := []struct {
			pattern string
			valid   bool
		}{
			{".*user_id=test.*", true},
			{"user_id=test", true},
			{"^.*user_id=test.*$", true},
			{"*user_id=test*", false},
			{"[invalid", false},
			{"?invalid", false},
		}

		for _, tt := range tests {
			_, err := regexp.Compile(tt.pattern)
			if tt.valid {
				assert.NoError(t, err, "Pattern %s should be valid", tt.pattern)
			} else {
				assert.Error(t, err, "Pattern %s should be invalid", tt.pattern)
			}
		}
	})

	t.Run("BuildKey", func(t *testing.T) {
		components := map[string]string{
			"user_id": "test",
			"path":    "/api",
		}
		key1 := buildKeyForTest(components, "|")
		key2 := buildKeyForTest(components, "|")

		assert.Contains(t, key1, "user_id=test")
		assert.Contains(t, key1, "path=/api")
		assert.Contains(t, key1, "|")
		assert.Equal(t, len(key1), len(key2))

		key3 := buildKeyForTest(components, "&")
		assert.Contains(t, key3, "&")
		assert.NotContains(t, key3, "|")
	})

	t.Run("BuildKeyStable", func(t *testing.T) {
		components := map[string]string{
			"z_key": "last",
			"a_key": "first",
			"m_key": "middle",
		}
		key := buildKeyForTest(components, "|")
		assert.Contains(t, key, "a_key=first")
		assert.Contains(t, key, "m_key=middle")
		assert.Contains(t, key, "z_key=last")
	})
}

func buildKeyForTest(components map[string]string, separator string) string {
	if components == nil {
		return ""
	}
	result := ""
	first := true
	for k, v := range components {
		if !first {
			result += separator
		}
		result += k + "=" + v
		first = false
	}
	return result
}
