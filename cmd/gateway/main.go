package main

import (
	"fmt"
	"log"

	"janusgate/internal/config"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting JanusGate API Gateway on port %d...\n", cfg.Server.Port)
	fmt.Printf("Loaded %d route(s) from config.yaml:\n", len(cfg.Routes))

	for _, route := range cfg.Routes {
		fmt.Printf(" - Route [%s] -> Path: %s | Upstreams: %d\n", route.ID, route.Path, len(route.Upstreams))
		for _, upstream := range route.Upstreams {
			fmt.Printf("     * Upstream [%s]: %s (Weight: %d)\n", upstream.ID, upstream.URL, upstream.Weight)
		}
	}
}
