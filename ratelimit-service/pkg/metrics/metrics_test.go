package metrics

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestInitGlobalMetrics(t *testing.T) {

    globalMetrics = nil

    InitGlobalMetrics()
    assert.NotNil(t, GetGlobalMetrics())

    InitGlobalMetrics()
    assert.NotNil(t, GetGlobalMetrics())
}

func TestSetAndGetGlobalMetrics(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)
    assert.Equal(t, mock, GetGlobalMetrics())

    mock2 := NewMockMetricsCollector()
    SetGlobalMetrics(mock2)
    assert.Equal(t, mock2, GetGlobalMetrics())
}

func TestUpdateRateLimitMetrics(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)

    UpdateRateLimitMetrics(10, 25)

    assert.True(t, mock.UpdateRateLimitMetricsCalled)
    assert.Equal(t, 10, mock.UpdateRateLimitViolatingCount)
    assert.Equal(t, 25, mock.UpdateRateLimitActiveCount)
}

func TestRecordRateLimitCheck(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)

    RecordRateLimitCheck("test_key", true, 30)

    assert.True(t, mock.RecordRateLimitCheckCalled)
    assert.Equal(t, "test_key", mock.RecordRateLimitCheckKey)
    assert.True(t, mock.RecordRateLimitCheckAllowed)
}

func TestRecordRateLimitReset(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)

    RecordRateLimitReset("test_key")

    assert.True(t, mock.RecordRateLimitResetCalled)
    assert.Equal(t, "test_key", mock.RecordRateLimitResetKey)
}

func TestRecordAPIRequest(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)

    RecordAPIRequest("/test", "GET", "200", 0.5)

    assert.True(t, mock.RecordAPIRequestCalled)
    assert.Equal(t, "/test", mock.RecordAPIRequestEndpoint)
    assert.Equal(t, "GET", mock.RecordAPIRequestMethod)
    assert.Equal(t, "200", mock.RecordAPIRequestStatus)
    assert.Equal(t, 0.5, mock.RecordAPIRequestDuration)
}

func TestRecordRedisOperation(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)

    RecordRedisOperation("get", "success", 0.05)

    assert.True(t, mock.RecordRedisOperationCalled)
    assert.Equal(t, "get", mock.RecordRedisOperationOp)
    assert.Equal(t, "success", mock.RecordRedisOperationStatus)
    assert.Equal(t, 0.05, mock.RecordRedisOperationDur)
}

func TestRecordConfigReload(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)

    RecordConfigReload(true)
    RecordConfigReload(false)

    assert.True(t, mock.RecordConfigReloadCalled)

    t.Log("Config reload recorded successfully")
}

func TestMockReset(t *testing.T) {
    mock := NewMockMetricsCollector()
    SetGlobalMetrics(mock)

    RecordRateLimitCheck("key", true, 30)
    assert.True(t, mock.RecordRateLimitCheckCalled)

    mock.Reset()
    assert.False(t, mock.RecordRateLimitCheckCalled)
}
