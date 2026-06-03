package setup

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
)

type RedisClient struct {
    client *redis.Client
}

func NewRedisClient(addr string) *RedisClient {
    rdb := redis.NewClient(&redis.Options{
        Addr: addr,
        DB:   15, 
    })
    
    return &RedisClient{client: rdb}
}

func (r *RedisClient) FlushAll(ctx context.Context) error {
    return r.client.FlushDB(ctx).Err()
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
    return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
    return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
    return r.client.Incr(ctx, key).Result()
}

func (r *RedisClient) Close() error {
    return r.client.Close()
}

func (r *RedisClient) Ping(ctx context.Context) error {
    return r.client.Ping(ctx).Err()
}

func (r *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
    return r.client.Keys(ctx, pattern).Result()
}