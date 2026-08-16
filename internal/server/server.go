package server

import (
	"context"
	"fmt"
	"log"
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

	slogHandler := slog.Default().Handler()
	httpErrorLogger := log.New(LogWriter{handler: slogHandler}, "", 0)

	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: 3 * time.Second,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			MaxHeaderBytes:    1 << 14,
			ErrorLog:          httpErrorLogger,
		},
	}
}

type LogWriter struct {
	handler slog.Handler
}

func (w LogWriter) Write(p []byte) (n int, err error) {
	slog.Error("net/http internal error", "details", string(p))
	return len(p), nil
}

func (s *Server) Start() error {
	slog.Info("JanusGate HTTP Server listening", "addr", s.httpServer.Addr)

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
