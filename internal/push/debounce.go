package push

import (
	"sync"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
)

// fileScraper tracks debounce state for one push-enabled type: file scraper.
// mu guards only the timer field; the (possibly slow) collect+push work runs
// outside the lock, in the timer's own goroutine - so the fsnotify event loop
// that calls schedule() is never blocked by it.
type fileScraper struct {
	cfg     config.ScraperConfig
	path    string // cleaned absolute path
	dir     string // cleaned absolute parent directory
	sender  *Sender
	pushCfg config.PushConfig

	mu    sync.Mutex
	timer *time.Timer // debounce timer; nil when idle
}

const debounceDelay = 200 * time.Millisecond

// schedule is called from the single fsnotify event-consuming goroutine. It
// coalesces rapid successive writes (e.g. a burst of many error lines written
// in one second) into one fire() using trailing-edge debouncing: each new
// event pushes the fire time back out, so it fires debounceDelay after the
// LAST write in a burst, not the first.
func (fs *fileScraper) schedule(d time.Duration) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.timer == nil {
		fs.timer = time.AfterFunc(d, fs.fire)
	} else {
		fs.timer.Reset(d)
	}
}

// fire runs in the AfterFunc's own goroutine, so the slow collect+push work
// never blocks the fsnotify event loop.
//
// Benign race, by design: an event can arrive in the tiny window after this
// fires but before it sets fs.timer = nil, causing schedule() to Reset() an
// already-fired timer and reactivate it - producing a second fire(). This
// costs nothing: the second CollectLog call reads from the already-advanced
// tailing offset and returns Count == 0, which collectAndPush drops. Accepting
// this is simpler and just as correct as adding compare-and-swap gymnastics
// here would be.
func (fs *fileScraper) fire() {
	fs.mu.Lock()
	fs.timer = nil
	fs.mu.Unlock()
	collectAndPush(fs.sender, fs.pushCfg, fs.cfg)
}
