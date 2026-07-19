package push

import (
	"fmt"
	"log"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
)

// pushConsumerID is the fixed synthetic consumer id push mode uses when
// calling metrics.CollectLog, so its file-offset/{{since}} tailing cursor is
// independent of whatever "default" or allowlisted-callerId consumers poll
// /metrics for the same scraper. This is the only coordination point between
// push and pull mode - no other state is shared or needs to be.
const pushConsumerID = "push"

// ValidatePushConfig checks the global push config when enabled, and returns
// the scrapers eligible for push (format: log + push: true). A push: true
// scraper that isn't format: log, or whose source type push doesn't support,
// is dropped with a warning rather than failing startup - push is additive,
// and a config mistake on one scraper shouldn't take down the whole agent.
func ValidatePushConfig(cfg *config.Config) ([]config.ScraperConfig, error) {
	if cfg.Push == nil || !cfg.Push.Enabled {
		return nil, nil
	}
	p := cfg.Push
	if p.Endpoint == "" {
		return nil, fmt.Errorf("push.enabled is true but push.endpoint is empty")
	}
	if p.ProjectID == "" {
		return nil, fmt.Errorf("push.enabled is true but push.project_id is empty")
	}
	if p.Secret == "" {
		return nil, fmt.Errorf("push.enabled is true but push.secret is empty")
	}

	var eligible []config.ScraperConfig
	for _, sc := range cfg.Scrapers {
		if !sc.Push {
			continue
		}
		if sc.Source.Format != "log" {
			log.Printf("probestyx push: scraper %q has push: true but format %q (not \"log\") - ignoring for push", sc.Name, sc.Source.Format)
			continue
		}
		if sc.Source.Type != "file" && sc.Source.Type != "command" {
			log.Printf("probestyx push: scraper %q has push: true but unsupported source type %q - ignoring for push", sc.Name, sc.Source.Type)
			continue
		}
		eligible = append(eligible, sc)
	}
	return eligible, nil
}

// Run starts push mode and blocks until the process exits (the file watcher
// runs on this goroutine for its lifetime; the command loop runs on its own).
// It's intentionally non-fatal: any setup problem here is logged and pull mode
// (/metrics) keeps working regardless. Launch from main.go as:
//
//	go push.Run(&cfg, sender)
func Run(cfg *config.Config, sender *Sender) {
	eligible, err := ValidatePushConfig(cfg)
	if err != nil {
		log.Printf("probestyx push: %v (push disabled)", err)
		return
	}
	if len(eligible) == 0 {
		log.Printf("probestyx push: enabled but no participating scrapers (need format: log + push: true)")
		return
	}

	var fileScrapers, cmdScrapers []config.ScraperConfig
	for _, sc := range eligible {
		switch sc.Source.Type {
		case "file":
			fileScrapers = append(fileScrapers, sc)
		case "command":
			cmdScrapers = append(cmdScrapers, sc)
		}
	}

	interval := time.Duration(cfg.Push.CommandPollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}
	go runCommandLoop(sender, *cfg.Push, cmdScrapers, interval)

	// Blocks this goroutine for the process's lifetime.
	runFileWatch(sender, *cfg.Push, fileScrapers)
}
