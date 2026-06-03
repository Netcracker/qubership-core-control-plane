package metrics

import "github.com/prometheus/client_golang/prometheus"

type MetricsCollector interface {
    // Rate limit metrics
    UpdateRateLimitMetrics(violatingCount int, activeLimitsCount int)
    UpdateViolatingUsers(count int)
    UpdateActiveLimits(count int)
    RecordRateLimitCheck(key string, allowed bool, limit int)
    RecordRateLimitReset(key string)
    RecordRateLimitRequest(path string, allowed bool)

    // API metrics
    RecordAPIRequest(endpoint, method, status string, duration float64)

    // Redis metrics
    RecordRedisOperation(operation, status string, duration float64)
    
    // NEW: Redis metrics for monitoring
    UpdateRedisKeysCount(count int)
    UpdateRedisMemoryUsage(bytes int64)
    UpdateRedisConnectedClients(count int)
    RecordRedisLatency(operation string, latencyMs float64)
    UpdateRedisHitRate(rate float64)

    // Config metrics
    RecordConfigReload(success bool)

    GetRegistry() *prometheus.Registry

    RecordRateLimit(key string, allowed bool, current int, limit int)
}