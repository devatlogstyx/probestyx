package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/devatlogstyx/probestyx/internal/auth"
	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/metrics"
)

var cfg *config.Config

func Init(c *config.Config) {
	cfg = c
	auth.Init(c)
	metrics.Init(c)
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	// Log who is requesting metrics
	log.Printf("Metrics request from %s - User-Agent: %s", r.RemoteAddr, r.UserAgent())
	
	// Validate signature only if secret is configured
	if cfg.Server.Secret != "" {
		if !auth.ValidateSignature(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	result := make(map[string]interface{})

	// Collect system metrics
	if cfg.System.Enabled {
		sysMetrics := metrics.CollectSystem()
		// Use the configured name or default to "system"
		systemName := cfg.System.Name
		if systemName == "" {
			systemName = "system"
		}
		result[systemName] = sysMetrics
	}

	// Collect from scrapers
	for _, scraper := range cfg.Scrapers {
		scraperMetrics, err := metrics.CollectScraper(scraper)
		if err != nil {
			log.Printf("Error collecting from %s: %v (skipping)", scraper.Name, err)
			continue
		}

		// Check if this scraper name already exists
		if _, exists := result[scraper.Name]; exists {
			log.Printf("WARN: Scraper name '%s' already exists, overwriting previous value", scraper.Name)
		}
		
		result[scraper.Name] = scraperMetrics
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}