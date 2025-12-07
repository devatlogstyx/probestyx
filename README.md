# Probestyx

A flexible metrics collection and aggregation tool that can scrape metrics from various sources and formats.

## Features

- **System Metrics**: Collect CPU, RAM, and disk usage
- **Multiple Source Types**: URL (HTTP) and local file sources
- **Multiple Format Support**: JSON, Prometheus, and raw text parsing
- **Flexible Metric Mapping**: Extract and transform metrics with calculations
- **Pattern Filtering**: Include/exclude metrics using regex patterns
- **Optional Authentication**: HMAC-based request signing (optional)

## Installation

```bash
go build -o probestyx ./cmd/probestyx
```

## Usage

```bash
# Run with default config (config.yaml)
./probestyx

# Run with custom config
./probestyx /path/to/config.yaml
```

## Configuration

### Basic Structure

```yaml
server:
  port: 9100
  secret: "optional-secret-key"  # Leave empty for no auth

system:
  enabled: true
  metrics:
    - cpu
    - ram
    - disk

scrapers:
  - name: scraper_name
    source:
      type: url|file
      url: "http://..."      # for type: url
      path: "/path/to/file"  # for type: file
      format: json|prometheus|raw
      pattern: "regex"       # for format: raw
    metrics:
      - path: "json.path"    # for JSON
        match: "metric_name" # for Prometheus/raw
        name: "output_name"
        calculate: "value * 100"  # optional transformation
    filter:                  # optional
      include:
        - "pattern.*"
      exclude:
        - ".*internal.*"
```

## Supported Formats

### 1. JSON Format

Extracts values from JSON using dot notation paths.

**Config Example:**
```yaml
- name: api_metrics
  source:
    type: url
    url: "https://api.example.com/metrics"
    format: json
  metrics:
    - path: "database.connections"
      name: "db_connections"
    - path: "cache.hit_rate"
      name: "cache_hit_rate"
      calculate: "value * 100"
```

**Source Data:**
```json
{
  "database": {
    "connections": 42
  },
  "cache": {
    "hit_rate": 0.95
  }
}
```

### 2. Prometheus Format

Parses Prometheus exposition format metrics.

**Config Example:**
```yaml
- name: node_exporter
  source:
    type: url
    url: "http://localhost:9100/metrics"
    format: prometheus
  metrics:
    - match: "node_cpu_seconds_total"
      name: "cpu_seconds"
    - match: "node_memory_MemAvailable_bytes"
      name: "memory_mb"
      calculate: "value / 1024 / 1024"
```

**Source Data:**
```
# TYPE node_cpu_seconds_total counter
node_cpu_seconds_total{cpu="0",mode="idle"} 12345.67

# TYPE node_memory_MemAvailable_bytes gauge
node_memory_MemAvailable_bytes 4294967296
```

### 3. Raw Format

Uses regex patterns to extract key-value pairs from text.

**Config Example:**
```yaml
- name: sensors
  source:
    type: file
    path: "/data/sensors.txt"
    format: raw
    pattern: 'sensor_(\w+)=([0-9.]+)'  # optional, default: (\w+)=(\S+)
  metrics:
    - match: "temperature"
      name: "temp_celsius"
    - match: "humidity"
      name: "humidity_percent"
```

**Source Data:**
```
sensor_temperature=23.5
sensor_humidity=65.2
sensor_pressure=1013.25
```

## Calculations

Transform metric values using simple expressions:

- `value * 2` - Multiplication
- `value / 1024` - Division
- `value + 10` - Addition
- `value - 5` - Subtraction

**Examples:**
```yaml
# Convert bytes to megabytes
calculate: "value / 1024 / 1024"

# Convert ratio to percentage
calculate: "value * 100"

# Convert seconds to milliseconds
calculate: "value * 1000"
```

## Filters

Include or exclude metrics using regex patterns:

```yaml
filter:
  include:
    - "^node_.*"      # Only metrics starting with "node_"
    - ".*memory.*"    # Metrics containing "memory"
  exclude:
    - ".*_bucket$"    # Exclude histogram buckets
    - ".*internal.*"  # Exclude internal metrics
```

## Authentication

Optional HMAC-SHA256 based authentication. If `secret` is not set, authentication is disabled.

### Enable Authentication

```yaml
server:
  port: 9100
  secret: "your-secret-key"
```

### Client Request

```bash
timestamp=$(date +%s)
signature=$(echo -n "$timestamp" | openssl dgst -sha256 -hmac "your-secret-key" | cut -d' ' -f2)

curl -H "X-Timestamp: $timestamp" \
     -H "X-Signature: $signature" \
     http://localhost:9100/metrics
```

### Disable Authentication

Simply leave `secret` empty or remove it:

```yaml
server:
  port: 9100
  # No secret = no authentication required
```

## Endpoints

- `GET /metrics` - Returns all collected metrics as JSON
- `GET /health` - Health check endpoint (always returns "OK")

## Example Response

```json
{
  "cpu": 45.2,
  "ram": 67.8,
  "disk": 52.3,
  "db_connections": 42,
  "cache_hit_rate": 95,
  "temperature_celsius": 23.5,
  "memory_mb": 4096
}
```

## Examples Directory

See the `examples/` directory for:
- `config.simple.yaml` - Simple configuration without auth
- `stats.json` - Example JSON data
- `metrics.prom` - Example Prometheus data
- `sensors.txt` - Example raw text data

## Testing

```bash
# Start the server
./probestyx examples/config-simple.yaml

# Query metrics (no auth needed with simple config)
curl http://localhost:9100/metrics

# Check health
curl http://localhost:9100/health
```

## License

MIT