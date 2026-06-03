package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
	"strings"
)

type DefaultMetricsCollector struct {
    registry *prometheus.Registry

    // Rate limit metrics
    totalRateLimitsActive prometheus.Gauge
    totalViolatingUsers   prometheus.Gauge
    rateLimitChecksTotal  *prometheus.CounterVec
    rateLimitResetsTotal  prometheus.Counter
    requestsTotal         *prometheus.CounterVec

    // API metrics
    apiRequestsTotal   *prometheus.CounterVec
    apiRequestDuration *prometheus.HistogramVec

    // Redis metrics
    redisOperationsTotal   *prometheus.CounterVec
    redisOperationDuration *prometheus.HistogramVec
    
    redisKeysTotal      prometheus.Gauge
    redisMemoryBytes    prometheus.Gauge
    redisConnectedClients prometheus.Gauge
    redisLatency        *prometheus.HistogramVec
    redisHitRate        prometheus.Gauge

    // Config metrics
    configReloadsTotal      prometheus.Counter
    configReloadErrorsTotal prometheus.Counter
}

func NewDefaultMetricsCollector() *DefaultMetricsCollector {
    registry := prometheus.NewRegistry()

    // Rate limit metrics
    totalRateLimitsActive := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "ratelimit_active_limits_total",
        Help: "Total number of active rate limits in Redis",
    })
    registry.MustRegister(totalRateLimitsActive)

    totalViolatingUsers := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "ratelimit_violating_users_total",
        Help: "Total number of users exceeding rate limits",
    })
    registry.MustRegister(totalViolatingUsers)

    rateLimitChecksTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "ratelimit_checks_total",
        Help: "Total number of rate limit checks",
    }, []string{"key", "result"})
    registry.MustRegister(rateLimitChecksTotal)

    rateLimitResetsTotal := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "ratelimit_resets_total",
        Help: "Total number of rate limit resets",
    })
    registry.MustRegister(rateLimitResetsTotal)

    requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "ratelimit_requests_total",
        Help: "Total number of rate limit requests",
    }, []string{"result", "path"})
    registry.MustRegister(requestsTotal)

    // API metrics
    apiRequestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "ratelimit_api_requests_total",
        Help: "Total number of API requests",
    }, []string{"endpoint", "method", "status"})
    registry.MustRegister(apiRequestsTotal)

    apiRequestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "ratelimit_api_request_duration_seconds",
        Help:    "API request duration in seconds",
        Buckets: prometheus.DefBuckets,
    }, []string{"endpoint", "method"})
    registry.MustRegister(apiRequestDuration)

    // Redis metrics
    redisOperationsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "ratelimit_redis_operations_total",
        Help: "Total number of Redis operations",
    }, []string{"operation", "status"})
    registry.MustRegister(redisOperationsTotal)

    redisOperationDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "ratelimit_redis_operation_duration_seconds",
        Help:    "Redis operation duration in seconds",
        Buckets: prometheus.DefBuckets,
    }, []string{"operation"})
    registry.MustRegister(redisOperationDuration)

    redisKeysTotal := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "ratelimit_redis_keys_total",
        Help: "Total number of Redis keys used by rate limiter",
    })
    registry.MustRegister(redisKeysTotal)

    redisMemoryBytes := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "ratelimit_redis_memory_bytes",
        Help: "Redis memory usage in bytes",
    })
    registry.MustRegister(redisMemoryBytes)

    redisConnectedClients := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "ratelimit_redis_connected_clients",
        Help: "Number of connected Redis clients",
    })
    registry.MustRegister(redisConnectedClients)

    redisLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "ratelimit_redis_latency_milliseconds",
        Help:    "Redis operation latency in milliseconds",
        Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 20, 50, 100},
    }, []string{"operation"})
    registry.MustRegister(redisLatency)

    redisHitRate := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "ratelimit_redis_hit_rate",
        Help: "Redis cache hit rate (0-1)",
    })
    registry.MustRegister(redisHitRate)

    // Config metrics
    configReloadsTotal := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "ratelimit_config_reloads_total",
        Help: "Total number of config reloads",
    })
    registry.MustRegister(configReloadsTotal)

    configReloadErrorsTotal := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "ratelimit_config_reload_errors_total",
        Help: "Total number of config reload errors",
    })
    registry.MustRegister(configReloadErrorsTotal)

    return &DefaultMetricsCollector{
        registry:                registry,
        totalRateLimitsActive:   totalRateLimitsActive,
        totalViolatingUsers:     totalViolatingUsers,
        rateLimitChecksTotal:    rateLimitChecksTotal,
        rateLimitResetsTotal:    rateLimitResetsTotal,
        requestsTotal:           requestsTotal,
        apiRequestsTotal:        apiRequestsTotal,
        apiRequestDuration:      apiRequestDuration,
        redisOperationsTotal:    redisOperationsTotal,
        redisOperationDuration:  redisOperationDuration,
        redisKeysTotal:          redisKeysTotal,
        redisMemoryBytes:        redisMemoryBytes,
        redisConnectedClients:   redisConnectedClients,
        redisLatency:            redisLatency,
        redisHitRate:            redisHitRate,
        configReloadsTotal:      configReloadsTotal,
        configReloadErrorsTotal: configReloadErrorsTotal,
    }
}

