package config

// Config structures
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	System   SystemConfig    `yaml:"system"`
	Scrapers []ScraperConfig `yaml:"scrapers"`
}

type ServerConfig struct {
	Port   int    `yaml:"port"`
	Secret string `yaml:"secret"`
}

type SystemConfig struct {
	Enabled bool     `yaml:"enabled"`
	Metrics []string `yaml:"metrics"`
}

type ScraperConfig struct {
	Name    string        `yaml:"name"`
	Source  SourceConfig  `yaml:"source"`
	Metrics []MetricMap   `yaml:"metrics"`
	Filter  *FilterConfig `yaml:"filter,omitempty"`
}

type SourceConfig struct {
	Type    string `yaml:"type"` // url, file
	URL     string `yaml:"url,omitempty"`
	Path    string `yaml:"path,omitempty"`
	Format  string `yaml:"format"` // json, prometheus, raw
	Pattern string `yaml:"pattern,omitempty"`
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