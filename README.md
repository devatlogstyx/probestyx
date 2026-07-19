# Probestyx

A flexible metrics collection and aggregation tool that can scrape metrics from various sources and formats.

## Features

- **System Metrics**: Collect CPU, RAM, disk, network, and process metrics
- **Multiple Source Types**: URL (HTTP) and local file sources
- **Multiple Format Support**: JSON, Prometheus, and raw text parsing
- **Flexible Metric Mapping**: Extract and transform metrics with calculations
- **Pattern Filtering**: Include/exclude metrics using regex patterns
- **Optional Authentication**: HMAC-based request signing (optional)

## Quick Installation

> **Important:** You must provide your own `config.yaml` file.

### Linux

```bash
# Download or create your config.yaml first
wget https://raw.githubusercontent.com/devatlogstyx/probestyx/main/examples/config.simple.yaml -O config.yaml

# Then install with your config
wget -O - https://raw.githubusercontent.com/devatlogstyx/probestyx/main/install.sh | sudo bash -s ./config.yaml
```

### macOS

```bash
# Download or create your config.yaml first
curl -O https://raw.githubusercontent.com/devatlogstyx/probestyx/main/examples/config.simple.yaml

# Then install with your config
curl -fsSL https://raw.githubusercontent.com/devatlogstyx/probestyx/main/install.sh | sudo bash -s ./config.yaml
```

### Windows (PowerShell as Administrator)

```powershell
# Download or create your config.yaml first
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/devatlogstyx/probestyx/main/examples/config.simple.yaml" -OutFile "config.yaml"

# Then install with your config
irm https://raw.githubusercontent.com/devatlogstyx/probestyx/main/install.ps1 | iex -ArgumentList ".\config.yaml"
```

**What the installation script does:**
- Downloads the latest release binary from GitHub
- Uses **your provided config.yaml**
- Installs to `/opt/probestyx` (Linux/macOS) or `C:\probestyx` (Windows)
- Sets up as a system service with auto-restart
- Starts the service automatically

## Configuration

### Basic Structure

```yaml
server:
  port: 9100
  secret: "optional-secret-key"  # Leave empty for no auth
  consumers: []                 # optional allowlist for ?callerId= (see Per-Consumer Tailing)

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
      type: url|file|command
      url: "http://..."        # for type: url
      path: "/path/to/file"    # for type: file
      command: "shell command" # for type: command
      format: json|prometheus|raw|log
      pattern: "regex"         # for format: raw/log
      max_lines: 100           # for format: log
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

### 3. Log Format

Scans a text source line-by-line and returns the lines that look like errors, plus a count. Unlike `raw` (which collapses matches into single key/value pairs), `log` is built for grepping error lines out of log files or command output — e.g. an nginx `error.log` file, or the output of `docker logs`.

- If `pattern` is set, it's used as a regex to test each line.
- If `pattern` is omitted, lines are matched case-insensitively against the word `error` (covers nginx's `[error]` and typical `Error: ...` app/Docker output).
- `max_lines` caps how many matching lines are returned per scrape (default 100).

Produces two values you can map: `count` and `lines`.

**Only new lines are reported each scrape** — not everything currently sitting in the log:

- `type: file` sources are tailed: probestyx remembers how far into the file it has already read. The first scrape after startup seeds that position at the current end of the file (so it doesn't dump the whole historical log on boot) and reports nothing; every scrape after that only sees lines appended since the previous one. If the file shrinks (log rotation/truncation), it detects that and starts over from the beginning of the new file.
- `type: command` sources (e.g. `docker logs`) have no seekable position, so instead you opt in with a `{{since}}` placeholder in the command — it's substituted with the timestamp (RFC3339) of that scraper's last successful run, e.g. `docker logs --since {{since}} --tail 1000 my-container 2>&1`. Without `{{since}}` in the command, it re-runs and re-scans the full output every time (matching plain `raw`/`json` command sources).
- This state lives in memory only, per scraper: a probestyx restart resets tailing to "start from now." If more than one separate consumer polls the same `/metrics` endpoint, they'd race over a single shared position by default — see **Per-Consumer Tailing** below to give each consumer its own independent view.

### Per-Consumer Tailing

If a single probestyx instance is polled by more than one independent monitoring backend, each needs its own "what's new since I last looked" position — otherwise whichever one happens to poll first after a new error line appears "consumes" it, and the others never see it.

Declare the expected consumers in `server.consumers`, then have each one pass its identity as `?callerId=`:

```yaml
server:
  port: 9100
  consumers:
    - logstyx-prod
    - staging-poller
