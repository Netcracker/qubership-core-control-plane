package metrics

import "github.com/prometheus/client_golang/prometheus"

type MockMetricsCollector struct {
    // Rate limit metrics
    UpdateRateLimitMetricsCalled bool
    UpdateRateLimitViolatingCount int
    UpdateRateLimitActiveCount    int
    
    UpdateViolatingUsersCalled bool
    UpdateViolatingUsersCount   int
    
    UpdateActiveLimitsCalled bool
    UpdateActiveLimitsCount   int
    
    RecordRateLimitCheckCalled bool
    RecordRateLimitCheckKey    string
    RecordRateLimitCheckAllowed bool
    RecordRateLimitCheckLimit   int
    
    RecordRateLimitResetCalled bool
    RecordRateLimitResetKey    string
    
    RecordRateLimitRequestCalled bool
    RecordRateLimitRequestPath   string
    RecordRateLimitRequestAllowed bool
    
    RecordRateLimitCalled bool
    RecordRateLimitKey    string
    RecordRateLimitAllowed bool
    RecordRateLimitCurrent int
    RecordRateLimitLimit   int
    
    RecordAPIRequestCalled   bool
    RecordAPIRequestEndpoint string
    RecordAPIRequestMethod   string
    RecordAPIRequestStatus   string
    RecordAPIRequestDuration float64
    
    RecordRedisOperationCalled bool
    RecordRedisOperationOp     string
    RecordRedisOperationStatus string
    RecordRedisOperationDur    float64
    
    RecordConfigReloadCalled bool
    RecordConfigReloadSuccess bool
}

func NewMockMetricsCollector() *MockMetricsCollector {
    return &MockMetricsCollector{}
}

func (m *MockMetricsCollector) UpdateRateLimitMetrics(violatingCount int, activeLimitsCount int) {
    m.UpdateRateLimitMetricsCalled = true
    m.UpdateRateLimitViolatingCount = violatingCount
    m.UpdateRateLimitActiveCount = activeLimitsCount
}

func (m *MockMetricsCollector) UpdateViolatingUsers(count int) {
    m.UpdateViolatingUsersCalled = true
    m.UpdateViolatingUsersCount = count
}

func (m *MockMetricsCollector) UpdateActiveLimits(count int) {
    m.UpdateActiveLimitsCalled = true
    m.UpdateActiveLimitsCount = count
}

func (m *MockMetricsCollector) RecordRateLimitCheck(key string, allowed bool, limit int) {
    m.RecordRateLimitCheckCalled = true
    m.RecordRateLimitCheckKey = key
    m.RecordRateLimitCheckAllowed = allowed
    m.RecordRateLimitCheckLimit = limit
}

func (m *MockMetricsCollector) RecordRateLimitReset(key string) {
    m.RecordRateLimitResetCalled = true
    m.RecordRateLimitResetKey = key
}

func (m *MockMetricsCollector) RecordRateLimitRequest(path string, allowed bool) {
    m.RecordRateLimitRequestCalled = true
    m.RecordRateLimitRequestPath = path
    m.RecordRateLimitRequestAllowed = allowed
}

func (m *MockMetricsCollector) RecordAPIRequest(endpoint, method, status string, duration float64) {
    m.RecordAPIRequestCalled = true
    m.RecordAPIRequestEndpoint = endpoint
    m.RecordAPIRequestMethod = method
    m.RecordAPIRequestStatus = status
    m.RecordAPIRequestDuration = duration
}

func (m *MockMetricsCollector) RecordRedisOperation(operation, status string, duration float64) {
    m.RecordRedisOperationCalled = true
    m.RecordRedisOperationOp = operation
    m.RecordRedisOperationStatus = status
    m.RecordRedisOperationDur = duration
}

func (m *MockMetricsCollector) RecordConfigReload(success bool) {
    m.RecordConfigReloadCalled = true
    m.RecordConfigReloadSuccess = success
}

func (m *MockMetricsCollector) UpdateRedisKeysCount(_ int) {}

func (m *MockMetricsCollector) UpdateRedisMemoryUsage(_ int64) {}

func (m *MockMetricsCollector) UpdateRedisConnectedClients(_ int) {}

func (m *MockMetricsCollector) RecordRedisLatency(_ string, _ float64) {}

func (m *MockMetricsCollector) UpdateRedisHitRate(_ float64) {}

func (m *MockMetricsCollector) RecordRateLimit(key string, allowed bool, current int, limit int) {
    m.RecordRateLimitCalled = true
    m.RecordRateLimitKey = key
    m.RecordRateLimitAllowed = allowed
    m.RecordRateLimitCurrent = current
    m.RecordRateLimitLimit = limit
}

func (m *MockMetricsCollector) GetRegistry() *prometheus.Registry {
    return prometheus.NewRegistry()
}

func (m *MockMetricsCollector) Reset() {
    m.UpdateRateLimitMetricsCalled = false
    m.UpdateViolatingUsersCalled = false
    m.UpdateActiveLimitsCalled = false
    m.RecordRateLimitCheckCalled = false
    m.RecordRateLimitResetCalled = false
    m.RecordRateLimitRequestCalled = false
    m.RecordRateLimitCalled = false
    m.RecordAPIRequestCalled = false
    m.RecordRedisOperationCalled = false
    m.RecordConfigReloadCalled = false
}