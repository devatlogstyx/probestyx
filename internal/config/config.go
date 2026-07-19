package config

// Config structures
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	System   SystemConfig    `yaml:"system"`
	Scrapers []ScraperConfig `yaml:"scrapers"`
	// Push is the opt-in outbound push section. Pointer (not value) so its
	// absence is distinguishable from a zeroed struct: nil means pure pull
	// mode, identical to today, with zero overhead - no sender, no watcher.
	Push *PushConfig `yaml:"push,omitempty"`
}

// PushConfig configures probestyx's opt-in outbound push mode. When Enabled,
// probestyx acts as a CLIENT of Logstyx's ingestion API (POST /api/v1/logs),
// forwarding newly-appeared lines from format: log scrapers as they're detected
// instead of waiting to be polled on GET /metrics. Its credentials are SEPARATE
// from ServerConfig.Secret: Server.Secret authenticates INBOUND callers of
// probestyx's own /metrics endpoint (and signs only a timestamp, under X-*
// headers), whereas the fields here authenticate probestyx ITSELF as an OUTBOUND
// client to Logstyx (signs the full payload, under lowercase headers). The two
// schemes are not interchangeable and their secrets must never be shared.
type PushConfig struct {
	// Enabled is the master switch. Even if a scraper sets push: true, nothing
	// is sent unless this is also true.
	Enabled bool `yaml:"enabled"`

	// Endpoint is the full ingestion URL, e.g. "https://api.logstyx.com/api/v1/logs".
	// Required when Enabled.
	Endpoint string `yaml:"endpoint"`

	// ProjectID is echoed into every payload AND prepended to the signed
	// string, so it's security-relevant, not just routing. An unknown/dead
	// ProjectID is silently accepted by the server as a fake 200 and the log
	// is discarded - a typo here fails silently. Verify it against the
	// Logstyx dashboard.
	ProjectID string `yaml:"project_id"`

	// Secret is the HMAC-SHA256 key for signing outbound payloads. Distinct
	// from ServerConfig.Secret (see type comment). A wrong secret yields a
	// hard 401 that trips the push sender's auth circuit breaker instead of
	// retrying forever.
	Secret string `yaml:"secret"`

	// Level is stamped on every pushed entry. The server documents it as
	// required but does not validate it. Defaults to "error" - format: log
	// scrapers exist specifically to surface errors.
	Level string `yaml:"level,omitempty"`

	// Device identifies the emitting host/agent, sent as-is in every payload.
	// Defaults to {"type":"probestyx","hostname":<os.Hostname()>,"platform":<runtime.GOOS>} when unset.
	Device map[string]string `yaml:"device,omitempty"`

	// Context is an optional static tag map merged into every payload's
	// "context" field - operator labels like environment/region. Sent verbatim.
	Context map[string]string `yaml:"context,omitempty"`

	// MaxQueueSize bounds the in-memory retry queue; on overflow the OLDEST
	// pending item is dropped (and logged). There is no disk persistence: a
	// restart loses whatever is queued (an accepted limitation, mirroring the
	// reference Logstyx SDKs). Defaults to 256.
	MaxQueueSize int `yaml:"max_queue_size,omitempty"`

	// CommandPollIntervalSeconds paces push-enabled type: command scrapers
	// (e.g. docker logs), which have no file to watch and so cannot be
	// event-driven - they're re-run on this ticker instead. Defaults to 15.
	CommandPollIntervalSeconds int `yaml:"command_poll_interval_seconds,omitempty"`
}

type ServerConfig struct {
	Port      int      `yaml:"port"`
	Secret    string   `yaml:"secret"`
	Consumers []string `yaml:"consumers,omitempty"` // allowlist of ?callerId= values for per-consumer log-tailing state
}

type SystemConfig struct {
    Enabled  bool     `yaml:"enabled"`
    Name     string   `yaml:"name"`
    CacheTTL int      `yaml:"cache_ttl"`  // Add this line
    Metrics  []string `yaml:"metrics"`
}

type ScraperConfig struct {
	Name    string        `yaml:"name"`
	Source  SourceConfig  `yaml:"source"`
	Metrics []MetricMap   `yaml:"metrics"`
	Filter  *FilterConfig `yaml:"filter,omitempty"`
	// Push opts this scraper into outbound push mode. Only honored when
	// Source.Format == "log" and the top-level Push section is enabled; on
	// any other format it's ignored with a startup warning, since only
	// format: log scrapers produce the count/lines shape push forwards.
	Push bool `yaml:"push,omitempty"`
}

type SourceConfig struct {
	Type     string `yaml:"type"` // url, file, command
	URL      string `yaml:"url,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Command  string `yaml:"command,omitempty"`  // for type: command
	Format   string `yaml:"format"`              // json, prometheus, raw, log
	Pattern  string `yaml:"pattern,omitempty"`   // for format: raw/log
	MaxLines int    `yaml:"max_lines,omitempty"` // for format: log, default 100
}

type MetricMap struct {
	Path      string `yaml:"path,omitempty"`      // for json
	Match     string `yaml:"match,omitempty"`     // for prometheus/raw
	Name      string `yaml:"name"`
	Calculate string `yaml:"calculate,omitempty"`
}

type FilterConfig struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}