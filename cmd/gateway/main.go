package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"janusgate/internal/auth"
	"janusgate/internal/config"
	"janusgate/internal/health"
	"janusgate/internal/middleware"
	"janusgate/internal/ratelimit"
	"janusgate/internal/router"
	"janusgate/internal/upstream"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, _, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	registry := upstream.NewRegistry()

	healthChecker := health.NewChecker(registry, 10*time.Second)
	healthChecker.RegisterRoutesUpstreams(cfg.Routes)
	healthChecker.Start(ctx)

	jwtMgr, err := auth.NewJWTManager(cfg.Auth.JWTSecret, "")
	if err != nil {
		slog.Error("Failed to initialize JWT Manager (check secret length)", "error", err)
		os.Exit(1)
	}

	redisLimiter, err := ratelimit.NewRedisLimiter(cfg.Redis)
	if err != nil {
		slog.Error("Failed to initialize Redis rate limiter", "error", err)
		os.Exit(1)
	}
	defer redisLimiter.Close()

	rt := router.NewRouter(cfg.Routes, jwtMgr)

	globalChain := middleware.New(
		middleware.Recovery,
		middleware.RequestID,
		middleware.Logger,
		middleware.RateLimit(redisLimiter, 60, time.Minute),
	)
	serverHandler := globalChain.Then(rt)

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           serverHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("Starting JanusGate API Gateway...", "addr", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGHUP:
			slog.Info("SIGHUP received, reloading configuration...")
			newCfg, _, err := config.Load("config.yaml")
			if err != nil {
				slog.Error("Failed to reload config (keeping old config)", "error", err)
				continue
			}

			if err := rt.LoadRoutes(newCfg.Routes); err != nil {
				slog.Error("Failed to apply new routes", "error", err)
				continue
			}

			healthChecker.RegisterRoutesUpstreams(newCfg.Routes)
			slog.Info("Configuration reloaded successfully")

		case syscall.SIGINT, syscall.SIGTERM:
			slog.Info("Shutdown signal received. Shutting down JanusGate gracefully...")
			cancel()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if err := server.Shutdown(shutdownCtx); err != nil {
				slog.Error("Server forced to shutdown", "error", err)
			}
			slog.Info("JanusGate stopped.")
			return
		}
	}
}
