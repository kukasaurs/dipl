// utils/redis.go
package utils

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/url"
	"time"
)

var RedisCacheDuration = 5 * time.Minute

func NewRedisClient(redisURL string) *redis.Client {
	u, err := url.Parse(redisURL)
	if err != nil {
		panic(fmt.Sprintf("invalid redis url: %v", err))
	}

	password, _ := u.User.Password()
	addr := u.Host

	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
}

func CloseRedis(ctx context.Context, client *redis.Client) error {
	return client.Close()
}

func GetFromCache(ctx context.Context, client *redis.Client, key string) (string, error) {
	return client.Get(ctx, key).Result()
}

func SetToCache(ctx context.Context, client *redis.Client, key string, value string, ttl time.Duration) error {
	return client.Set(ctx, key, value, ttl).Err()
}

func DeleteFromCache(ctx context.Context, client *redis.Client, key string) error {
	return client.Del(ctx, key).Err()
}
