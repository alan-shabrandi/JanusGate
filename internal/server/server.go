package server

import (
	"context"
	"fmt"
	"log"
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
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}
}

func (s *Server) Start() {
	go func() {
		log.Printf("🚀 JanusGate HTTP Server listening on http://localhost%s\n", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server HTTP ListenAndServe error: %v", err)
		}
	}()
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Println("Stopping HTTP server gracefully... (waiting for active requests to finish)")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("HTTP server stopped cleanly.")
	return nil
}
