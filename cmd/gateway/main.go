package main

import (
	"fmt"
	"log"

	"janusgate/internal/config"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Config loading failed: %v", err)
	}

	fmt.Println("=== JanusGate Configuration Loaded Successfully ===")
	fmt.Printf("Server Port: %d\n", cfg.Server.Port)
	fmt.Printf("Read Timeout: %s\n", cfg.Server.ReadTimeout)
	fmt.Printf("Write Timeout: %s\n", cfg.Server.WriteTimeout)
	fmt.Printf("Idle Timeout: %s\n", cfg.Server.IdleTimeout)
	fmt.Println("--------------------------------------------------")

	for i, r := range cfg.Routes {
		fmt.Printf("[%d] Route ID: %s | Path: %s | Methods: %v | StripPrefix: %t\n",
			i+1, r.ID, r.Path, r.Methods, r.StripPrefix)
		for _, u := range r.Upstreams {
			fmt.Printf("    ↳ Upstream ID: %s | URL: %s | Weight: %d\n", u.ID, u.URL, u.Weight)
		}
	}
}
