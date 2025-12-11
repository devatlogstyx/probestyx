# Probestyx

A flexible metrics collection and aggregation tool that can scrape metrics from various sources and formats.

## Features

- **System Metrics**: Collect CPU, RAM, disk, network, and process metrics
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
    # CPU Metrics
    - cpu_usage_percent
    - cpu_usage_per_core
    - cpu_count
    - cpu_count_physical
    - cpu_load_1min
    - cpu_load_5min
    - cpu_load_15min
    
    # Memory Metrics
    - ram_usage_percent
    - available_ram_mb
    - total_ram_mb
    - ram_cached_mb
    - ram_buffers_mb
    - swap_usage_percent
    - swap_total_mb
    - swap_used_mb
    
    # Disk Metrics
    - disk_usage_percent
    - available_disk_gb
    - total_disk_gb
    - inode_usage_percent
    - disk_read_bytes
    - disk_write_bytes
    - disk_read_bytes_per_sec
    - disk_write_bytes_per_sec
    - disk_read_count
    - disk_write_count
    
    # Network Metrics
    - network_bytes_sent
    - network_bytes_recv
    - network_bytes_sent_per_sec
    - network_bytes_recv_per_sec
    - network_packets_sent
    - network_packets_recv
    - network_errors_in
    - network_errors_out
    - active_connections
    
    # System Info
    - system_uptime_seconds
    - boot_time_unix
    - os_platform
    - os_version
    - hostname
    - kernel_version
    - process_count

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

## System Metrics Reference

### CPU Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `cpu_usage_percent` | Overall CPU usage | Percentage (0-100) |
| `cpu_usage_per_core` | Per-core CPU usage | Array of percentages |
| `cpu_count` | Number of logical CPU cores | Count |
| `cpu_count_physical` | Number of physical CPU cores | Count |
| `cpu_load_1min` | 1-minute load average | Load |
| `cpu_load_5min` | 5-minute load average | Load |
| `cpu_load_15min` | 15-minute load average | Load |

### Memory Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `ram_usage_percent` | RAM usage percentage | Percentage (0-100) |
| `available_ram_mb` | Available RAM | Megabytes |
| `total_ram_mb` | Total RAM | Megabytes |
| `ram_cached_mb` | RAM used for caching | Megabytes |
| `ram_buffers_mb` | RAM used for buffers | Megabytes |
| `swap_usage_percent` | Swap usage percentage | Percentage (0-100) |
| `swap_total_mb` | Total swap space | Megabytes |
| `swap_used_mb` | Used swap space | Megabytes |

### Disk Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `disk_usage_percent` | Disk usage percentage | Percentage (0-100) |
| `available_disk_gb` | Available disk space | Gigabytes |
| `total_disk_gb` | Total disk space | Gigabytes |
| `inode_usage_percent` | Inode usage percentage | Percentage (0-100) |
| `disk_read_bytes` | Cumulative bytes read | Bytes |
| `disk_write_bytes` | Cumulative bytes written | Bytes |
| `disk_read_bytes_per_sec` | Disk read rate | Bytes/second |
| `disk_write_bytes_per_sec` | Disk write rate | Bytes/second |
| `disk_read_count` | Total read operations | Count |
| `disk_write_count` | Total write operations | Count |

### Network Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `network_bytes_sent` | Cumulative bytes sent | Bytes |
| `network_bytes_recv` | Cumulative bytes received | Bytes |
| `network_bytes_sent_per_sec` | Network send rate | Bytes/second |
| `network_bytes_recv_per_sec` | Network receive rate | Bytes/second |
| `network_packets_sent` | Total packets sent | Count |
| `network_packets_recv` | Total packets received | Count |
| `network_errors_in` | Inbound network errors | Count |
| `network_errors_out` | Outbound network errors | Count |
| `active_connections` | Active network connections | Count |

### System Information

| Metric | Description | Type |
|--------|-------------|------|
| `system_uptime_seconds` | System uptime | Seconds |
| `boot_time_unix` | System boot time | Unix timestamp |
| `os_platform` | Operating system platform | String |
| `os_version` | OS version | String |
| `hostname` | System hostname | String |
| `kernel_version` | Kernel version | String |
| `process_count` | Number of running processes | Count |

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
  "cpu_usage_percent": 45.2,
  "cpu_count": 8,
  "cpu_load_1min": 2.5,
  "ram_usage_percent": 67.8,
  "available_ram_mb": 8192.5,
  "disk_usage_percent": 52.3,
  "disk_read_bytes_per_sec": 1048576,
  "disk_write_bytes_per_sec": 524288,
  "network_bytes_sent_per_sec": 102400,
  "network_bytes_recv_per_sec": 204800,
  "active_connections": 42,
  "system_uptime_seconds": 86400,
  "hostname": "web-server-01",
  "process_count": 156,
  "db_connections": 42,
  "cache_hit_rate": 95
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

