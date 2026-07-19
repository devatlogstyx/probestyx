package push

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/metrics"
)

// logPayload is the exact wire shape Logstyx's ingestion API expects. FIELD
// DECLARATION ORDER IS LOAD-BEARING: the server destructures {level, projectId,
// device, context, data} off the request body, rebuilds an object in that exact
// order, and re-runs JSON.stringify to recompute the signature. encoding/json
// emits struct fields in declaration order (unlike a map, which sorts keys
// alphabetically), so this order is what keeps our signed bytes identical to
// the server's reconstruction. Do not add other json-visible fields: the server
// strips the body to exactly these five keys, so any extra key we sign would
// not exist in the server's reconstruction and would produce a mismatched
// signature (401).
type logPayload struct {
	Level     string            `json:"level"`
	ProjectID string            `json:"projectId"`
	Device    map[string]string `json:"device"`  // nested key order is preserved
	Context   map[string]string `json:"context"` // by the server's JSON round-trip,
	Data      logData           `json:"data"`    // so maps are safe here (unlike the top level).
}

// logData is the opaque per-request payload: one POST is one scraper's
// already-batched output for one trigger (a debounced file write, or a command
// poll tick). We aggregate {scraper, count, lines} rather than sending one POST
// per line because there is no batch endpoint, and this keeps us far under
// Logstyx's ingress rate limit during a log burst.
type logData struct {
	Scraper string   `json:"scraper"`
	Count   int      `json:"count"`
	Lines   []string `json:"lines"`
}

// marshalPayload produces the bytes that are BOTH signed and sent as the
// request body. It deviates from a plain json.Marshal in two mandatory ways:
//
//  1. SetEscapeHTML(false): the server re-stringifies with JS's JSON.stringify,
//     which does not escape < > & (or U+2028/U+2029). Go's json.Marshal escapes
//     them by default, which would diverge from the server for any pushed line
//     containing them (URLs, comparison operators - common in nginx/app logs),
//     breaking the signature for exactly those payloads.
//  2. Trimming the single trailing '\n' that Encoder.Encode always appends
//     (plain json.Marshal does not add one, and the server's stringify has
//     none either). Signing that newline would break every signature.
func marshalPayload(p *logPayload) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

// buildRequest signs and constructs the outbound HTTP request. Signing happens
// fresh here (a fresh timestamp) every time this is called, so callers must
// call it per send attempt, not reuse a pre-signed request across retries.
func buildRequest(ctx context.Context, cfg config.PushConfig, p *logPayload) (*http.Request, error) {
	tsMillis := time.Now().UnixMilli()
	body, err := marshalPayload(p)
	if err != nil {
		return nil, err
	}
	sig := sign(cfg.Secret, cfg.ProjectID, body, tsMillis)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// Header names are lowercase, no X- prefix, matching Logstyx's real
	// server-client scheme - a different, incompatible convention from
	// probestyx's own inbound /metrics auth (X-Signature/X-Timestamp).
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("timestamp", strconv.FormatInt(tsMillis, 10))
	req.Header.Set("signature", sig)
	return req, nil
}

// buildPayload turns a scraper's log delta into the wire payload. Context/
// Device deliberately have no omitempty on their json tags (see logPayload):
// a present "context":null round-trips identically through the server's
// re-stringify, so consistency between what we sign and what we send is what
// matters here, not which choice - this is deliberate, not an oversight.
func buildPayload(cfg config.PushConfig, scraperName string, delta metrics.LogDelta) *logPayload {
	return &logPayload{
		Level:     orDefault(cfg.Level, "error"),
		ProjectID: cfg.ProjectID,
		Device:    deviceOrDefault(cfg.Device),
		Context:   cfg.Context,
		Data: logData{
			Scraper: scraperName,
			Count:   delta.Count,
			Lines:   delta.Lines,
		},
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// deviceOrDefault mirrors the shape Logstyx's own server-side SDKs populate
// (type/os/platform identifiers), adapted for probestyx as a Go agent rather
// than a browser or mobile client.
func deviceOrDefault(d map[string]string) map[string]string {
	if len(d) > 0 {
		return d
	}
	hostname, _ := os.Hostname()
	return map[string]string{
		"type":     "probestyx",
		"hostname": hostname,
		"platform": runtime.GOOS,
	}
}
