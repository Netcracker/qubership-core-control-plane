//go:build integration
// +build integration

package integration_test

import (
	"context"
	"testing"

	"ratelimit-service/pkg/controller"
	"ratelimit-service/tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_LayeredLimits exercises the composite (outer + inner)
// rate-limit scheme defined in tests/fixtures/ratelimit-config-layered.yaml.
//
// Outer rule: path=/api/v1/orders -> 1000 req/min, priority 50.
// Inner rule: path=/api/v1/orders AND user_id=<any> -> 10 req/min, priority 100.
//
// The tests below verify:
//   - per-user counters are isolated (alice and bob are independent);
//   - the inner (higher-priority) rule wins when both match the same key;
//   - the outer rule applies to anonymous (no user_id) traffic;
//   - rules and counters survive across calls within the same window.
func TestIntegration_LayeredLimits(t *testing.T) {
	ctx := context.Background()
	env := helpers.NewEnv(t)

	// Load fixture and register all rules through the public parser, the
	// same path the production controller uses.
	yamlData := helpers.LoadFixture(t, "ratelimit-config-layered.yaml")
	rules, err := controller.ParseConfigYAML(yamlData, "layered-test-cm")
	require.NoError(t, err)
	require.Len(t, rules, 2, "fixture should produce exactly 2 rules")

	for _, r := range rules {
		require.NoError(t, env.Manager.AddRule(r))
	}

	t.Run("Alice — inner rule wins, 10 req/min", func(t *testing.T) {
		components := map[string]string{
			"path":    "/api/v1/orders",
			"user_id": "alice",
		}

		// First 10 requests should pass under the inner rule (limit 10).
		for i := 1; i <= 10; i++ {
			res, err := env.Manager.CheckWithComponents(ctx, components, "|")
			require.NoError(t, err)
			assert.True(t, res["allowed"].(bool),
				"request %d for alice should be allowed", i)
			// The inner rule (priority 100) should be the matched one.
			assert.Equal(t, 10, res["limit"], "alice should hit the inner (10/min) rule")
		}

		// 11th — denied.
		res, err := env.Manager.CheckWithComponents(ctx, components, "|")
		require.NoError(t, err)
		assert.False(t, res["allowed"].(bool), "alice's 11th request should be denied")
	})

	t.Run("Bob — independent counter from Alice", func(t *testing.T) {
		components := map[string]string{
			"path":    "/api/v1/orders",
			"user_id": "bob",
		}
		// Alice has already exhausted her quota; bob should still have full 10.
		for i := 1; i <= 10; i++ {
			res, err := env.Manager.CheckWithComponents(ctx, components, "|")
			require.NoError(t, err)
			assert.True(t, res["allowed"].(bool),
				"request %d for bob should be allowed (independent counter)", i)
		}
		res, err := env.Manager.CheckWithComponents(ctx, components, "|")
		require.NoError(t, err)
		assert.False(t, res["allowed"].(bool), "bob's 11th request should be denied")
	})

	t.Run("Anonymous request — outer rule, 1000 req/min", func(t *testing.T) {
		components := map[string]string{
			"path": "/api/v1/orders",
		}
		// We won't exhaust 1000 in a test; just verify the matched limit
		// is the outer one, not the inner.
		res, err := env.Manager.CheckWithComponents(ctx, components, "|")
		require.NoError(t, err)
		assert.True(t, res["allowed"].(bool), "anonymous request should be allowed")
		assert.Equal(t, 1000, res["limit"],
			"anonymous (no user_id) request should match the outer (1000/min) rule")
	})

	t.Run("Different path — neither rule matches", func(t *testing.T) {
		components := map[string]string{
			"path":    "/api/v1/products",
			"user_id": "alice",
		}
		res, err := env.Manager.CheckWithComponents(ctx, components, "|")
		require.NoError(t, err)
		assert.True(t, res["allowed"].(bool),
			"request to a different path should be allowed (no rule matches)")
		// When no rule matches, manager returns limit=0.
		assert.Equal(t, 0, res["limit"])
	})
}

// TestIntegration_LayeredPriority_Misconfiguration documents that priority
// ordering matters: if the inner (more-specific) rule has lower priority
// than the outer one, the outer wins for path+user_id requests, and the
// layered semantic breaks silently. This test is a guard against future
// regressions in priority handling.
func TestIntegration_LayeredPriority_Misconfiguration(t *testing.T) {
	ctx := context.Background()
	env := helpers.NewEnv(t)

	// Same shape as the fixture, but with inverted priorities.
	misconfigured := `
domain: auth_limit
separator: "|"
descriptors:
  - key: path
    value: /api/v1/orders
    rate_limit:
      unit: minute
      requests_per_unit: 1000
    algorithm: fixed_window
    priority: 100        # outer is now HIGHER priority — wrong
    descriptors:
      - key: user_id
        rate_limit:
          unit: minute
          requests_per_unit: 10
        algorithm: fixed_window
        priority: 50     # inner is LOWER priority — wrong
`
	rules, err := controller.ParseConfigYAML(misconfigured, "misconfigured-cm")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	for _, r := range rules {
		require.NoError(t, env.Manager.AddRule(r))
	}

	res, err := env.Manager.CheckWithComponents(ctx, map[string]string{
		"path":    "/api/v1/orders",
		"user_id": "alice",
	}, "|")
	require.NoError(t, err)

	// With inverted priorities, the outer (1000/min) rule wins, NOT the inner.
	// This documents the failure mode: per-user isolation is lost.
	assert.Equal(t, 1000, res["limit"],
		"with inverted priorities, the outer rule wins — this is a known footgun")
}
