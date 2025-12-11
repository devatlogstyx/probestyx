package metrics

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/parsers"
	"github.com/devatlogstyx/probestyx/internal/utils"
)

func CollectScraper(scraper config.ScraperConfig) (map[string]interface{}, error) {
	var rawData string
	var err error

	// Fetch data based on source type
	switch scraper.Source.Type {
	case "url":
		rawData, err = fetchURL(scraper.Source.URL)
	case "file":
		data, e := os.ReadFile(scraper.Source.Path)
		rawData = string(data)
		err = e
	default:
		return nil, fmt.Errorf("unknown source type: %s", scraper.Source.Type)
	}

	if err != nil {
		return nil, err
	}

	// Parse based on format
	var parsed map[string]interface{}
	switch scraper.Source.Format {
	case "json":
		parsed, err = parsers.ParseJSON(rawData)
	case "prometheus":
		parsed, err = parsers.ParsePrometheus(rawData)
	case "raw":
		parsed, err = parsers.ParseRaw(rawData, scraper.Source.Pattern)
	default:
		return nil, fmt.Errorf("unknown format: %s", scraper.Source.Format)
	}

	if err != nil {
		return nil, err
	}

	// Apply filters if specified
	if scraper.Filter != nil {
		parsed = parsers.ApplyFilters(parsed, scraper.Filter)
	}

	// Map and transform metrics
	result := make(map[string]interface{})
	for _, metricMap := range scraper.Metrics {
		var value interface{}
		var found bool

		if metricMap.Path != "" {
			// JSON path lookup
			value, found = utils.GetJSONPath(parsed, metricMap.Path)
		} else if metricMap.Match != "" {
			// Pattern match
			value, found = parsed[metricMap.Match]
		}

		if !found {
			continue
		}

		// Apply calculation if specified
		if metricMap.Calculate != "" {
			if numVal, ok := utils.ToFloat64(value); ok {
				value = utils.Calculate(numVal, metricMap.Calculate)
			}
		}

		result[metricMap.Name] = value
	}

	return result, nil
}

func fetchURL(url string) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}