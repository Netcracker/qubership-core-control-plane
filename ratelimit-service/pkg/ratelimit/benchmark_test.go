// pkg/ratelimit/benchmark_test.go
package ratelimit

import (
    "context"
    "testing"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
)

func setupBenchmarkRedis(b *testing.B) (*RedisClient, *miniredis.Miniredis) {
    mr := miniredis.RunT(b)
    
    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })
    
    redisClient := &RedisClient{
        client: client,
    }
    
    return redisClient, mr
}

func BenchmarkCheckRateLimit(b *testing.B) {
    redisClient, mr := setupBenchmarkRedis(b)
    defer mr.Close()
    
    manager := NewRateLimitManager(redisClient)
    redisClient.SetManager(manager)
    
    rule := &Rule{
        Name:      "bench",
        Pattern:   ".*",
        Limit:     1000,
        Window:    time.Minute,
        Algorithm: FixedWindow,
    }
    manager.AddRule(rule)
    
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        manager.Check(ctx, "test-key")
    }
}

func BenchmarkCheckWithComponents(b *testing.B) {
    redisClient, mr := setupBenchmarkRedis(b)
    defer mr.Close()
    
    manager := NewRateLimitManager(redisClient)
    redisClient.SetManager(manager)
    
    rule := &Rule{
        Name:      "bench",
        Pattern:   ".*user_id=.*",
        Limit:     1000,
        Window:    time.Minute,
        Algorithm: FixedWindow,
    }
    manager.AddRule(rule)
    
    ctx := context.Background()
    components := map[string]string{
        "user_id": "bench-user",
        "path":    "/api/test",
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        manager.CheckWithComponents(ctx, components, "|")
    }
}