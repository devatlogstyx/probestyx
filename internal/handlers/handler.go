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

// resolveConsumerID maps the request's ?callerId= to the state bucket format: log
// scrapers should tail from. An empty callerId (the common case) always maps to
// "default". A non-empty callerId is only honored if it's on the server.consumers
// allowlist - without that allowlist configured, per-consumer tracking is off and
// arbitrary caller-supplied IDs are ignored, so an unauthenticated caller can't
// grow the tailing-state map by making up new IDs.
func resolveConsumerID(r *http.Request) (string, bool) {
	callerID := r.URL.Query().Get("callerId")
	if callerID == "" {
		return "default", true
	}

	if len(cfg.Server.Consumers) == 0 {
		return "default", true
	}

	for _, allowed := range cfg.Server.Consumers {
		if allowed == callerID {
			return callerID, true
		}
	}

	return "", false
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

	consumerID, ok := resolveConsumerID(r)
	if !ok {
		http.Error(w, "unknown callerId", http.StatusBadRequest)
		return
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
			
			scraperMetrics, err := metrics.CollectScraper(s, consumerID)
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