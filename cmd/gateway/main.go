package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"janusgate/internal/config"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("=== JanusGate Configuration Loaded Successfully ===")
	fmt.Printf("Server Port: %d\n", cfg.Server.Port)
	fmt.Printf("Read Timeout: %s | Write Timeout: %s | Idle Timeout: %s\n",
		cfg.Server.ReadTimeout, cfg.Server.WriteTimeout, cfg.Server.IdleTimeout)
	fmt.Printf("Loaded Routes (%d):\n", len(cfg.Routes))
	for _, r := range cfg.Routes {
		fmt.Printf(" - [%s] Path: %s -> %d Upstream(s)\n", r.ID, r.Path, len(r.Upstreams))
	}
	fmt.Println("--------------------------------------------------")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("JanusGate is running!"))
	})

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		fmt.Printf("🚀 JanusGate HTTP Server listening on http://localhost%s\n", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down JanusGate server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("JanusGate server stopped gracefully.")
}
