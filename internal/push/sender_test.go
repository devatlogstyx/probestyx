package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
)

// newTestSender builds a Sender bypassing NewSender's production defaults, so
// tests can use millisecond-scale backoff instead of waiting seconds.
func newTestSender(endpoint string, maxQueue int) *Sender {
	if maxQueue <= 0 {
		maxQueue = 8
	}
	return &Sender{
		cfg: config.PushConfig{
			Endpoint:     endpoint,
			ProjectID:    "proj_test",
			Secret:       "secret_test",
			MaxQueueSize: maxQueue,
		},
		client:      &http.Client{Timeout: 2 * time.Second},
		in:          make(chan *pushItem, 64),
		backoffBase: 5 * time.Millisecond,
		backoffCap:  50 * time.Millisecond,
	}
}

func testPayload(scraper string) *logPayload {
	return &logPayload{
		Level:     "error",
		ProjectID: "proj_test",
		Device:    map[string]string{},
		Context:   map[string]string{},
		Data:      logData{Scraper: scraper, Count: 1, Lines: []string{"boom"}},
	}
}

// waitFor polls until cond() is true or the timeout elapses, failing the test
// on timeout. Used because the sender's run loop is asynchronous.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestSender_SuccessDeliversAndSetsHeaders(t *testing.T) {
	var gotTimestamp, gotSignature, gotContentType string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTimestamp = r.Header.Get("timestamp")
		gotSignature = r.Header.Get("signature")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
		w.Write([]byte(`{"error":200,"message":"Success"}`))
	}))
	defer srv.Close()

	s := newTestSender(srv.URL, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	s.Enqueue("nginx_errors", testPayload("nginx_errors"))

	waitFor(t, time.Second, func() bool { return atomic.LoadUint64(&s.sent) == 1 })

	if gotTimestamp == "" {
		t.Error("expected a timestamp header")
	}
	if len(gotSignature) != 64 {
		t.Errorf("expected a 64-char hex signature header, got %q", gotSignature)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", gotContentType)
	}
	var parsed logPayload
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("body did not parse as logPayload: %v", err)
	}
	if parsed.Data.Scraper != "nginx_errors" {
		t.Errorf("unexpected data.scraper: %q", parsed.Data.Scraper)
	}
}

func TestSender_RetriesThenSucceeds(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"error":200,"message":"Success"}`))
	}))
	defer srv.Close()

	s := newTestSender(srv.URL, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	s.Enqueue("app", testPayload("app"))

	waitFor(t, 2*time.Second, func() bool { return atomic.LoadUint64(&s.sent) == 1 })

	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Errorf("expected exactly 3 attempts (2 failures + 1 success), got %d", got)
	}
	if got := atomic.LoadUint64(&s.failed); got != 2 {
		t.Errorf("expected 2 recorded failures, got %d", got)
	}
}

func TestSender_AuthFatalDropsQueueAndStopsRetrying(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(401)
	}))
	defer srv.Close()

	s := newTestSender(srv.URL, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	s.Enqueue("a", testPayload("a"))
	s.Enqueue("b", testPayload("b"))

	waitFor(t, time.Second, func() bool { return atomic.LoadInt32(&attempts) >= 1 })
	// Give the 401 a moment to be classified and latch the circuit breaker.
	waitFor(t, time.Second, func() bool { return atomic.LoadUint64(&s.sent) == 0 && attemptsSettled(&attempts, 200*time.Millisecond) })

	before := atomic.LoadInt32(&attempts)

	// Further enqueues after the breaker trips must be dropped without ever
	// reaching the server again.
	s.Enqueue("c", testPayload("c"))
	time.Sleep(100 * time.Millisecond)

	if got := atomic.LoadInt32(&attempts); got != before {
		t.Errorf("expected no further attempts after auth breaker tripped, attempts went from %d to %d", before, got)
	}
	if atomic.LoadUint64(&s.dropped) == 0 {
		t.Error("expected dropped counter to increment for both the queued item(s) and the post-breaker enqueue")
	}
}

// attemptsSettled reports whether the attempts counter has stopped changing
// for the given quiet period - a simple way to know the single in-flight 401
// has been classified without hard-coding an exact goroutine-scheduling delay.
func attemptsSettled(counter *int32, quiet time.Duration) bool {
	before := atomic.LoadInt32(counter)
	time.Sleep(quiet)
	return atomic.LoadInt32(counter) == before
}

func TestSender_PermanentClientErrorDropsWithoutRetry(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(400)
	}))
	defer srv.Close()

	s := newTestSender(srv.URL, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	s.Enqueue("bad", testPayload("bad"))

	waitFor(t, time.Second, func() bool { return atomic.LoadUint64(&s.dropped) == 1 })

	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("expected exactly 1 attempt for a permanent 4xx (no retry), got %d", got)
	}
	if atomic.LoadUint64(&s.sent) != 0 {
		t.Error("expected sent counter to stay 0 for a dropped item")
	}
}

func TestSender_QueueOverflowDropsOldest(t *testing.T) {
	// No server/Start() needed: enqueueLocal is exercised directly and
	// synchronously, since it's a pure function of queue state.
	s := newTestSender("http://unused.invalid", 3)

	for i := 0; i < 5; i++ {
		s.enqueueLocal(&pushItem{scraper: string(rune('a' + i))})
	}

	if len(s.queue) != 3 {
		t.Fatalf("expected queue capped at 3, got %d", len(s.queue))
	}
	// Oldest (a, b) should have been dropped; c, d, e should remain, in order.
	want := []string{"c", "d", "e"}
	for i, w := range want {
		if s.queue[i].scraper != w {
			t.Errorf("queue[%d] = %q, want %q", i, s.queue[i].scraper, w)
		}
	}
	if atomic.LoadUint64(&s.dropped) != 2 {
		t.Errorf("expected dropped counter == 2, got %d", s.dropped)
	}
}
