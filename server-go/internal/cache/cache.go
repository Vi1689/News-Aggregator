package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheManager struct {
	client *redis.Client
}

func NewCacheManager(addr, password string, db int) *CacheManager {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		panic("Failed to connect to Redis: " + err.Error())
	}

	return &CacheManager{client: client}
}

func (c *CacheManager) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *CacheManager) SetEX(ctx context.Context, key string, value interface{}, seconds int) error {
	return c.client.Set(ctx, key, value, time.Duration(seconds)*time.Second).Err()
}

func (c *CacheManager) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *CacheManager) DelPattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func (c *CacheManager) Close() error {
	return c.client.Close()
}
