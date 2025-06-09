package utils

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

var RedisCacheDuration = 30 * time.Second

func GetFromCache(ctx context.Context, client *redis.Client, key string) (string, error) {
	return client.Get(ctx, key).Result()
}

func SetToCache(ctx context.Context, client *redis.Client, key string, value string, ttl time.Duration) error {
	return client.Set(ctx, key, value, ttl).Err()
}

func DeleteFromCache(ctx context.Context, client *redis.Client, key string) error {
	return client.Del(ctx, key).Err()
}
