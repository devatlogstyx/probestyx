package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

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
	var mu sync.Mutex // Protect result map from concurrent writes

	// Collect system metrics
	if cfg.System.Enabled {
		sysMetrics := metrics.CollectSystem()
		systemName := cfg.System.Name
		if systemName == "" {
			systemName = "system"
		}
		result[systemName] = sysMetrics
	}

	// Collect from scrapers in parallel
	var wg sync.WaitGroup
	for _, scraper := range cfg.Scrapers {
		wg.Add(1)
		
		// Capture scraper in closure
		go func(s config.ScraperConfig) {
			defer wg.Done()
			
			scraperMetrics, err := metrics.CollectScraper(s)
			if err != nil {
				log.Printf("Error collecting from %s: %v (skipping)", s.Name, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()
			
			// Check if this scraper name already exists
			if _, exists := result[s.Name]; exists {
				log.Printf("WARN: Scraper name '%s' already exists, overwriting previous value", s.Name)
			}
			
			result[s.Name] = scraperMetrics
		}(scraper)
	}

	wg.Wait() // Wait for all scrapers to complete

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}