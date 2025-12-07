package parsers

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/devatlogstyx/probestyx/internal/config"
)

func ParseJSON(data string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(data), &result)
	return result, err
}

func ParsePrometheus(data string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Simple parsing: metric_name{labels} value
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		metricName := parts[0]
		// Remove labels for simplicity (you can enhance this)
		if idx := strings.Index(metricName, "{"); idx != -1 {
			metricName = metricName[:idx]
		}

		if val, err := strconv.ParseFloat(parts[1], 64); err == nil {
			result[metricName] = val
		}
	}

	return result, nil
}

func ParseRaw(data string, pattern string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	if pattern == "" {
		pattern = `(\w+)=(\S+)`
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	matches := re.FindAllStringSubmatch(data, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]

			// Try to parse as number
			if numVal, err := strconv.ParseFloat(value, 64); err == nil {
				result[key] = numVal
			} else {
				result[key] = value
			}
		}
	}

	return result, nil
}

func ApplyFilters(data map[string]interface{}, filter *config.FilterConfig) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		include := len(filter.Include) == 0
		for _, pattern := range filter.Include {
			if matched, _ := regexp.MatchString(pattern, key); matched {
				include = true
				break
			}
		}

		if !include {
			continue
		}

		exclude := false
		for _, pattern := range filter.Exclude {
			if matched, _ := regexp.MatchString(pattern, key); matched {
				exclude = true
				break
			}
		}

		if !exclude {
			result[key] = value
		}
	}

	return result
}