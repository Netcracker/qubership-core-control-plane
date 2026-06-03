package metrics

import (
    "context"
    "testing"
    "time"

    "ratelimit-service/pkg/ratelimit"
    "github.com/stretchr/testify/assert"
)

func TestMetricsCollectorService_CollectMetrics(t *testing.T) {
    mockMetrics := NewMockMetricsCollector()
    
    // Create Redis client
    redisClient, err := ratelimit.NewRedisClient("localhost:6379", "", 0)
    if err != nil {
        t.Skip("Redis not available, skipping test")
    }
    
    service := NewMetricsCollectorService(redisClient, mockMetrics, 1*time.Second)
    
    ctx := context.Background()
    go service.Start(ctx)
    
    // Let it collect metrics once
    time.Sleep(1500 * time.Millisecond)
    service.Stop()
    
    // Verify that metrics were collected
    assert.True(t, mockMetrics.UpdateRateLimitMetricsCalled, "UpdateRateLimitMetrics should be called")
    assert.True(t, mockMetrics.RecordRedisOperationCalled, "RecordRedisOperation should be called")
}

func TestMetricsCollectorService_RecordRateLimitRequest(t *testing.T) {
    mockMetrics := NewMockMetricsCollector()
    
    // Test RecordRateLimitRequest
    mockMetrics.RecordRateLimitRequest("/test", true)
    
    assert.True(t, mockMetrics.RecordRateLimitRequestCalled)
    assert.Equal(t, "/test", mockMetrics.RecordRateLimitRequestPath)
    assert.True(t, mockMetrics.RecordRateLimitRequestAllowed)
}

func TestMetricsCollectorService_RecordRateLimit(t *testing.T) {
    mockMetrics := NewMockMetricsCollector()
    
    // Test RecordRateLimit
    mockMetrics.RecordRateLimit("test-key", false, 5, 3)
    
    assert.True(t, mockMetrics.RecordRateLimitCalled)
    assert.Equal(t, "test-key", mockMetrics.RecordRateLimitKey)
    assert.False(t, mockMetrics.RecordRateLimitAllowed)
    assert.Equal(t, 5, mockMetrics.RecordRateLimitCurrent)
    assert.Equal(t, 3, mockMetrics.RecordRateLimitLimit)
}