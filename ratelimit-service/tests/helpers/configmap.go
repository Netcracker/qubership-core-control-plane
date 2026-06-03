package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SetRules creates a ConfigMap with the rate-limit-config=true label and the
// provided YAML payload in data["config.yaml"]. It registers a t.Cleanup that
// deletes the ConfigMap.
//
// configMapName must be unique per test to avoid collisions when tests run
// in parallel; callers typically use t.Name() (with characters sanitised).
//
// The function blocks for up to 10s waiting for the ConfigMap to be observable
// via the API server. It does NOT wait for the ratelimit-service controller to
// reconcile — callers that need that should poll GET /api/v1/ratelimit/rules
// or invoke POST /api/v1/config/reload.
func SetRules(
	ctx context.Context,
	t *testing.T,
	clientset kubernetes.Interface,
	namespace string,
	configMapName string,
	configYAML string,
) {
	t.Helper()

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"rate-limit-config": "true",
				"app":               "ratelimit-test",
				"test-name":         sanitiseLabel(t.Name()),
			},
		},
		Data: map[string]string{
			"config.yaml": configYAML,
		},
	}

	_, err := clientset.CoreV1().ConfigMaps(namespace).
		Create(ctx, cm, metav1.CreateOptions{})
	require.NoError(t, err, "creating ConfigMap %s/%s", namespace, configMapName)

	t.Cleanup(func() {
		_ = clientset.CoreV1().ConfigMaps(namespace).
			Delete(context.Background(), configMapName, metav1.DeleteOptions{})
	})

	// Best-effort: wait until the API server returns our ConfigMap on GET.
	// This guards against the (rare) read-after-write race on watch caches.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, getErr := clientset.CoreV1().ConfigMaps(namespace).
			Get(ctx, configMapName, metav1.GetOptions{})
		if getErr == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("ConfigMap %s/%s not visible after 10s", namespace, configMapName)
}

// sanitiseLabel strips characters not allowed in k8s label values.
func sanitiseLabel(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_' || r == '.':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	const maxLen = 63
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	return string(out)
}

// MustBuildConfigYAML returns the contents of a ConfigMap data["config.yaml"]
// assembled from a domain, separator, and a pre-formatted descriptors block.
// Useful when a test wants to build rules programmatically rather than loading
// a fixture file.
func MustBuildConfigYAML(t *testing.T, domain, separator string, descriptors string) string {
	t.Helper()

	// A small string-builder is sufficient here — full YAML marshalling
	// would require importing the parser's Config types, which would
	// couple helpers to pkg/ratelimit/config.go's internal layout.
	return fmt.Sprintf("domain: %s\nseparator: %q\ndescriptors:\n%s\n",
		domain, separator, descriptors)
}
