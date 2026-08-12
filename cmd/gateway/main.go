package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"janusgate/internal/config"
	"janusgate/internal/proxy"
	"janusgate/internal/server"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mux := http.NewServeMux()

	offlineTarget := "http://localhost:9999"
	reverseProxy, err := proxy.NewProxy(offlineTarget)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	mux.Handle("/test-down", reverseProxy)

	srv := server.NewServer(&cfg.Server, mux)
	srv.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	_ = srv.Shutdown(10 * time.Second)
}
