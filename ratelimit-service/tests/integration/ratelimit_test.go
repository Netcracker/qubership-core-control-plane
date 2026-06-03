//go:build integration
// +build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ratelimit-service/pkg/ratelimit"
	"ratelimit-service/tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := helpers.NewEnv(t)

	rule := &ratelimit.Rule{
		Name:      "test",
		Pattern:   ".*user_id=test.*",
		Limit:     2,
		Window:    time.Minute,
		Algorithm: ratelimit.FixedWindow,
	}
	err := env.Manager.AddRule(rule)
	require.NoError(t, err)

	ctx := context.Background()
	components := map[string]string{
		"user_id": "test",
		"path":    "/api",
	}

	// First request should be allowed
	result, err := env.Manager.CheckWithComponents(ctx, components, "|")
	require.NoError(t, err)
	assert.True(t, result["allowed"].(bool))
	assert.Equal(t, 2, result["limit"])

	// Second request should be allowed
	result, err = env.Manager.CheckWithComponents(ctx, components, "|")
	require.NoError(t, err)
	assert.True(t, result["allowed"].(bool))

	// Third request should be rejected
	result, err = env.Manager.CheckWithComponents(ctx, components, "|")
	require.NoError(t, err)
	assert.False(t, result["allowed"].(bool))
}

func TestIntegration_GetViolatingUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := helpers.NewEnv(t)

	rule := &ratelimit.Rule{
		Name:      "test",
		Pattern:   ".*user_id=.*",
		Limit:     1,
		Window:    time.Minute,
		Algorithm: ratelimit.FixedWindow,
	}
	env.Manager.AddRule(rule)

	ctx := context.Background()

	// Create rate limited user
	components1 := map[string]string{"user_id": "user1", "path": "/api"}
	env.Manager.CheckWithComponents(ctx, components1, "|") // 1st request
	env.Manager.CheckWithComponents(ctx, components1, "|") // 2nd request — rate limited

	// Create another user within limit
	components2 := map[string]string{"user_id": "user2", "path": "/api"}
	env.Manager.CheckWithComponents(ctx, components2, "|") // 1st request only

	// Get violating users
	users, err := env.Redis.GetViolatingUsers(ctx)
	require.NoError(t, err)

	// Should have at least one violating user
	assert.GreaterOrEqual(t, len(users), 1)
}
