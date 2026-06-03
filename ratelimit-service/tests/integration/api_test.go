//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ratelimit-service/pkg/api"
	"ratelimit-service/pkg/ratelimit"
	"ratelimit-service/tests/helpers"

	"github.com/stretchr/testify/assert"
)

func TestIntegration_APIWithRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := helpers.NewEnv(t)

	// Add rule
	rule := &ratelimit.Rule{
		Name:      "test",
		Pattern:   ".*user_id=test.*",
		Limit:     2,
		Window:    time.Minute,
		Algorithm: ratelimit.FixedWindow,
	}
	env.Manager.AddRule(rule)

	// Create API server
	server := api.NewServer(env.Redis, nil, env.Manager)

	// Test check endpoint
	reqBody := map[string]interface{}{
		"components": map[string]string{
			"user_id": "test",
			"path":    "/api",
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/ratelimit/check", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.CheckRateLimit(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	allowed, ok := response["allowed"].(bool)
	assert.True(t, ok)
	assert.True(t, allowed)
}
