package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func GetJSONPath(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, false
			}
		default:
			return nil, false
		}
	}

	return current, true
}

func Calculate(value float64, expr string) float64 {
	// Simple expression parser for basic operations
	expr = strings.ReplaceAll(expr, "value", fmt.Sprintf("%f", value))
	expr = strings.TrimSpace(expr)

	// Handle simple operations: value * X, value / X, value + X, value - X
	if strings.Contains(expr, "*") {
		parts := strings.Split(expr, "*")
		if len(parts) == 2 {
			if multiplier, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return value * multiplier
			}
		}
	} else if strings.Contains(expr, "/") {
		parts := strings.Split(expr, "/")
		if len(parts) == 2 {
			if divisor, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return value / divisor
			}
		}
	} else if strings.Contains(expr, "+") {
		parts := strings.Split(expr, "+")
		if len(parts) == 2 {
			if addend, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return value + addend
			}
		}
	} else if strings.Contains(expr, "-") {
		parts := strings.Split(expr, "-")
		if len(parts) == 2 {
			if subtrahend, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				return value - subtrahend
			}
		}
	}

	return value
}

func ToFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func Round(val float64, precision int) float64 {
	ratio := float64(1)
	for i := 0; i < precision; i++ {
		ratio *= 10
	}
	return float64(int(val*ratio+0.5)) / ratio
}