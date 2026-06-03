package metrics

import (
    "testing"

    dto "github.com/prometheus/client_model/go"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNewDefaultMetricsCollector(t *testing.T) {
    collector := NewDefaultMetricsCollector()
    
    assert.NotNil(t, collector)
    assert.NotNil(t, collector.GetRegistry())
    assert.NotNil(t, collector.totalRateLimitsActive)
    assert.NotNil(t, collector.totalViolatingUsers)
    assert.NotNil(t, collector.rateLimitChecksTotal)
    assert.NotNil(t, collector.rateLimitResetsTotal)
    assert.NotNil(t, collector.apiRequestsTotal)
    assert.NotNil(t, collector.apiRequestDuration)
    assert.NotNil(t, collector.redisOperationsTotal)
    assert.NotNil(t, collector.redisOperationDuration)
    assert.NotNil(t, collector.configReloadsTotal)
    assert.NotNil(t, collector.configReloadErrorsTotal)
}

func TestDefaultMetricsCollector_UpdateRateLimitMetrics(t *testing.T) {
    collector := NewDefaultMetricsCollector()
    
    collector.UpdateRateLimitMetrics(10, 25)
    
    var gauge dto.Metric
    
    collector.totalViolatingUsers.Write(&gauge)
    assert.Equal(t, 10.0, *gauge.Gauge.Value)
    
    collector.totalRateLimitsActive.Write(&gauge)
    assert.Equal(t, 25.0, *gauge.Gauge.Value)
}

func TestDefaultMetricsCollector_RecordRateLimitCheck(t *testing.T) {
    collector := NewDefaultMetricsCollector()
    
    collector.RecordRateLimitCheck("test_key", true, 30)
    collector.RecordRateLimitCheck("test_key", false, 30)
    
    metricFamilies, err := collector.GetRegistry().Gather()
    require.NoError(t, err)
    
    foundAllowed := false
    foundRejected := false
    
    for _, mf := range metricFamilies {
        if *mf.Name == "ratelimit_checks_total" {
            for _, metric := range mf.GetMetric() {
                labels := metric.GetLabel()
                for _, label := range labels {
                    if *label.Name == "result" {
                        if *label.Value == "allowed" {
                            foundAllowed = true
                        }
                        if *label.Value == "rejected" {
                            foundRejected = true
                        }
                    }
                }
            }
        }
    }
    
    assert.True(t, foundAllowed, "Should have allowed counter")
    assert.True(t, foundRejected, "Should have rejected counter")
}

func TestDefaultMetricsCollector_RecordRateLimitReset(t *testing.T) {
    collector := NewDefaultMetricsCollector()
    
    collector.RecordRateLimitReset("test_key")
    collector.RecordRateLimitReset("test_key")
    
    var counter dto.Metric
    collector.rateLimitResetsTotal.Write(&counter)
    assert.Equal(t, 2.0, *counter.Counter.Value)
}

func TestDefaultMetricsCollector_RecordAPIRequest(t *testing.T) {
    collector := NewDefaultMetricsCollector()
    
    collector.RecordAPIRequest("/test", "GET", "200", 0.5)
    
    metricFamilies, err := collector.GetRegistry().Gather()
    require.NoError(t, err)
    
    found := false
    for _, mf := range metricFamilies {
        if *mf.Name == "ratelimit_api_requests_total" {
            found = true
            break
        }
    }
    assert.True(t, found)
}

func TestDefaultMetricsCollector_RecordRedisOperation(t *testing.T) {
    collector := NewDefaultMetricsCollector()
    
    collector.RecordRedisOperation("get", "success", 0.05)
    
    metricFamilies, err := collector.GetRegistry().Gather()
    require.NoError(t, err)
    
    found := false
    for _, mf := range metricFamilies {
        if *mf.Name == "ratelimit_redis_operations_total" {
            found = true
            break
        }
    }
    assert.True(t, found)
}

func TestDefaultMetricsCollector_RecordConfigReload(t *testing.T) {
    collector := NewDefaultMetricsCollector()
    
    collector.RecordConfigReload(true)
    collector.RecordConfigReload(false)
    collector.RecordConfigReload(false)
    
    var counter dto.Metric
    
    collector.configReloadsTotal.Write(&counter)
    assert.Equal(t, 3.0, *counter.Counter.Value)
    
    collector.configReloadErrorsTotal.Write(&counter)
    assert.Equal(t, 2.0, *counter.Counter.Value)
}