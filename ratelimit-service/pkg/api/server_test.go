package api

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "ratelimit-service/pkg/ratelimit"
    "github.com/alicebob/miniredis/v2"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func createTestServerWithRedis(t *testing.T) (*Server, *miniredis.Miniredis) {
    mr := miniredis.RunT(t)
    
    redisClient, err := ratelimit.NewRedisClient(mr.Addr(), "", 0)
    require.NoError(t, err)
    
    rateLimitManager := ratelimit.NewRateLimitManager(redisClient)
    redisClient.SetManager(rateLimitManager)
    
    server := NewServer(redisClient, nil, rateLimitManager)
    
    return server, mr
}

func TestServer_CheckRateLimit(t *testing.T) {
    server, mr := createTestServerWithRedis(t)
    defer mr.Close()
    
    rule := &ratelimit.Rule{
        Name:      "test_rule",
        Pattern:   ".*user_id=test.*",
        Limit:     2,
        Window:    time.Minute,
        Algorithm: ratelimit.FixedWindow,
    }
    err := server.rateLimitManager.AddRule(rule)
    require.NoError(t, err)
    
    tests := []struct {
        name         string
        components   map[string]string
        expectedCode int
        expectedAllowed bool
    }{
        {
            name: "matching user - first request allowed",
            components: map[string]string{
                "user_id": "test",
                "path":    "/api",
            },
            expectedCode: http.StatusOK,
            expectedAllowed: true,
        },
        {
            name: "non-matching user - always allowed",
            components: map[string]string{
                "user_id": "other",
                "path":    "/api",
            },
            expectedCode: http.StatusOK,
            expectedAllowed: true,
        },
    }
    
    
    t.Run("matching user - first request", func(t *testing.T) {
        reqBody := map[string]interface{}{
            "components": tests[0].components,
        }
        body, _ := json.Marshal(reqBody)
        req := httptest.NewRequest("POST", "/api/v1/ratelimit/check", bytes.NewBuffer(body))
        req.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        
        server.CheckRateLimit(w, req)

        assert.Equal(t, http.StatusOK, w.Code)

        var response map[string]interface{}
        json.NewDecoder(w.Body).Decode(&response)

        allowed, ok := response["allowed"].(bool)
        assert.True(t, ok)
        assert.True(t, allowed, "First request should be allowed")
        
        limit, ok := response["limit"].(float64)
        assert.True(t, ok)
        assert.Equal(t, float64(2), limit)
        
        remaining, ok := response["remaining"].(float64)
        assert.True(t, ok)
        assert.Equal(t, float64(1), remaining)
    })
    
    t.Run("matching user - second request", func(t *testing.T) {
        reqBody := map[string]interface{}{
            "components": tests[0].components,
        }
        body, _ := json.Marshal(reqBody)
        req := httptest.NewRequest("POST", "/api/v1/ratelimit/check", bytes.NewBuffer(body))
        req.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        
        server.CheckRateLimit(w, req)

        assert.Equal(t, http.StatusOK, w.Code)

        var response map[string]interface{}
        json.NewDecoder(w.Body).Decode(&response)

        allowed, ok := response["allowed"].(bool)
        assert.True(t, ok)
        assert.True(t, allowed, "Second request should be allowed")
        
        remaining, ok := response["remaining"].(float64)
        assert.True(t, ok)
        assert.Equal(t, float64(0), remaining)
    })
    
    t.Run("matching user - third request rejected", func(t *testing.T) {
        reqBody := map[string]interface{}{
            "components": tests[0].components,
        }
        body, _ := json.Marshal(reqBody)
        req := httptest.NewRequest("POST", "/api/v1/ratelimit/check", bytes.NewBuffer(body))
        req.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        
        server.CheckRateLimit(w, req)

        assert.Equal(t, http.StatusOK, w.Code)

        var response map[string]interface{}
        json.NewDecoder(w.Body).Decode(&response)

        allowed, ok := response["allowed"].(bool)
        assert.True(t, ok)
        assert.False(t, allowed, "Third request should be rate limited")
        
        remaining, ok := response["remaining"].(float64)
        assert.True(t, ok)
        assert.Equal(t, float64(0), remaining)
    })
    
    t.Run("non-matching user", func(t *testing.T) {
        reqBody := map[string]interface{}{
            "components": tests[1].components,
        }
        body, _ := json.Marshal(reqBody)
        req := httptest.NewRequest("POST", "/api/v1/ratelimit/check", bytes.NewBuffer(body))
        req.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        
        server.CheckRateLimit(w, req)

        assert.Equal(t, http.StatusOK, w.Code)

        var response map[string]interface{}
        json.NewDecoder(w.Body).Decode(&response)

        allowed, ok := response["allowed"].(bool)
        assert.True(t, ok)
        assert.True(t, allowed, "Non-matching user should be allowed")
    })
}

func TestServer_CheckRateLimit_NoRules(t *testing.T) {
    server, mr := createTestServerWithRedis(t)
    defer mr.Close()
    
    reqBody := map[string]interface{}{
        "components": map[string]string{
            "user_id": "any-user",
            "path":    "/api",
        },
    }
    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/api/v1/ratelimit/check", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    
    server.CheckRateLimit(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    
    var response map[string]interface{}
    json.NewDecoder(w.Body).Decode(&response)
    
    allowed, ok := response["allowed"].(bool)
    assert.True(t, ok)
    assert.True(t, allowed, "Without rules, all requests should be allowed")
    
    limit, ok := response["limit"].(float64)
    assert.True(t, ok)
    assert.Equal(t, float64(0), limit, "Limit should be 0 when no rules match")
}