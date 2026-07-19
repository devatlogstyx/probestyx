package push

import (
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/metrics"
)

// runCommandLoop paces push-enabled type: command scrapers (e.g. docker logs)
// on a shared ticker, since they have no file to watch and so cannot be
// event-driven the way file scrapers are. Runs in its own goroutine, separate
// from the fsnotify loop, since command scrapers share no state or pacing
// model with file scrapers - one select juggling both would just interleave
// two unrelated concerns.
func runCommandLoop(sender *Sender, pushCfg config.PushConfig, scrapers []config.ScraperConfig, interval time.Duration) {
	if len(scrapers) == 0 {
		return
	}

	// Prime once so runCommandSince seeds "since=now" and the first real tick
	// doesn't dump backlog history. Result discarded.
	for _, sc := range scrapers {
		_, _ = metrics.CollectLog(sc, pushConsumerID)
	}

	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		for _, sc := range scrapers {
			collectAndPush(sender, pushCfg, sc) // sequential; commands are heavyweight, no need to fan out
		}
	}
}
