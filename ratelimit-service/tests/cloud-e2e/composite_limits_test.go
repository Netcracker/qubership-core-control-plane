//go:build cloud_e2e_layered
// +build cloud_e2e_layered

package cloud_e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"ratelimit-service/pkg/utils"
	"ratelimit-service/tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TestCloudE2E_CompositeLimits exercises the layered (outer + inner)
// rate-limit scheme through the Istio Gateway with real Redis. The test:
//
//  1. Deploys a ConfigMap with outer=path 1000/min, inner=(path,user) 10/min.
//  2. Sends 10 requests from each of 100 unique user_ids → all 1000 should pass.
//  3. 101st request from user_id "layered-user-000" (who already exhausted her 10) → 429.
//  4. First request from user_id "layered-eve" (101st unique user) → 429, because the
//     outer 1000/min cap is now exhausted.
//  5. Waits 65 seconds for the minute window to roll over, then verifies the
//     first request from "layered-user-000" passes again.
//
// This test runs only under build tag cloud_e2e_layered and is excluded from
// the regular cloud_e2e suite due to its >1-minute runtime.
func TestCloudE2E_CompositeLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running composite-limits test in -short mode")
	}

	ctx := context.Background()
	namespace := utils.GetEnv("NAMESPACE", "core-1-core")
	gatewayURL := fmt.Sprintf("http://localhost:%s", utils.GetEnv("GATEWAY_PORT", "8080"))
	operatorURL := fmt.Sprintf("http://localhost:%s", utils.GetEnv("OPERATOR_PORT", "8082"))

	// Build k8s client from kubeconfig.
	kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	configYAML := `
domain: auth_limit
separator: "|"
descriptors:
  - key: path
    value: /api/v1/orders
    rate_limit:
      unit: minute
      requests_per_unit: 1000
    algorithm: fixed_window
    priority: 50
    descriptors:
      - key: user_id
        rate_limit:
          unit: minute
          requests_per_unit: 10
        algorithm: fixed_window
        priority: 100
`
	helpers.SetRules(ctx, t, clientset, namespace, "cloud-e2e-composite", configYAML)

	// Force immediate reconciliation.
	_, err = http.Post(operatorURL+"/api/v1/config/reload", "application/json", nil)
	require.NoError(t, err)
	time.Sleep(3 * time.Second)

	// Step 1: 100 users × 10 requests = 1000 allowed (outer cap exactly).
	t.Log("Step 1: 100 users × 10 requests = 1000 allowed")
	allowed := 0
	denied := 0
	for u := 0; u < 100; u++ {
		userID := fmt.Sprintf("layered-user-%03d", u)
		for i := 0; i < 10; i++ {
			code, err := sendOrdersRequest(gatewayURL, userID)
			require.NoError(t, err)
			if code == 200 {
				allowed++
			} else {
				denied++
			}
		}
	}
	t.Logf("Step 1 result: allowed=%d, denied=%d", allowed, denied)
	// Tolerate small skew due to network/timing.
	assert.InDelta(t, 1000, allowed, 20,
		"step 1 should allow ~1000 requests (got %d)", allowed)

	// Step 2: any of the 100 users hits their personal cap → 429.
	t.Log("Step 2: layered-user-000 (already exhausted) → 429 by inner rule")
	code, err := sendOrdersRequest(gatewayURL, "layered-user-000")
	require.NoError(t, err)
	assert.Equal(t, 429, code, "layered-user-000's 11th request should be denied (inner rule)")

	// Step 3: a brand-new user → 429 by outer rule (global cap is exhausted).
	t.Log("Step 3: layered-eve (101st unique user) → 429 by outer rule")
	code, err = sendOrdersRequest(gatewayURL, "layered-eve")
	require.NoError(t, err)
	assert.Equal(t, 429, code, "layered-eve's first request should be denied (outer 1000/min cap is full)")

	// Step 4: wait for the minute window to roll over.
	t.Log("Step 4: sleeping 65s for window rollover")
	time.Sleep(65 * time.Second)

	code, err = sendOrdersRequest(gatewayURL, "layered-user-000")
	require.NoError(t, err)
	assert.Equal(t, 200, code, "after window rollover, layered-user-000's request should pass again")

	// ConfigMap cleanup is handled by helpers.SetRules t.Cleanup.
}

// sendOrdersRequest issues a GET to /api/v1/orders with the given user_id
// header and returns the HTTP status code.
func sendOrdersRequest(gatewayURL, userID string) (int, error) {
	req, err := http.NewRequest("GET", gatewayURL+"/api/v1/orders", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("x-user-id", userID)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// Drain body so connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}
