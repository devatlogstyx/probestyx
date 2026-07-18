package metrics

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/parsers"
	"github.com/devatlogstyx/probestyx/internal/utils"
)

// Per-scraper state for format: log, keyed by scraper name (assumed unique).
// fileOffsets tracks how far into a tailed file we've already read; cmdLastRun
// tracks the last time a {{since}}-templated command was run. Both let repeated
// scrapes report only newly-appeared error lines instead of re-reporting
// whatever is still sitting in the log/tail window.
var (
	logStateMu  sync.Mutex
	fileOffsets = make(map[string]int64)
	cmdLastRun  = make(map[string]time.Time)
)

// Shared HTTP client - created once
var (
	httpClient *http.Client
	once       sync.Once
)

func getHTTPClient() *http.Client {
	once.Do(func() {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	})
	return httpClient
}

// CollectScraper runs one scraper. consumerID namespaces the format: log tailing
// state (file offset / last {{since}} run) so independent consumers polling the
// same probestyx instance each get their own "what's new since I last looked"
// view instead of racing over a single shared position. Use "default" when the
// caller doesn't distinguish consumers.
func CollectScraper(scraper config.ScraperConfig, consumerID string) (map[string]interface{}, error) {
	var rawData string
	var err error

	stateKey := scraper.Name + "\x00" + consumerID

	// Fetch data based on source type
	switch scraper.Source.Type {
	case "url":
		rawData, err = fetchURL(scraper.Source.URL)
	case "file":
		if scraper.Source.Format == "log" {
			rawData, err = tailFile(stateKey, scraper.Source.Path)
		} else {
			data, e := os.ReadFile(scraper.Source.Path)
			rawData = string(data)
			err = e
		}
	case "command":
		if scraper.Source.Format == "log" && strings.Contains(scraper.Source.Command, "{{since}}") {
			rawData, err = runCommandSince(stateKey, scraper.Source.Command)
		} else {
			rawData, err = runCommand(scraper.Source.Command)
		}
	default:
		return nil, fmt.Errorf("unknown source type: %s", scraper.Source.Type)
	}

	if err != nil {
		return nil, err
	}

	// Parse based on format
	var parsed map[string]interface{}
	switch scraper.Source.Format {
	case "json":
		parsed, err = parsers.ParseJSON(rawData)
	case "prometheus":
		parsed, err = parsers.ParsePrometheus(rawData)
	case "raw":
		parsed, err = parsers.ParseRaw(rawData, scraper.Source.Pattern)
	case "log":
		parsed, err = parsers.ParseLog(rawData, scraper.Source.Pattern, scraper.Source.MaxLines)
	default:
		return nil, fmt.Errorf("unknown format: %s", scraper.Source.Format)
	}

	if err != nil {
		return nil, err
	}

	// Apply filters if specified
	if scraper.Filter != nil {
		parsed = parsers.ApplyFilters(parsed, scraper.Filter)
	}

	// Map and transform metrics
	result := make(map[string]interface{})
	for _, metricMap := range scraper.Metrics {
		var value interface{}
		var found bool

		if metricMap.Path != "" {
			// JSON path lookup
			value, found = utils.GetJSONPath(parsed, metricMap.Path)
		} else if metricMap.Match != "" {
			// Pattern match
			value, found = parsed[metricMap.Match]
		}

		if !found {
			continue
		}

		// Apply calculation if specified
		if metricMap.Calculate != "" {
			if numVal, ok := utils.ToFloat64(value); ok {
				value = utils.Calculate(numVal, metricMap.Calculate)
			}
		}

		result[metricMap.Name] = value
	}

	return result, nil
}

func fetchURL(url string) (string, error) {
	resp, err := getHTTPClient().Get(url)  // Use shared client
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Limit response size
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB max
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// tailFile returns only the bytes appended to path since the last call for this
// scraper key. On the very first call it seeds the offset at the current end of
// file and returns nothing, so startup doesn't dump the entire historical log.
// If the file has shrunk since the last read (log rotation/truncation), it
// resets to the beginning and reads the new file from scratch.
func tailFile(key, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()

	logStateMu.Lock()
	offset, seen := fileOffsets[key]
	if !seen {
		fileOffsets[key] = size
		logStateMu.Unlock()
		return "", nil
	}
	if offset > size {
		offset = 0 // file was truncated or rotated to a new, smaller file
	}
	logStateMu.Unlock()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}

	data, err := io.ReadAll(io.LimitReader(f, 10*1024*1024)) // cap a single read, same as fetchURL
	if err != nil {
		return "", err
	}

	logStateMu.Lock()
	fileOffsets[key] = offset + int64(len(data))
	logStateMu.Unlock()

	return string(data), nil
}

// runCommandSince substitutes {{since}} in cmdTemplate with the timestamp of this
// scraper's last successful run (RFC3339), then runs it - e.g.
// "docker logs --since {{since}} --tail 1000 my-container 2>&1". On the first
// run there's no prior timestamp, so it seeds from "now" rather than guessing
// how far back to look.
func runCommandSince(key, cmdTemplate string) (string, error) {
	logStateMu.Lock()
	since, seen := cmdLastRun[key]
	logStateMu.Unlock()

	if !seen {
		since = time.Now()
	}

	cmdStr := strings.ReplaceAll(cmdTemplate, "{{since}}", since.UTC().Format(time.RFC3339))
	out, err := runCommand(cmdStr)

	logStateMu.Lock()
	cmdLastRun[key] = time.Now()
	logStateMu.Unlock()

	return out, err
}

// runCommand executes a shell command configured in config.yaml (e.g. "docker logs --tail 500 mycontainer 2>&1")
// and returns its combined stdout+stderr. The command string comes from the operator-controlled
// config file, not from request input, so shell interpretation here does not expose an
// injection surface to callers of /metrics.
func runCommand(cmdStr string) (string, error) {
	if cmdStr == "" {
		return "", fmt.Errorf("command is empty")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // docker logs writes to stderr for some containers

	if err := cmd.Run(); err != nil && out.Len() == 0 {
		return "", err
	}

	return out.String(), nil
}