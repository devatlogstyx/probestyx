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
		for k, v := range sysMetrics {
			result[k] = v
		}
	}

	// Collect from scrapers
	for _, scraper := range cfg.Scrapers {
		scraperMetrics, err := metrics.CollectScraper(scraper)
		if err != nil {
			log.Printf("Error collecting from %s: %v", scraper.Name, err)
			result[scraper.Name+"_error"] = err.Error()
			continue
		}

		// Merge metrics (last one wins)
		for k, v := range scraperMetrics {
			if _, exists := result[k]; exists {
				log.Printf("WARN: Metric '%s' from scraper '%s' overwrites previous value", k, scraper.Name)
			}
			result[k] = v
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}