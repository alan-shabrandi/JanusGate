package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"janusgate/internal/config"

	"github.com/redis/go-redis/v9"
)

const tokenBucketLuaScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local state = redis.call("HMGET", key, "tokens", "last_updated")
local tokens = tonumber(state[1])
local last_updated = tonumber(state[2])

if tokens == nil then
    tokens = limit
    last_updated = now
else
    local delta = math.max(0, now - last_updated)
    tokens = math.min(limit, tokens + (delta * refill_rate))
end

if tokens >= requested then
    tokens = tokens - requested
    last_updated = now
    redis.call("HMSET", key, "tokens", tokens, "last_updated", last_updated)
    
    local ttl = math.ceil(limit / refill_rate) * 2
    redis.call("EXPIRE", key, ttl)
    return {1, math.floor(tokens)}
else
    redis.call("HMSET", key, "tokens", tokens, "last_updated", last_updated)
    local ttl = math.ceil(limit / refill_rate) * 2
    redis.call("EXPIRE", key, ttl)
    return {0, math.floor(tokens)}
end
`

type redisRateLimiter struct {
	client *redis.Client
	script *redis.Script
}

func NewRedisLimiter(cfg config.RedisConfig) (RateLimiter, error) {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection ping failed: %w", err)
	}

	slog.Info("Connected to Redis successfully", "addr", cfg.Addr, "db", cfg.DB)

	return &redisRateLimiter{
		client: client,
		script: redis.NewScript(tokenBucketLuaScript),
	}, nil
}

func (r *redisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}

	refillRate := float64(limit) / window.Seconds()
	now := time.Now().Unix()

	redisKey := fmt.Sprintf("ratelimit:%s", key)

	res, err := r.script.Run(ctx, r.client, []string{redisKey}, limit, refillRate, now, 1).Result()
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit lua script: %w", err)
	}

	results, ok := res.([]interface{})
	if !ok || len(results) < 1 {
		return false, fmt.Errorf("invalid response format from redis lua script")
	}

	allowed := results[0].(int64) == 1
	return allowed, nil
}

func (r *redisRateLimiter) Close() error {
	slog.Info("Closing Redis connection pool...")
	return r.client.Close()
}