func (m *DefaultMetricsCollector) UpdateRateLimitMetrics(violatingCount int, activeLimitsCount int) {
    m.totalViolatingUsers.Set(float64(violatingCount))
    m.totalRateLimitsActive.Set(float64(activeLimitsCount))
}

func (m *DefaultMetricsCollector) UpdateViolatingUsers(count int) {
    m.totalViolatingUsers.Set(float64(count))
}

func (m *DefaultMetricsCollector) UpdateActiveLimits(count int) {
    m.totalRateLimitsActive.Set(float64(count))
}

func (m *DefaultMetricsCollector) RecordRateLimitCheck(key string, allowed bool, limit int) {
    result := "allowed"
    if !allowed {
        result = "rejected"
    }
    m.rateLimitChecksTotal.WithLabelValues(key, result).Inc()
}

func (m *DefaultMetricsCollector) RecordRateLimitRequest(path string, allowed bool) {
    result := "allowed"
    if !allowed {
        result = "rejected"
    }
    m.requestsTotal.WithLabelValues(result, path).Inc()
}

func (m *DefaultMetricsCollector) RecordRateLimitReset(key string) {
    m.rateLimitResetsTotal.Inc()
}

func (m *DefaultMetricsCollector) RecordAPIRequest(endpoint, method, status string, duration float64) {
    m.apiRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
    m.apiRequestDuration.WithLabelValues(endpoint, method).Observe(duration)
}

func (m *DefaultMetricsCollector) RecordRedisOperation(operation, status string, duration float64) {
    m.redisOperationsTotal.WithLabelValues(operation, status).Inc()
    m.redisOperationDuration.WithLabelValues(operation).Observe(duration)
}

func (m *DefaultMetricsCollector) RecordConfigReload(success bool) {
    m.configReloadsTotal.Inc()
    if !success {
        m.configReloadErrorsTotal.Inc()
    }
}

func (m *DefaultMetricsCollector) GetRegistry() *prometheus.Registry {
    return m.registry
}

func (m *DefaultMetricsCollector) RecordRateLimit(key string, allowed bool, current int, limit int) {
    result := "allowed"
    if !allowed {
        result = "rejected"
    }
    m.rateLimitChecksTotal.WithLabelValues(key, result).Inc()
    
    path := extractPathFromKey(key)
    m.RecordRateLimitRequest(path, allowed)
}

func (m *DefaultMetricsCollector) UpdateRedisKeysCount(count int) {
    m.redisKeysTotal.Set(float64(count))
}

func (m *DefaultMetricsCollector) UpdateRedisMemoryUsage(bytes int64) {
    m.redisMemoryBytes.Set(float64(bytes))
}

func (m *DefaultMetricsCollector) UpdateRedisConnectedClients(count int) {
    m.redisConnectedClients.Set(float64(count))
}

func (m *DefaultMetricsCollector) RecordRedisLatency(operation string, latencyMs float64) {
    m.redisLatency.WithLabelValues(operation).Observe(latencyMs)
}

func (m *DefaultMetricsCollector) UpdateRedisHitRate(rate float64) {
    m.redisHitRate.Set(rate)
}

func extractPathFromKey(key string) string {
    for _, part := range strings.Split(key, "|") {
        if strings.HasPrefix(part, "path=") {
            return strings.TrimPrefix(part, "path=")
        }
    }
    return "unknown"
}