```

```bash
curl "http://localhost:9100/metrics?callerId=logstyx-prod"
curl "http://localhost:9100/metrics?callerId=staging-poller"
```

- Each declared `callerId` gets its own independent file offset / command `{{since}}` timestamp per scraper — polling as `logstyx-prod` has no effect on what `staging-poller` sees next, and vice versa.
- A request with no `callerId` (or when `server.consumers` isn't set at all) falls back to a single shared `"default"` bucket — today's single-consumer behavior, unchanged.
- A `callerId` that isn't in `server.consumers` is rejected with `400 Bad Request`, rather than silently tracked. This is deliberate: `/metrics` is often unauthenticated, so honoring arbitrary caller-supplied IDs would let anyone grow the in-memory tailing-state map without bound just by requesting new IDs. Only enable this by explicitly listing who's allowed to have their own view.

**Config Example — nginx error log (file source, tailed):**
```yaml
- name: nginx_errors
  source:
    type: file
    path: "/var/log/nginx/error.log"
    format: log
    max_lines: 50
  metrics:
    - match: "count"
      name: "nginx_error_count"
    - match: "lines"
      name: "nginx_error_lines"
```

**Config Example — Docker container logs (command source, `{{since}}`-windowed):**
```yaml
- name: docker_errors
  source:
    type: command
    command: "docker logs --since {{since}} --tail 1000 my-container 2>&1"
    format: log
    max_lines: 50
  metrics:
    - match: "count"
      name: "docker_error_count"
    - match: "lines"
      name: "docker_error_lines"
```

`type: command` runs a shell command (via `sh -c` on Linux/macOS, `cmd /C` on Windows) and captures its combined stdout+stderr as the source text. Only put trusted, operator-authored commands in `config.yaml` — this is not meant to run untrusted or request-derived input.

**Example Output** (both scrapers above, on a scrape where one new error appeared in each source since the last scrape):
```json
{
  "nginx_errors": {
    "nginx_error_count": 1,
    "nginx_error_lines": [
      "2026/07/18 10:00:05 [error] 123#0: *2 open() \"/var/www/missing.html\" failed (2: No such file or directory)"
    ]
  },
  "docker_errors": {
    "docker_error_count": 1,
    "docker_error_lines": [
      "something failed with an Error: boom"
    ]
  }
}
```

- `<name>_count` / `<name>_lines` reflect only what's new since the last scrape (see tailing behavior above) — not the full history retained in the log or tail window.
- `<name>_lines` is capped at `max_lines`; if more than that appeared since the last scrape, `count` still reflects the true total while `lines` shows only the first `max_lines` of them.
- Nothing in the matched lines is redacted — if your logs contain IPs, paths, or tokens, they're shipped as-is. Enable `server.secret` (HMAC auth) on any deployment using `log` format.

Under pull mode, a new error only reaches whoever's polling `/metrics` on their next scrape — up to a full poll interval of latency. If that's too slow, see **Push Mode** below: probestyx can send new lines to Logstyx itself, as they appear.

### 4. Raw Format

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

## Push Mode

Probestyx is normally pull-only: it exposes `/metrics` and waits to be polled. Push mode is an opt-in add-on for `format: log` scrapers specifically — instead of waiting for the next poll, probestyx detects new error lines itself and POSTs them straight to Logstyx's ingestion API (`/api/v1/logs`), cutting latency from "up to one poll interval" down to about the length of a short debounce window.

### How it decides when to push

- **`type: file` scrapers are event-driven**: probestyx watches the log file's directory (not the file itself — this is what lets it survive log rotation, including the rename-then-recreate scheme, since a directory watch isn't tied to the old file's inode) and pushes shortly after new lines are written, debounced by ~200ms so a burst of many lines in one write collapses into a single push instead of one per line.
- **`type: command` scrapers** (e.g. `docker logs`) have no file to watch, so they're re-run on a ticker instead — `push.command_poll_interval_seconds` (default 15).
- Either way, one push is one scraper's new lines since the last trigger (matching the `{count, lines}` shape `format: log` already produces) — never one push per line, since Logstyx's ingestion API has no batch endpoint and this keeps well under its rate limit during a burst.
- Push maintains its **own** tailing cursor per scraper, completely independent of whatever polls `/metrics` — enabling push for a scraper doesn't change what `/metrics` reports for it, and vice versa. If a scraper is both pulled and pushed, the same lines will appear through both channels independently (each side reports "what's new since *it* last looked").

### Configuration

```yaml
server:
  port: 9100