## Running in Production

### Systemd Service (Linux)

Create a systemd service file to run Probestyx as a background service with auto-restart.

1. Create the service file:

```bash
sudo nano /etc/systemd/system/probestyx.service
```

2. Add the following configuration:

```ini
[Unit]
Description=Probestyx Metrics Collection Service
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=probestyx
Group=probestyx
WorkingDirectory=/opt/probestyx
ExecStart=/opt/probestyx/probestyx /opt/probestyx/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=probestyx

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/probestyx

[Install]
WantedBy=multi-user.target
```

3. Set up the application:

```bash
# Create user for running the service
sudo useradd -r -s /bin/false probestyx

# Create directory and copy files
sudo mkdir -p /opt/probestyx
sudo cp probestyx /opt/probestyx/
sudo cp config.yaml /opt/probestyx/
sudo chown -R probestyx:probestyx /opt/probestyx
sudo chmod +x /opt/probestyx/probestyx
```

4. Enable and start the service:

```bash
# Reload systemd to recognize the new service
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable probestyx

# Start the service
sudo systemctl start probestyx

# Check service status
sudo systemctl status probestyx

# View logs
sudo journalctl -u probestyx -f
```

5. Manage the service:

```bash
# Stop the service
sudo systemctl stop probestyx

# Restart the service
sudo systemctl restart probestyx

# Disable auto-start
sudo systemctl disable probestyx

# View recent logs
sudo journalctl -u probestyx -n 100 --no-pager
```

### Docker Container

Run Probestyx in a Docker container with auto-restart:

1. Create a Dockerfile:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o probestyx ./cmd/probestyx

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/probestyx .
COPY config.yaml .
EXPOSE 9100
CMD ["./probestyx"]
```

2. Build and run:

```bash
# Build the image
docker build -t probestyx .

# Run with auto-restart
docker run -d \
  --name probestyx \
  --restart unless-stopped \
  -p 9100:9100 \
  -v $(pwd)/config.yaml:/root/config.yaml:ro \
  probestyx

# View logs
docker logs -f probestyx

# Stop container
docker stop probestyx

# Start container
docker start probestyx
```

### Docker Compose

Create a `docker-compose.yml`:

```yaml
version: '3.8'

services:
  probestyx:
    build: .
    container_name: probestyx
    restart: unless-stopped
    ports:
      - "9100:9100"
    volumes:
      - ./config.yaml:/root/config.yaml:ro
    environment:
      - TZ=UTC
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

Run with:

```bash
docker-compose up -d
docker-compose logs -f
docker-compose down
```

### Supervisor (Alternative)

For systems without systemd, use Supervisor:

1. Install Supervisor:

```bash
sudo apt-get install supervisor  # Debian/Ubuntu
sudo yum install supervisor      # CentOS/RHEL
```

2. Create configuration file `/etc/supervisor/conf.d/probestyx.conf`:

```ini
[program:probestyx]
command=/opt/probestyx/probestyx /opt/probestyx/config.yaml
directory=/opt/probestyx
user=probestyx
autostart=true
autorestart=true
redirect_stderr=true
stdout_logfile=/var/log/probestyx.log
stdout_logfile_maxbytes=10MB
stdout_logfile_backups=3
```

3. Manage with Supervisor:

```bash
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start probestyx
sudo supervisorctl status probestyx
```

### Windows Service

For Windows, use NSSM (Non-Sucking Service Manager):

1. Download NSSM from https://nssm.cc/download

2. Install as service:

```cmd
nssm install Probestyx "C:\probestyx\probestyx.exe" "C:\probestyx\config.yaml"
nssm set Probestyx AppDirectory "C:\probestyx"
nssm set Probestyx DisplayName "Probestyx Metrics Collection"
nssm set Probestyx Description "Collects and aggregates system and application metrics"
nssm set Probestyx Start SERVICE_AUTO_START
nssm start Probestyx
```

3. Manage the service:

```cmd
nssm status Probestyx
nssm restart Probestyx
nssm stop Probestyx
nssm remove Probestyx confirm
```

## License

MIT