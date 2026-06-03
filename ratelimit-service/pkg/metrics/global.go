package metrics

var globalMetrics MetricsCollector

func InitGlobalMetrics() {
    globalMetrics = NewDefaultMetricsCollector()
}

func SetGlobalMetrics(m MetricsCollector) {
    globalMetrics = m
}

func GetGlobalMetrics() MetricsCollector {
    if globalMetrics == nil {
        InitGlobalMetrics()
    }
    return globalMetrics
}

// Convenience functions
func UpdateRateLimitMetrics(violatingCount int, activeLimitsCount int) {
    GetGlobalMetrics().UpdateRateLimitMetrics(violatingCount, activeLimitsCount)
}

func RecordRateLimitCheck(key string, allowed bool, limit int) {
    GetGlobalMetrics().RecordRateLimitCheck(key, allowed, limit)
}

func RecordRateLimitReset(key string) {
    GetGlobalMetrics().RecordRateLimitReset(key)
}

func RecordAPIRequest(endpoint, method, status string, duration float64) {
    GetGlobalMetrics().RecordAPIRequest(endpoint, method, status, duration)
}

func RecordRedisOperation(operation, status string, duration float64) {
    GetGlobalMetrics().RecordRedisOperation(operation, status, duration)
}

func RecordConfigReload(success bool) {
    GetGlobalMetrics().RecordConfigReload(success)
}
