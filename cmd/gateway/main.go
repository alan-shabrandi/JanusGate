package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"janusgate/internal/auth"
	"janusgate/internal/config"
	"janusgate/internal/middleware"
	"janusgate/internal/ratelimit"
	"janusgate/internal/router"
)

func main() {
	cfg, v, err := config.LoadConfig("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret)

	redisLimiter, err := ratelimit.NewRedisLimiter(cfg.Redis)
	if err != nil {
		slog.Error("Failed to initialize Redis rate limiter", "error", err)
		os.Exit(1)
	}
	defer redisLimiter.Close()

	rt := router.NewRouter(cfg.Routes, jwtMgr)

	config.WatchChanges(v, func(newCfg *config.Config) {
		slog.Info("Config changed, reloading routes...")
		if err := rt.LoadRoutes(newCfg.Routes); err != nil {
			slog.Error("Failed to reload routes", "error", err)
		}
	})

	globalChain := middleware.New(
		middleware.Recovery,
		middleware.RequestID,
		middleware.Logger,
		middleware.RateLimit(redisLimiter, 60, time.Minute),
	)

	serverHandler := globalChain.Then(rt)

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	slog.Info("Starting JanusGate API Gateway...", "addr", serverAddr)

	if err := http.ListenAndServe(serverAddr, serverHandler); err != nil {
		slog.Error("Server error", "error", err)
	}
}
