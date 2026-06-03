package metrics

import (
    "context"
    "time"

    "ratelimit-service/pkg/ratelimit"

    "k8s.io/klog/v2"
)

type MetricsCollectorService struct {
    redisClient *ratelimit.RedisClient
    metrics     MetricsCollector
    interval    time.Duration
    stopCh      chan struct{}
}

func NewMetricsCollectorService(redisClient *ratelimit.RedisClient, metrics MetricsCollector, interval time.Duration) *MetricsCollectorService {
    if interval == 0 {
        interval = 30 * time.Second
    }

    return &MetricsCollectorService{
        redisClient: redisClient,
        metrics:     metrics,
        interval:    interval,
        stopCh:      make(chan struct{}),
    }
}

func (s *MetricsCollectorService) Start(ctx context.Context) {
    klog.Info("Starting metrics collector service")

    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    s.collectMetrics(ctx)

    for {
        select {
        case <-ctx.Done():
            klog.Info("Stopping metrics collector service")
            return
        case <-s.stopCh:
            klog.Info("Metrics collector service stopped")
            return
        case <-ticker.C:
            s.collectMetrics(ctx)
        }
    }
}

func (s *MetricsCollectorService) Stop() {
    close(s.stopCh)
}

func (s *MetricsCollectorService) collectMetrics(ctx context.Context) {
    start := time.Now()

    // Get violating users
    violating, err := s.redisClient.GetViolatingUsers(ctx)
    if err != nil {
        klog.Errorf("Failed to get violating users: %v", err)
        s.metrics.RecordConfigReload(false)
        return
    }
    
    violatingCount := len(violating)
    s.metrics.UpdateViolatingUsers(violatingCount)

    // Get statistics
    stats, err := s.redisClient.GetAllStatistics(ctx)
    if err != nil {
        klog.Errorf("Failed to get statistics: %v", err)
        s.metrics.RecordConfigReload(false)
        return
    }

    // Extract total keys count from stats map
    totalKeys := len(stats)
    s.metrics.UpdateRateLimitMetrics(violatingCount, totalKeys)
    
    // NEW: Get Redis stats for monitoring
    redisStats, err := s.redisClient.GetRedisStats(ctx)
    if err != nil {
        klog.Warningf("Failed to get Redis stats: %v", err)
    } else {
        if mem, ok := redisStats["used_memory"].(int64); ok {
            s.metrics.UpdateRedisMemoryUsage(mem)
        }
        if clients, ok := redisStats["connected_clients"].(int); ok {
            s.metrics.UpdateRedisConnectedClients(clients)
        }
        if keys, ok := redisStats["total_keys"].(int); ok {
            s.metrics.UpdateRedisKeysCount(keys)
        }
        if hitRate, ok := redisStats["hit_rate"].(float64); ok {
            s.metrics.UpdateRedisHitRate(hitRate)
        }
    }
    
    s.metrics.RecordRedisOperation("collect", "success", time.Since(start).Seconds())

    klog.V(4).Infof("Metrics updated: %d violating users, %d active limits, %d Redis keys",
        violatingCount, totalKeys, totalKeys)
}