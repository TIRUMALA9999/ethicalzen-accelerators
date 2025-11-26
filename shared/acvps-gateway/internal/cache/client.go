package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethicalzen/acvps-gateway/internal/config"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

// Client represents a Redis cache client
type Client struct {
	client *redis.Client
	config config.CacheConfig

	// Metrics
	hits   uint64
	misses uint64
}

// New creates a new cache client
func New(cfg config.CacheConfig) (*Client, error) {
	// Parse Redis URL
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Override with config
	if cfg.RedisPassword != "" {
		opt.Password = cfg.RedisPassword
	}
	opt.DB = cfg.RedisDB
	opt.PoolSize = cfg.PoolSize
	opt.MaxRetries = cfg.MaxRetries

	// Create client
	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.WithFields(log.Fields{
		"redis_url": cfg.RedisURL,
		"db":        cfg.RedisDB,
		"pool_size": cfg.PoolSize,
	}).Info("Cache client initialized")

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// Close closes the cache client connection
func (c *Client) Close() error {
	return c.client.Close()
}

// GetRedisClient returns the underlying Redis client
func (c *Client) GetRedisClient() *redis.Client {
	return c.client
}

// Ping checks if the cache is reachable
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Scan scans for keys matching a pattern
func (c *Client) Scan(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64
	for {
		var scanKeys []string
		var err error
		scanKeys, cursor, err = c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan keys: %w", err)
		}
		keys = append(keys, scanKeys...)
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

// Get retrieves a value from the cache
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		atomic.AddUint64(&c.misses, 1)
		return "", fmt.Errorf("cache miss")
	}
	if err != nil {
		atomic.AddUint64(&c.misses, 1)
		return "", err
	}

	atomic.AddUint64(&c.hits, 1)
	return val, nil
}

// Set stores a value in the cache with TTL
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// GetStruct retrieves a JSON-serialized struct from the cache
func (c *Client) GetStruct(ctx context.Context, key string) (interface{}, error) {
	val, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached value: %w", err)
	}

	return result, nil
}

// SetStruct stores a JSON-serialized struct in the cache
func (c *Client) SetStruct(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return c.Set(ctx, key, string(data), ttl)
}

// Delete removes a value from the cache
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in the cache
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetHitRate returns the cache hit rate
func (c *Client) GetHitRate() float64 {
	hits := atomic.LoadUint64(&c.hits)
	misses := atomic.LoadUint64(&c.misses)

	total := hits + misses
	if total == 0 {
		return 0.0
	}

	return float64(hits) / float64(total)
}

// GetStats returns cache statistics
func (c *Client) GetStats() map[string]interface{} {
	hits := atomic.LoadUint64(&c.hits)
	misses := atomic.LoadUint64(&c.misses)
	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return map[string]interface{}{
		"hits":     hits,
		"misses":   misses,
		"total":    total,
		"hit_rate": hitRate,
	}
}

// Flush clears all keys in the cache (use with caution!)
func (c *Client) Flush(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

// SetMultiple stores multiple key-value pairs
func (c *Client) SetMultiple(ctx context.Context, pairs map[string]string, ttl time.Duration) error {
	pipe := c.client.Pipeline()

	for key, value := range pairs {
		pipe.Set(ctx, key, value, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetMultiple retrieves multiple values by keys
func (c *Client) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	pipe := c.client.Pipeline()

	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make(map[string]string)
	for i, cmd := range cmds {
		val, err := cmd.Result()
		if err == nil {
			result[keys[i]] = val
			atomic.AddUint64(&c.hits, 1)
		} else if err == redis.Nil {
			atomic.AddUint64(&c.misses, 1)
		}
	}

	return result, nil
}
