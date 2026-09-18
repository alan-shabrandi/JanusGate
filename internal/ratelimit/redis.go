package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"janusgate/internal/config"

	"github.com/redis/go-redis/v9"
)

//nolint:gosec // False positive
const tokenBucketLuaScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window_millis = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])

-- Use Redis server time to prevent application clock skew
local redis_time = redis.call("TIME")
local now = (tonumber(redis_time[1]) * 1000) + math.floor(tonumber(redis_time[2]) / 1000)

local state = redis.call("HMGET", key, "tokens", "last_updated")
local tokens = tonumber(state[1])
local last_updated = tonumber(state[2])

if tokens == nil then
    tokens = limit
    last_updated = now
else
    local delta = math.max(0, now - last_updated)
    local refill_rate = limit / window_millis
    tokens = math.min(limit, tokens + (delta * refill_rate))
end

local allowed = 0
if tokens >= requested then
    tokens = tokens - requested
    allowed = 1
end

-- Use tostring() to prevent Redis float conversion errors
redis.call("HSET", key, "tokens", tostring(tokens), "last_updated", tostring(now))
redis.call("PEXPIRE", key, math.ceil(window_millis * 2))

return {allowed, math.floor(tokens)}
`

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
	Close() error
}

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
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolSize:     100,
		MinIdleConns: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection ping failed: %w", err)
	}

	slog.Info("Connected to Redis successfully for Rate Limiting", "addr", cfg.Addr, "db", cfg.DB)

	return &redisRateLimiter{
		client: client,
		script: redis.NewScript(tokenBucketLuaScript),
	}, nil
}

func (r *redisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	if limit <= 0 {
		return true, limit, nil
	}

	windowMillis := window.Milliseconds()
	if windowMillis <= 0 {
		windowMillis = 1000
	}

	redisKey := fmt.Sprintf("ratelimit:%s", key)

	// Removed nowMillis from ARGV
	res, err := r.script.Run(ctx, r.client, []string{redisKey}, limit, windowMillis, 1).Result()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, 0, err
		}
		// Return the error to the middleware so it can decide to fail-open
		return false, 0, fmt.Errorf("redis script execution failed: %w", err)
	}

	results, ok := res.([]interface{})
	if !ok || len(results) < 2 {
		return false, 0, errors.New("invalid response format from redis lua script")
	}

	allowedInt, ok1 := results[0].(int64)
	remainingInt, ok2 := results[1].(int64)

	if !ok1 || !ok2 {
		return false, 0, errors.New("unexpected type in redis response")
	}

	return allowedInt == 1, int(remainingInt), nil
}

func (r *redisRateLimiter) Close() error {
	slog.Info("Closing Redis connection pool...")
	return r.client.Close()
}
