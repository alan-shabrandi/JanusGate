package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"janusgate/internal/config"
	"janusgate/internal/middleware"
	"janusgate/internal/router"
	"janusgate/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, v, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err, "path", *configPath)
		os.Exit(1)
	}

	rt := router.NewRouter()
	if err := rt.LoadRoutes(cfg.Routes); err != nil {
		slog.Error("Failed to load routes into router", "error", err)
		os.Exit(1)
	}

	mwChain := middleware.New(
		middleware.RequestID,
		middleware.Logger,
	)

	handlerWithMiddleware := mwChain.Then(rt)

	config.WatchChanges(v, func(newCfg *config.Config) {
		slog.Info("Configuration file changed, reloading routes...")
		if err := rt.LoadRoutes(newCfg.Routes); err != nil {
			slog.Error("Failed to hot-reload routes", "error", err)
		} else {
			slog.Info("Routes hot-reloaded successfully!")
		}
	})

	srv := server.NewServer(&cfg.Server, handlerWithMiddleware)

	go func() {
		slog.Info("Starting JanusGate server", "port", cfg.Server.Port)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited successfully")
}
