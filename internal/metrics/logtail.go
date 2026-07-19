package metrics

import (
	"fmt"
	"strings"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/parsers"
)

// LogDelta is the raw output of a format: log scraper for one collection:
// how many lines matched, and up to max_lines of the actual matched lines.
type LogDelta struct {
	Count int
	Lines []string
}

// CollectLog tails a format: log scraper and returns only the lines that
// matched since the last call for this consumerID. It shares tailFile's and
// runCommandSince's state (the same fileOffsets/cmdLastRun maps CollectScraper
// uses) via the same "scraper.Name + consumerID" keying, so a caller using a
// fixed consumerID (e.g. push mode's "push") gets its own tailing cursor,
// independent of any HTTP /metrics consumers - with no changes to that state.
//
// Unlike CollectScraper, this does not go through the scraper's metrics/filter
// mapping: it returns ParseLog's {count, lines} directly. That means a scraper
// used only for push doesn't need a metrics: block at all; a scraper that's
// both pulled and pushed keeps its existing metrics: mapping for /metrics,
// untouched by this function.
func CollectLog(scraper config.ScraperConfig, consumerID string) (LogDelta, error) {
	if scraper.Source.Format != "log" {
		return LogDelta{}, fmt.Errorf("CollectLog: scraper %q is not format: log", scraper.Name)
	}

	stateKey := scraper.Name + "\x00" + consumerID

	var raw string
	var err error
	switch scraper.Source.Type {
	case "file":
		raw, err = tailFile(stateKey, scraper.Source.Path)
	case "command":
		if strings.Contains(scraper.Source.Command, "{{since}}") {
			raw, err = runCommandSince(stateKey, scraper.Source.Command)
		} else {
			raw, err = runCommand(scraper.Source.Command)
		}
	default:
		return LogDelta{}, fmt.Errorf("CollectLog: unsupported source type %q for scraper %q", scraper.Source.Type, scraper.Name)
	}
	if err != nil {
		return LogDelta{}, err
	}

	parsed, err := parsers.ParseLog(raw, scraper.Source.Pattern, scraper.Source.MaxLines)
	if err != nil {
		return LogDelta{}, err
	}

	count, _ := parsed["count"].(float64)
	lines, _ := parsed["lines"].([]string)
	return LogDelta{Count: int(count), Lines: lines}, nil
}
