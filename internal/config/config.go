package config

// Config structures
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	System   SystemConfig    `yaml:"system"`
	Scrapers []ScraperConfig `yaml:"scrapers"`
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