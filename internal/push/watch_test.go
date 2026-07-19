package push

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
)

func testLogScraper(name, path string) config.ScraperConfig {
	return config.ScraperConfig{
		Name: name,
		Source: config.SourceConfig{
			Type:   "file",
			Path:   path,
			Format: "log",
		},
	}
}

func appendLine(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		t.Fatalf("append: %v", err)
	}
}

// drainOne waits for exactly one enqueued push and asserts its Data.Count.
// Sender.Start is never called in these tests, so items just accumulate in
// the buffered channel for direct inspection - no network involved.
func drainOne(t *testing.T, sender *Sender, wantCount int) {
	t.Helper()
	select {
	case item := <-sender.in:
		if item.payload.Data.Count != wantCount {
			t.Fatalf("expected count %d, got %d (lines=%v)", wantCount, item.payload.Data.Count, item.payload.Data.Lines)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a push")
	}
}

func assertNoFurtherPush(t *testing.T, sender *Sender, wait time.Duration) {
	t.Helper()
	select {
	case item := <-sender.in:
		t.Fatalf("expected no further push, got one: %+v", item.payload.Data)
	case <-time.After(wait):
	}
}

func TestFileWatch_DebouncesBurstIntoOneCollect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "error.log")
	if err := os.WriteFile(path, []byte("2026/01/01 [notice] startup\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sender := newTestSender("http://unused.invalid", 64)
	scraper := testLogScraper("nginx_errors", path)

	go runFileWatch(sender, config.PushConfig{}, []config.ScraperConfig{scraper})
	time.Sleep(150 * time.Millisecond) // let priming + the initial directory watch attach

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := f.WriteString("2026/01/01 [error] burst line\n"); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()

	drainOne(t, sender, 20) // the 20-line burst must coalesce into ONE push
	assertNoFurtherPush(t, sender, 500*time.Millisecond)
}

func TestFileWatch_SurvivesCopyTruncateRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "error.log")
	if err := os.WriteFile(path, []byte("2026/01/01 [notice] startup\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sender := newTestSender("http://unused.invalid", 64)
	scraper := testLogScraper("app_errors", path)
	go runFileWatch(sender, config.PushConfig{}, []config.ScraperConfig{scraper})
	time.Sleep(150 * time.Millisecond)

	appendLine(t, path, "2026/01/01 [error] before rotation\n")
	drainOne(t, sender, 1)

	// Simulate copy-then-truncate rotation: the log file is truncated to zero
	// in place (same path, same inode) after being copied elsewhere.
	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	appendLine(t, path, "2026/01/01 [error] after truncate\n")

	drainOne(t, sender, 1)
}

func TestFileWatch_SurvivesRenameRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "error.log")
	if err := os.WriteFile(path, []byte("2026/01/01 [notice] startup\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sender := newTestSender("http://unused.invalid", 64)
	scraper := testLogScraper("app_errors", path)
	go runFileWatch(sender, config.PushConfig{}, []config.ScraperConfig{scraper})
	time.Sleep(150 * time.Millisecond)

	appendLine(t, path, "2026/01/01 [error] before rotation\n")
	drainOne(t, sender, 1)

	// Simulate logrotate's rename-then-recreate: move the old file away, then
	// create a brand new (empty) file at the same path. This is the scheme a
	// file-level fsnotify watch would NOT survive (it follows the old file's
	// inode away); the directory watch should pick up the Create at the
	// original path and keep tailing it.
	rotated := filepath.Join(dir, "error.log.1")
	if err := os.Rename(path, rotated); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}

	appendLine(t, path, "2026/01/01 [error] after rotation\n")
	drainOne(t, sender, 1)
}
