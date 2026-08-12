package main

import (
	"fmt"
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

	targetURL := "https://httpbin.org"
	reverseProxy, err := proxy.NewProxy(targetURL)
	if err != nil {
		log.Fatalf("Failed to create reverse proxy: %v", err)
	}

	mux.Handle("/users", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("--> Proxying request [%s %s] to %s\n", r.Method, r.URL.Path, targetURL)
		reverseProxy.ServeHTTP(w, r)
	}))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	srv := server.NewServer(&cfg.Server, mux)
	srv.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := srv.Shutdown(10 * time.Second); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}
}
