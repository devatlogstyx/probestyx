package push

import (
	"log"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/metrics"
)

// collectAndPush is the shared bridge used by both the file watcher's
// debounce fire and the command poll ticker: collect new lines for one
// scraper using the fixed "push" consumer id (independent of any HTTP
// /metrics consumers - see metrics.CollectLog), and enqueue a payload only if
// anything new actually appeared.
func collectAndPush(sender *Sender, pushCfg config.PushConfig, scraper config.ScraperConfig) {
	delta, err := metrics.CollectLog(scraper, pushConsumerID)
	if err != nil {
		log.Printf("probestyx push: collect %q failed: %v", scraper.Name, err)
		return
	}
	if delta.Count == 0 {
		return // nothing new; makes debounce double-fires and spurious watch events free
	}
	sender.Enqueue(scraper.Name, buildPayload(pushCfg, scraper.Name, delta))
}
