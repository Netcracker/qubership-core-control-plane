//go:build e2e
// +build e2e

// NOTE: This test requires a real Redis connection (TEST_REDIS_ADDR) — miniredis does not
// support the Redis INFO command set that metrics.NewMetricsCollectorService relies on.
// The test skips automatically when Redis is unavailable (port-forward managed by Makefile).
package e2e

import (
	"context"
	"testing"
	"time"

	"ratelimit-service/pkg/metrics"
	"ratelimit-service/pkg/ratelimit"
	"ratelimit-service/pkg/utils"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_MetricsWithRealRedis(t *testing.T) {
	namespace := utils.GetEnv("NAMESPACE", "core-1-core")
	redisAddr := utils.GetEnv("TEST_REDIS_ADDR", "localhost:6379")

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   15,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s. Run: kubectl port-forward -n %s service/redis 6379:6379", redisAddr, namespace)
		return
	}
	defer rdb.Close()

	rdb.FlushDB(ctx)
	defer rdb.FlushDB(ctx)

	redisClient, _ := ratelimit.NewRedisClient(redisAddr, "", 15)
	collector := metrics.NewDefaultMetricsCollector()

	testKeys := map[string]int{
		"path=/test|user_id=alice":   45,
		"user_id=bob":                75,
		"path=/test|user_id=charlie": 1,
	}

	for key, value := range testKeys {
		err := rdb.Set(ctx, key, value, time.Minute).Err()
		require.NoError(t, err)
	}

	service := metrics.NewMetricsCollectorService(redisClient, collector, 1*time.Second)
	go service.Start(ctx)

	time.Sleep(2 * time.Second)

	metricFamilies, err := collector.GetRegistry().Gather()
	require.NoError(t, err)

	foundViolating := false
	for _, mf := range metricFamilies {
		if *mf.Name == "ratelimit_violating_users_total" {
			foundViolating = true
			value := mf.GetMetric()[0].GetGauge().GetValue()
			t.Logf("Violating users: %f", value)
			break
		}
	}

	assert.True(t, foundViolating, "Violating users metric should exist")

	service.Stop()
	t.Log("E2E metrics collector test completed")
}
