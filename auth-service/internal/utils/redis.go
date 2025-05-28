package utils

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisClient struct {
	client *redis.Client
}

func (c *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *RedisClient) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errors.New("key not found")
		}
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

func (c *RedisClient) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisClient) Close() error {
	return c.client.Close()
}

func (c *RedisClient) Exists(ctx context.Context, key string) bool {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return count > 0
}
func WrapRedisClient(client *redis.Client) *RedisClient {
	return &RedisClient{client: client}
}
