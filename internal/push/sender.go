package push

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
)

type pushItem struct {
	payload *logPayload
	scraper string
}

type sendOutcome int

const (
	sendOK        sendOutcome = iota // 200 with expected body -> dequeue, reset failures
	sendRetry                        // network err / 429 / 5xx -> keep, backoff
	sendDropItem                     // deterministic per-item failure (400/other 4xx, build error) -> drop just this item
	sendAuthFatal                    // 401 -> trip circuit breaker, stop the sender
)

// Sender owns an in-memory retry queue and a background goroutine that drains
// it with exponential backoff. All queue state (queue, failures, authBroken) is
// touched only inside run(), so no mutex is needed - Enqueue communicates with
// it purely via a channel.
type Sender struct {
	cfg    config.PushConfig
	client *http.Client
	in     chan *pushItem

	// Owned solely by run(); never touched from another goroutine.
	queue      []*pushItem
	failures   int  // consecutive global failure count -> exponential backoff
	authBroken bool // latched on first 401; sender then drains and refuses new work

	// backoffBase/backoffCap parameterize backoff() - 1s/30s in production
	// (set by NewSender). Tests construct a Sender directly with much smaller
	// values so the retry/backoff state machine can be exercised in
	// milliseconds instead of tens of seconds.
	backoffBase time.Duration
	backoffCap  time.Duration

	// Atomic observability counters.
	sent, failed, dropped uint64
}

// NewSender builds a Sender with a dedicated HTTP client, mirroring the
// Timeout/Transport shape metrics.getHTTPClient uses (a separate instance,
// since this is a different package with its own outbound traffic pattern).
func NewSender(cfg config.PushConfig) *Sender {
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = 256
	}
	return &Sender{
		cfg: cfg,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		in:          make(chan *pushItem, 64),
		backoffBase: time.Second,
		backoffCap:  30 * time.Second,
	}
}

// Start launches the sender's background run loop and returns immediately.
// The loop runs until ctx is done.
func (s *Sender) Start(ctx context.Context) {
	go s.run(ctx)
}

// Enqueue is non-blocking: a stalled/overloaded sender never blocks the
// collect/watch loop that calls this. If the internal channel buffer is full,
// the item is dropped and logged rather than blocking the caller.
func (s *Sender) Enqueue(scraperName string, payload *logPayload) {
	select {
	case s.in <- &pushItem{payload: payload, scraper: scraperName}:
	default:
		atomic.AddUint64(&s.dropped, 1)
		log.Printf("probestyx push: enqueue channel full, dropping item for scraper %s", scraperName)
	}
}

func (s *Sender) run(ctx context.Context) {
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		if !s.authBroken && len(s.queue) > 0 && timer == nil {
			timer = time.NewTimer(s.backoff())
			timerC = timer.C
		}

		select {
		case it := <-s.in:
			if s.authBroken {
				atomic.AddUint64(&s.dropped, 1)
				continue
			}
			s.enqueueLocal(it)

		case <-timerC:
			timer = nil
			timerC = nil
			s.attemptHead(ctx)

		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

// backoff: failures==0 gives backoffBase steady-state pacing (in production,
// 1s - far under Logstyx's ~20 req/s ingress cap and doubling as a rate-limit
// guard). Each consecutive failure doubles the delay, capped at backoffCap.
// failures is clamped separately (see attemptHead) so this shift can't overflow.
func (s *Sender) backoff() time.Duration {
	d := s.backoffBase << s.failures
	if d <= 0 || d > s.backoffCap { // d<=0 guards a pathological shift overflow
		return s.backoffCap
	}
	return d
}

// enqueueLocal appends, dropping the OLDEST item first if at capacity. Bounds
// memory and is never silent about it.
func (s *Sender) enqueueLocal(it *pushItem) {
	max := s.cfg.MaxQueueSize
	if len(s.queue) >= max {
		dropped := s.queue[0]
		s.queue = s.queue[1:]
		atomic.AddUint64(&s.dropped, 1)
		log.Printf("probestyx push: queue full (%d), dropping oldest item for scraper %s", max, dropped.scraper)
	}
	s.queue = append(s.queue, it)
}

func (s *Sender) attemptHead(ctx context.Context) {
	if len(s.queue) == 0 {
		return
	}
	it := s.queue[0]
	switch s.trySend(ctx, it) {
	case sendOK:
		s.queue = s.queue[1:]
		s.failures = 0
		atomic.AddUint64(&s.sent, 1)

	case sendRetry:
		if s.failures < 32 { // clamp: prevents unbounded growth (and shift overflow in backoff) during a long outage
			s.failures++
		}
		// Rotate the head to the back so one slow/poison item doesn't
		// permanently starve newer items behind it.
		s.queue = append(s.queue[1:], it)
		atomic.AddUint64(&s.failed, 1)

	case sendDropItem:
		s.queue = s.queue[1:]
		s.failures = 0 // the pipeline is healthy; only this item was bad
		atomic.AddUint64(&s.dropped, 1)

	case sendAuthFatal:
		n := len(s.queue)
		s.authBroken = true
		s.queue = nil
		log.Printf("probestyx push: FATAL 401 from Logstyx - check push.secret / push.project_id. "+
			"Push disabled for this process; dropped %d queued item(s).", n)
	}
}

// trySend classifies the outcome of one delivery attempt. A wrong secret or
// project_id will never succeed on retry (sendAuthFatal), so it's handled as a
// circuit breaker rather than retried forever into a bounded queue that would
// otherwise hide the misconfiguration silently.
func (s *Sender) trySend(ctx context.Context, it *pushItem) sendOutcome {
	req, err := buildRequest(ctx, s.cfg, it.payload)
	if err != nil {
		log.Printf("probestyx push: build error for scraper %s: %v (dropping)", it.scraper, err)
		return sendDropItem
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return sendRetry // dial/timeout/reset -> transient
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	switch {
	case resp.StatusCode == 200:
		// A 200 does not guarantee the log was actually stored - Logstyx
		// silently accepts an unknown/dead project_id as a fake success - so
		// this can only catch a misrouted response (e.g. a reverse proxy
		// returning an HTML 200 for the wrong reason), not a bad project_id.
		if !looksLikeSuccessBody(body) {
			log.Printf("probestyx push: 200 with unexpected body for scraper %s: %q", it.scraper, body)
		}
		return sendOK

	case resp.StatusCode == 401:
		return sendAuthFatal

	case resp.StatusCode == 429 || resp.StatusCode >= 500:
		return sendRetry // nginx rate-limit or server transient; backoff self-throttles

	default: // 400 and other 4xx
		log.Printf("probestyx push: permanent %d for scraper %s: %q (dropping item)",
			resp.StatusCode, it.scraper, body)
		return sendDropItem // malformed payload won't fix on retry; don't poison the queue
	}
}

func looksLikeSuccessBody(b []byte) bool {
	var r struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	return json.Unmarshal(b, &r) == nil && r.Error == 200
}
