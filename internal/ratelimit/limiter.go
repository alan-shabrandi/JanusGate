package ratelimit

import (
	"context"
	"time"
)

type Limiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, time.Duration, error)
}
