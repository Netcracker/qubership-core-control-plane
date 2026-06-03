package ratelimit

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/redis/go-redis/v9"
    "k8s.io/klog/v2"
)

type RedisClient struct {
    client  *redis.Client
    manager *RateLimitManager
}

// NewRedisClient creates a new Redis client
func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
    client := redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
    })
    
    // Test connection
    ctx := context.Background()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Redis: %w", err)
    }
    
    return &RedisClient{
        client: client,
    }, nil
}

// SetManager sets the rate limit manager
func (c *RedisClient) SetManager(manager *RateLimitManager) {
    c.manager = manager
}

// CheckRateLimit checks rate limit for a key
func (c *RedisClient) CheckRateLimit(ctx context.Context, key string, rule *Rule) (allowed bool, current int, err error) {
    redisKey := fmt.Sprintf("ratelimit:%s:%s", rule.Name, key)
    
    // Increment counter
    count, err := c.client.Incr(ctx, redisKey).Result()
    if err != nil {
        return true, 0, err
    }
    
    current = int(count)
    
    // Set TTL on first request
    if count == 1 {
        c.client.Expire(ctx, redisKey, rule.Window)
    }
    
    allowed = current <= rule.Limit
    
    if !allowed {
        klog.V(4).Infof("Rate limit exceeded for key %s: %d/%d", key, current, rule.Limit)
    }
    
    return allowed, current, nil
}

// GetUserRateLimitInfo gets rate limit info for a user
func (c *RedisClient) GetUserRateLimitInfo(ctx context.Context, userID string) (map[string]interface{}, error) {
    pattern := fmt.Sprintf("*%s*", userID)
    keys, err := c.client.Keys(ctx, pattern).Result()
    if err != nil {
        return nil, err
    }
    
    info := make(map[string]interface{})
    for _, key := range keys {
        val, err := c.client.Get(ctx, key).Result()
        if err != nil {
            continue
        }
        info[key] = val
    }
    
    return info, nil
}

// GetViolatingUsers gets users who are currently rate limited
func (c *RedisClient) GetViolatingUsers(ctx context.Context) ([]map[string]interface{}, error) {
    pattern := "ratelimit:*"
    keys, err := c.client.Keys(ctx, pattern).Result()
    if err != nil {
        return nil, err
    }
    
    usersMap := make(map[string]bool)
    for _, key := range keys {
        // Extract user ID from key
        if userID := extractUserIDFromKey(key); userID != "" {
            usersMap[userID] = true
        }
    }
    
    users := make([]map[string]interface{}, 0, len(usersMap))
    for userID := range usersMap {
        users = append(users, map[string]interface{}{
            "user_id": userID,
            "reason":  "rate_limit_exceeded",
        })
    }
    
    return users, nil
}

// GetAllStatistics gets all rate limit statistics
func (c *RedisClient) GetAllStatistics(ctx context.Context) (map[string]interface{}, error) {
    keys, err := c.client.Keys(ctx, "ratelimit:*").Result()
    if err != nil {
        return nil, err
    }
    
    stats := make(map[string]interface{})
    for _, key := range keys {
        val, err := c.client.Get(ctx, key).Result()
        if err != nil {
            continue
        }
        stats[key] = val
    }
    
    return stats, nil
}

// ResetUserRateLimit resets rate limits for a user
func (c *RedisClient) ResetUserRateLimit(ctx context.Context, userID string) error {
    pattern := fmt.Sprintf("*%s*", userID)
    keys, err := c.client.Keys(ctx, pattern).Result()
    if err != nil {
        return err
    }
    
    for _, key := range keys {
        if err := c.client.Del(ctx, key).Err(); err != nil {
            return err
        }
        klog.V(4).Infof("Deleted Redis key: %s", key)
    }
    
    klog.Infof("Reset rate limits for user: %s (deleted %d keys)", userID, len(keys))
    return nil
}

func (c *RedisClient) GetRedisStats(ctx context.Context) (map[string]interface{}, error) {
    // Get Redis INFO
    info, err := c.client.Info(ctx, "memory", "stats", "clients").Result()
    if err != nil {
        return nil, err
    }
    
    stats := make(map[string]interface{})
    
    // Parse Redis INFO output
    lines := strings.Split(info, "\n")
    for _, line := range lines {
        if strings.Contains(line, ":") && !strings.HasPrefix(line, "#") {
            parts := strings.SplitN(line, ":", 2)
            if len(parts) == 2 {
                key := strings.TrimSpace(parts[0])
                value := strings.TrimSpace(parts[1])
                
                switch key {
                case "used_memory":
                    var mem int64
                    fmt.Sscanf(value, "%d", &mem)
                    stats["used_memory"] = mem
                case "connected_clients":
                    var clients int
                    fmt.Sscanf(value, "%d", &clients)
                    stats["connected_clients"] = clients
                case "keyspace_hits", "keyspace_misses":
                    var val int64
                    fmt.Sscanf(value, "%d", &val)
                    stats[key] = val
                }
            }
        }
    }
    
    // Get total keys count for rate limit keys only
    keys, err := c.client.Keys(ctx, "ratelimit:*").Result()
    if err == nil {
        stats["total_keys"] = len(keys)
    }
    
    // Calculate hit rate
    hits, _ := stats["keyspace_hits"].(int64)
    misses, _ := stats["keyspace_misses"].(int64)
    total := hits + misses
    if total > 0 {
        stats["hit_rate"] = float64(hits) / float64(total)
    } else {
        stats["hit_rate"] = 0.0
    }
    
    return stats, nil
}

// extractUserIDFromKey extracts user ID from Redis key
func extractUserIDFromKey(key string) string {
    // Key format: ratelimit:rule_name:path=/test|user_id=xxx
    parts := strings.Split(key, "|")
    for _, part := range parts {
        if strings.Contains(part, "user_id=") {
            userPart := strings.Split(part, "user_id=")
            if len(userPart) > 1 {
                // Remove any trailing characters
                userID := strings.Split(userPart[1], "&")[0]
                userID = strings.Split(userID, "|")[0]
                userID = strings.Split(userID, ":")[0]
                return userID
            }
        }
    }
    return ""
}

// Close closes Redis connection
func (c *RedisClient) Close() error {
    return c.client.Close()
}