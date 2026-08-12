package main

import (
	"fmt"
	"log"

	"janusgate/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting JanusGate API Gateway on port %d...\n", cfg.Server.Port)
	fmt.Printf("Loaded %d route(s):\n", len(cfg.Routes))

	for _, route := range cfg.Routes {
		fmt.Printf(" - Route ID: %s | Path: %s | Upstreams: %d\n", route.ID, route.Path, len(route.Upstreams))
	}
}
