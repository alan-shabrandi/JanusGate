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
	"janusgate/internal/router"
	"janusgate/internal/server"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rt := router.NewRouter()

	for _, route := range cfg.Routes {
		if len(route.Upstreams) == 0 {
			continue
		}

		targetURL := route.Upstreams[0].URL
		revProxy, err := proxy.NewProxy(targetURL)
		if err != nil {
			log.Fatalf("Failed to create proxy for route %s: %v", route.ID, err)
		}

		var handler http.Handler = revProxy

		if route.StripPrefix {
			handler = proxy.StripPrefix(route.Path, revProxy)
		}

		if err := rt.AddRoute(route, handler); err != nil {
			log.Fatalf("Failed to register route %s: %v", route.ID, err)
		}

		log.Printf("Registered Route [%s]: %s -> %s (StripPrefix: %t)",
			route.ID, route.Path, targetURL, route.StripPrefix)
	}

	srv := server.NewServer(&cfg.Server, rt)
	srv.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	_ = srv.Shutdown(10 * time.Second)
}
