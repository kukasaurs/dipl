package utils

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/redis/go-redis/v9"
	"time"
)

// RedisClient is a wrapper around redis.Client
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient creates a new Redis client from the provided URL
func NewRedisClient(redisURL string) (*RedisClient, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(options)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisClient{client: client}, nil
}

// Set stores a value in Redis with the given key and expiration
func (c *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

// Get retrieves a value from Redis
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

// Delete removes a key from Redis
func (c *RedisClient) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Close closes the Redis client connection
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
