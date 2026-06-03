//go:build integration
// +build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ratelimit-service/pkg/controller"
	"ratelimit-service/pkg/ratelimit"
	"ratelimit-service/tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIntegration_ControllerWithRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := helpers.NewEnv(t)

	// Create fake k8s client
	clientset := fake.NewSimpleClientset()

	// Create controller
	ctrl := controller.NewConfigMapController(clientset, env.Redis, env.Manager)
	assert.NotNil(t, ctrl)

	ctx := context.Background()

	// Add rule directly via manager
	rule := &ratelimit.Rule{
		Name:      "test",
		Pattern:   ".*user_id=test.*",
		Limit:     2,
		Window:    time.Minute,
		Algorithm: ratelimit.FixedWindow,
	}
	env.Manager.AddRule(rule)

	// Check rate limit
	allowed, _, err := env.Manager.Check(ctx, "user_id=test")
	require.NoError(t, err)
	assert.True(t, allowed)
}
