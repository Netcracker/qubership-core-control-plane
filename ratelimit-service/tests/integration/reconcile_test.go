//go:build integration
// +build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"ratelimit-service/pkg/controller"
	"ratelimit-service/pkg/ratelimit"
	"ratelimit-service/tests/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestIntegration_ReconcileLoop verifies the controller's watcher correctly
// reacts to Create / Update / Delete events on ConfigMaps labelled
// rate-limit-config=true.
//
// Acceptance:
//   - Create CM → rules appear in manager within ~3s.
//   - Update CM (change limit) → updated limit is visible in manager.
//   - Delete CM → all rules from that CM are gone.
//
// Uses fake.Clientset — no real Kubernetes cluster required.
func TestIntegration_ReconcileLoop(t *testing.T) {
	const (
		namespace = "test-reconcile"
		cmName    = "rl-reconcile-cm"
	)

	// NAMESPACE is read by NewConfigMapController at construction time.
	t.Setenv("NAMESPACE", namespace)

	env := helpers.NewEnv(t)
	clientset := fake.NewSimpleClientset()

	ctrl := controller.NewConfigMapController(clientset, env.Redis, env.Manager)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // cancels watch goroutine on test exit

	go ctrl.Run(ctx)

	// Give the controller time to register its watch before we send events.
	time.Sleep(200 * time.Millisecond)

	// ── Step 1: create the ConfigMap, expect rules to appear. ───────────────
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: namespace,
			Labels:    map[string]string{"rate-limit-config": "true"},
		},
		Data: map[string]string{
			"config.yaml": `
domain: auth_limit
separator: "|"
descriptors:
  - key: path
    value: /v1/foo
    rate_limit:
      unit: minute
      requests_per_unit: 100
    algorithm: fixed_window
    priority: 50
`,
		},
	}
	_, err := clientset.CoreV1().ConfigMaps(namespace).
		Create(ctx, cm, metav1.CreateOptions{})
	require.NoError(t, err)

	requireRuleEventually(t, env.Manager, cmName, 100, 3*time.Second)

	// ── Step 2: update the ConfigMap — limit 100 → 200. ────────────────────
	cm.Data["config.yaml"] = `
domain: auth_limit
separator: "|"
descriptors:
  - key: path
    value: /v1/foo
    rate_limit:
      unit: minute
      requests_per_unit: 200
    algorithm: fixed_window
    priority: 50
`
	_, err = clientset.CoreV1().ConfigMaps(namespace).
		Update(ctx, cm, metav1.UpdateOptions{})
	require.NoError(t, err)

	requireRuleEventually(t, env.Manager, cmName, 200, 3*time.Second)
	assertRuleCount(t, env.Manager, cmName, 1, "after update should still have exactly 1 rule")

	// ── Step 3: delete the ConfigMap, expect all its rules to disappear. ────
	err = clientset.CoreV1().ConfigMaps(namespace).
		Delete(ctx, cmName, metav1.DeleteOptions{})
	require.NoError(t, err)

	assertRuleCountEventually(t, env.Manager, cmName, 0, 3*time.Second,
		"after delete, all rules from this CM should be removed")
}

// ── helpers ─────────────────────────────────────────────────────────────────

func rulesForCM(mgr *ratelimit.RateLimitManager, cmName string) []*ratelimit.Rule {
	var out []*ratelimit.Rule
	for _, r := range mgr.GetAllRules() {
		if strings.HasPrefix(r.Name, cmName+"/") {
			out = append(out, r)
		}
	}
	return out
}

func requireRuleEventually(
	t *testing.T,
	mgr *ratelimit.RateLimitManager,
	cmName string,
	wantLimit int,
	timeout time.Duration,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, r := range rulesForCM(mgr, cmName) {
			if r.Limit == wantLimit {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("rule with limit=%d for CM %q not observed within %v", wantLimit, cmName, timeout)
}

func assertRuleCount(
	t *testing.T,
	mgr *ratelimit.RateLimitManager,
	cmName string,
	want int,
	msg string,
) {
	t.Helper()
	got := len(rulesForCM(mgr, cmName))
	assert.Equal(t, want, got, msg)
}

func assertRuleCountEventually(
	t *testing.T,
	mgr *ratelimit.RateLimitManager,
	cmName string,
	want int,
	timeout time.Duration,
	msg string,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(rulesForCM(mgr, cmName)) == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s — got %d rules, want %d after %v",
		msg, len(rulesForCM(mgr, cmName)), want, timeout)
}
