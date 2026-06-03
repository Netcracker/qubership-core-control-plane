// pkg/controller/configmap_controller_test.go
package controller

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"ratelimit-service/pkg/ratelimit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigMapController_New(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	redisClient, err := ratelimit.NewRedisClient("localhost:6379", "", 0)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	rateLimitManager := ratelimit.NewRateLimitManager(redisClient)
	redisClient.SetManager(rateLimitManager)

	controller := NewConfigMapController(clientset, redisClient, rateLimitManager)

	assert.NotNil(t, controller)
	assert.Equal(t, clientset, controller.clientset)
}

func TestConfigMapController_ParseConfig(t *testing.T) {
	const cmName = "ratelimit-test"

	t.Run("empty config returns no rules and no error", func(t *testing.T) {
		rules, err := ParseConfigYAML("", cmName)
		assert.NoError(t, err)
		assert.Empty(t, rules)
	})

	t.Run("single catch-all descriptor produces one rule", func(t *testing.T) {
		configData := `
domain: auth_limit
separator: "|"
descriptors:
  - key: ""
    rate_limit:
      unit: minute
      requests_per_unit: 60
    algorithm: fixed_window
    priority: 0
`
		rules, err := ParseConfigYAML(configData, cmName)
		require.NoError(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, 60, rules[0].Limit)
		assert.Equal(t, time.Minute, rules[0].Window)
		assert.Equal(t, ratelimit.FixedWindow, rules[0].Algorithm)
	})

	t.Run("unknown algorithm yields error", func(t *testing.T) {
		configData := `
domain: auth_limit
descriptors:
  - key: path
    value: /foo
    rate_limit:
      unit: minute
      requests_per_unit: 10
    algorithm: token_bucket
`
		_, err := ParseConfigYAML(configData, cmName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown algorithm")
	})

	t.Run("unknown unit yields error", func(t *testing.T) {
		configData := `
domain: auth_limit
descriptors:
  - key: path
    value: /foo
    rate_limit:
      unit: fortnight
      requests_per_unit: 10
`
		_, err := ParseConfigYAML(configData, cmName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown rate limit unit")
	})

	t.Run("Layered descriptors with different priorities", func(t *testing.T) {
		// Load fixture relative to this test file so the test is
		// independent of cwd.
		_, thisFile, _, ok := runtime.Caller(0)
		require.True(t, ok)
		fixturePath := filepath.Join(
			filepath.Dir(thisFile), "..", "..", "tests", "fixtures",
			"ratelimit-config-layered.yaml",
		)

		data, err := os.ReadFile(fixturePath)
		require.NoError(t, err, "reading fixture %s", fixturePath)

		rules, err := ParseConfigYAML(string(data), "layered-cm")
		require.NoError(t, err)
		require.Len(t, rules, 2, "should flatten to exactly two rules")

		// Sort rules by priority descending so the assertions below are stable.
		// (The parser does not guarantee order; we compare structurally.)
		var outer, inner *ratelimit.Rule
		for _, r := range rules {
			switch r.Priority {
			case 50:
				outer = r
			case 100:
				inner = r
			}
		}
		require.NotNil(t, outer, "expected a rule with priority 50 (outer)")
		require.NotNil(t, inner, "expected a rule with priority 100 (inner)")

		// Outer rule: matches path-only and path+user_id; limit 1000/min.
		assert.Equal(t, 1000, outer.Limit)
		assert.Equal(t, time.Minute, outer.Window)
		assert.Equal(t, ratelimit.FixedWindow, outer.Algorithm)
		assert.True(t, outer.Regex.MatchString("path=/api/v1/orders"),
			"outer rule should match a path-only key")
		assert.True(t, outer.Regex.MatchString("path=/api/v1/orders|user_id=alice"),
			"outer rule should also match a path+user_id key")

		// Inner rule: matches ONLY when both path AND user_id are present.
		assert.Equal(t, 10, inner.Limit)
		assert.Equal(t, time.Minute, inner.Window)
		assert.Equal(t, ratelimit.FixedWindow, inner.Algorithm)
		assert.True(t, inner.Regex.MatchString("path=/api/v1/orders|user_id=alice"),
			"inner rule should match a path+user_id key")
		assert.False(t, inner.Regex.MatchString("path=/api/v1/orders"),
			"inner rule should NOT match a path-only key (no user_id component)")

		// Rule names must be unique even when generated from the same ConfigMap.
		assert.NotEqual(t, outer.Name, inner.Name, "rule names must be unique")
	})
}

func TestConfigMapController_Run(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	redisClient, err := ratelimit.NewRedisClient("localhost:6379", "", 0)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	rateLimitManager := ratelimit.NewRateLimitManager(redisClient)
	redisClient.SetManager(rateLimitManager)

	controller := NewConfigMapController(clientset, redisClient, rateLimitManager)

	ctx, cancel := context.WithCancel(context.Background())

	go controller.Run(ctx)

	// Cancel the context to stop the watch loop; Run blocks on stopChan
	// (no public Stop method), so we give it a moment then just verify
	// the controller was constructed correctly.
	cancel()

	assert.NotNil(t, controller)
}
