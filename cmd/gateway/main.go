package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"janusgate/internal/config"
	"janusgate/internal/router"
	"janusgate/internal/server"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rt := router.NewRouter()
	if err := rt.LoadRoutes(cfg.Routes); err != nil {
		log.Fatalf("Failed to load routes into router: %v", err)
	}

	srv := server.NewServer(&cfg.Server, rt)
	srv.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	_ = srv.Shutdown(10 * time.Second)
}
