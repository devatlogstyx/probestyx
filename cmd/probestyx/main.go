package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/handlers"

	"gopkg.in/yaml.v3"
)

var version = "dev" // Will be overridden during build

func main() {
	// Add version flag
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Probestyx version %s\n", version)
		os.Exit(0)
	}
	// Load config
	configFile := "config.yaml"
	args := flag.Args()
	if len(args) > 0 {
		configFile = args[0]
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	// Validate config
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9100
	}

	// Initialize handlers with config
	handlers.Init(&cfg)

	// Start server
	http.HandleFunc("/metrics", handlers.MetricsHandler)
	http.HandleFunc("/health", handlers.HealthHandler)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Probestyx starting on %s", addr)
	if cfg.Server.Secret != "" {
		log.Printf("Authentication enabled with secret key")
	} else {
		log.Printf("Running without authentication (no secret key configured)")
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}