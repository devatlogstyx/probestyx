package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
)

var cfg *config.Config

func Init(c *config.Config) {
	cfg = c
}

func ValidateSignature(r *http.Request) bool {
	signature := r.Header.Get("X-Signature")
	timestamp := r.Header.Get("X-Timestamp")

	if signature == "" || timestamp == "" {
		return false
	}

	// Check timestamp is within 5 minutes
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	if abs(now-ts) > 300 {
		return false
	}

	// Verify HMAC
	mac := hmac.New(sha256.New, []byte(cfg.Server.Secret))
	mac.Write([]byte(timestamp))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}