push:
  enabled: true
  endpoint: "https://api.logstyx.com/api/v1/logs"
  project_id: "your-project-id"
  secret: "your-project-secret"       # from the Logstyx dashboard - NOT the same as server.secret
  level: "error"                      # optional, default "error"
  context:                            # optional static tags attached to every push
    environment: "production"
    region: "us-east-1"

scrapers:
  - name: nginx_errors
    source:
      type: file
      path: "/var/log/nginx/error.log"
      format: log
      max_lines: 50
    push: true                        # opt this scraper into push mode
```

`push.secret` is deliberately a **separate** field from `server.secret`: `server.secret` authenticates callers *of* probestyx's own `/metrics` endpoint, while `push.secret` is the credential probestyx uses to authenticate *itself* to Logstyx. They're different keys for different directions and must never be the same value. `push: true` on a scraper only takes effect when `Source.Format` is `log` and the global `push.enabled` is `true` — anything else is ignored with a startup warning, not a hard failure.

### Reliability characteristics, plainly

- **In-memory retry queue only** — failed pushes are retried with exponential backoff (1s up to 30s), but nothing is persisted to disk. A probestyx restart during a Logstyx outage loses whatever was queued, and (since tailing state is also in-memory) resets to "start from now" — it will not go back and re-send what it missed while it was down.
- **A `401` (bad `secret`/`project_id`) is treated as fatal**, not retried: push logs one prominent warning, drops everything queued, and stops trying for the rest of that process's life, rather than silently retrying into a bounded queue forever. Restart probestyx after fixing the config.
- **A `200` response does not guarantee the log was actually stored** — an unknown or mistyped `project_id` is silently accepted by Logstyx as a fake success and the log is discarded server-side, with no way to detect this from the response. Double-check `project_id` against the dashboard rather than trusting a lack of errors.
- The queue is bounded (`push.max_queue_size`, default 256); if it fills up (e.g. a prolonged outage), the *oldest* pending item is dropped to make room, and this is logged.

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

- `GET /metrics` - Returns all collected metrics as JSON. Accepts `?callerId=<id>` for [per-consumer log tailing](#per-consumer-tailing) when `server.consumers` is configured.
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

## Service Management

After installation, manage Probestyx with these commands:

### Linux (systemd)

```bash
sudo systemctl status probestyx    # Check status
sudo systemctl restart probestyx   # Restart
sudo systemctl stop probestyx      # Stop
sudo journalctl -u probestyx -f    # View logs
```

### macOS (launchd)

```bash
sudo launchctl list | grep probestyx           # Check status
sudo launchctl stop com.probestyx              # Stop
sudo launchctl start com.probestyx             # Start
tail -f /var/log/probestyx.log                 # View logs
```

### Windows (NSSM)

```powershell
nssm status Probestyx                                      # Check status
nssm restart Probestyx                                     # Restart
nssm stop Probestyx                                        # Stop
Get-Content C:\probestyx\probestyx.log -Tail 50 -Wait      # View logs
```

## Docker

Run Probestyx in a Docker container:

```bash
# Quick start
docker run -d \
  --name probestyx \
  --restart unless-stopped \
  -p 9100:9100 \
  -v $(pwd)/config.yaml:/root/config.yaml:ro \
  devatlogstyx/probestyx:latest

# View logs
docker logs -f probestyx
```

## Testing

```bash
# Query metrics
curl http://localhost:9100/metrics

# Check health
curl http://localhost:9100/health
```

## License

MIT