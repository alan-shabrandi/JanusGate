package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	testcontainersredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestEnv struct {
	RedisAddr string
	redisC    *testcontainersredis.RedisContainer
}

func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// ۲. تغییر متد Wait Strategy
	redisContainer, err := testcontainersredis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis testcontainer: %v", err)
	}

	endpoint, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		_ = redisContainer.Terminate(context.Background())
		t.Fatalf("Failed to get Redis connection string: %v", err)
	}

	var hostPort string
	_, err = fmt.Sscanf(endpoint, "redis://%s", &hostPort)
	if err != nil {
		hostPort = endpoint
	}

	env := &TestEnv{
		RedisAddr: hostPort,
		redisC:    redisContainer,
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := redisContainer.Terminate(cleanupCtx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	})

	return env
}
