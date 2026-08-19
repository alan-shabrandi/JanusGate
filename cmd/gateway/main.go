package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	//nolint:gosec // Profiling endpoint enabled intentionally for gateway debugging
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"janusgate/internal/auth"
	"janusgate/internal/config"
	"janusgate/internal/health"
	"janusgate/internal/metrics"
	"janusgate/internal/middleware"
	"janusgate/internal/ratelimit"
	"janusgate/internal/router"
	"janusgate/internal/upstream"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pprofAddr := "localhost:6060"
	pprofServer := &http.Server{
		Addr:              pprofAddr,
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		slog.Info("Starting Internal Profiling Server (pprof)...", "addr", pprofAddr)
		if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("Internal pprof server stopped unexpectedly", "error", err)
		}
	}()

	cfg, mgr, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	holder := config.NewHolder(cfg)
	registry := upstream.NewRegistry()

	m := metrics.Init(nil)

	metricsServer := metrics.NewServer(9090, nil)
	metricsServer.Start()

	healthChecker := health.NewChecker(registry, 10*time.Second)
	healthChecker.RegisterRoutesUpstreams(cfg.Routes)
	healthChecker.Start(ctx)

	jwtMgr, err := auth.NewJWTManager(cfg.Auth.JWTSecret, "")
	if err != nil {
		slog.Error("Failed to initialize JWT Manager", "error", err)
		os.Exit(1)
	}

	redisLimiter, err := ratelimit.NewRedisLimiter(cfg.Redis)
	if err != nil {
		slog.Error("Failed to initialize Redis rate limiter", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = redisLimiter.Close()
	}()

	rt := router.NewRouter(cfg.Routes, jwtMgr, registry)

	watcher, err := config.NewBackgroundWatcher("config.yaml", mgr, holder)
	if err != nil {
		slog.Warn("Failed to initialize background config watcher", "error", err)
	} else {
		err = watcher.Start(ctx, func(newCfg *config.Config) {
			if err := rt.LoadRoutes(newCfg.Routes); err != nil {
				slog.Error("Failed to apply routes on hot reload", "error", err)
				return
			}
			healthChecker.RegisterRoutesUpstreams(newCfg.Routes)
			slog.Info("Routes and HealthChecker updated automatically via file watcher")
		})
		if err != nil {
			slog.Error("Failed to start background watcher", "error", err)
		} else {
			defer func() {
				_ = watcher.Stop()
			}()
		}
	}

	globalChain := middleware.New(
		middleware.Recovery,
		middleware.RequestID(),
		middleware.Trace(),
		middleware.Logger,
		middleware.Metrics(m),
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
			newCfg, err := mgr.Reload()
			if err != nil {
				slog.Error("Failed to reload config on SIGHUP", "error", err)
				continue
			}

			holder.Update(newCfg)

			if err := rt.LoadRoutes(newCfg.Routes); err != nil {
				slog.Error("Failed to apply new routes on SIGHUP", "error", err)
				continue
			}

			healthChecker.RegisterRoutesUpstreams(newCfg.Routes)
			slog.Info("Configuration reloaded successfully via SIGHUP")

		case syscall.SIGINT, syscall.SIGTERM:
			slog.Info("Shutdown signal received. Shutting down JanusGate gracefully...")
			cancel()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			_ = pprofServer.Shutdown(shutdownCtx)
			_ = metricsServer.Shutdown(shutdownCtx)

			if err := server.Shutdown(shutdownCtx); err != nil {
				slog.Error("Server forced to shutdown", "error", err)
			}
			slog.Info("JanusGate stopped.")
			return
		}
	}
}
