// pkg/ratelimit/redis_client_test.go
package ratelimit

import (
    "context"
    "testing"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/assert"
)

func setupTestRedis(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
    mr := miniredis.RunT(t)
    
    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })
    
    redisClient := &RedisClient{
        client: client,
    }
    
    return redisClient, mr
}

func TestRedisClient_CheckRateLimit(t *testing.T) {
    client, mr := setupTestRedis(t)
    defer mr.Close()
    
    ctx := context.Background()
    rule := &Rule{
        Name:   "test",
        Limit:  2,
        Window: 10 * time.Second,
    }
    
    // First request
    allowed, current, err := client.CheckRateLimit(ctx, "test-key", rule)
    assert.NoError(t, err)
    assert.True(t, allowed)
    assert.Equal(t, 1, current)
    
    // Second request
    allowed, current, err = client.CheckRateLimit(ctx, "test-key", rule)
    assert.NoError(t, err)
    assert.True(t, allowed)
    assert.Equal(t, 2, current)
    
    // Third request (should be denied)
    allowed, current, err = client.CheckRateLimit(ctx, "test-key", rule)
    assert.NoError(t, err)
    assert.False(t, allowed)
    assert.Equal(t, 3, current)
}

func TestRedisClient_ResetUserRateLimit(t *testing.T) {
    client, mr := setupTestRedis(t)
    defer mr.Close()
    
    ctx := context.Background()
    
    // Create some keys
    rule := &Rule{Name: "test", Limit: 10, Window: time.Minute}
    client.CheckRateLimit(ctx, "user_id=test1", rule)
    client.CheckRateLimit(ctx, "user_id=test2", rule)
    
    // Reset for test1
    err := client.ResetUserRateLimit(ctx, "test1")
    assert.NoError(t, err)
    
    // Check keys
    keys, _ := client.client.Keys(ctx, "*").Result()
    assert.Equal(t, 1, len(keys)) // Only test2 key should remain
}

func TestRedisClient_GetViolatingUsers(t *testing.T) {
    client, mr := setupTestRedis(t)
    defer mr.Close()
    
    ctx := context.Background()
    
    // Create rate limited users
    rule := &Rule{Name: "test", Limit: 1, Window: time.Minute}
    
    // User1: exceed limit
    client.CheckRateLimit(ctx, "user_id=user1", rule)
    client.CheckRateLimit(ctx, "user_id=user1", rule)
    
    // User2: within limit
    client.CheckRateLimit(ctx, "user_id=user2", rule)
    
    users, err := client.GetViolatingUsers(ctx)
    assert.NoError(t, err)
    assert.GreaterOrEqual(t, len(users), 1)
}