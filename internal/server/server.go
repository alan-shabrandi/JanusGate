package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"janusgate/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg *config.ServerConfig, handler http.Handler) *Server {
	addr := fmt.Sprintf(":%d", cfg.Port)

	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: 3 * time.Second,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

func (s *Server) Start() error {
	slog.Info("🚀 JanusGate HTTP Server listening", "addr", s.httpServer.Addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Stopping HTTP server gracefully... (waiting for active requests to finish)")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	slog.Info("HTTP server stopped cleanly.")
	return nil
}
