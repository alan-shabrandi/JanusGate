package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"janusgate/internal/config"

	"github.com/redis/go-redis/v9"
)

type redisRateLimiter struct {
	client *redis.Client
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
	}, nil
}

func (r *redisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return true, nil
}

func (r *redisRateLimiter) Close() error {
	slog.Info("Closing Redis connection pool...")
	return r.client.Close()
}
