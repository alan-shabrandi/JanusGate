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

	fmt.Printf("Starting JanusGate API Gateway on port %s [%s]...\n", cfg.Port, cfg.Env)
}
