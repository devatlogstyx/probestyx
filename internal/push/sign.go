package push

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// sign reproduces Logstyx's server-side validateSignature exactly:
//
//	HMAC-SHA256(key=secret, msg = projectID + string(body) + decimal(tsMillis))
//	-> hex -> ToUpper
//
// with no separators between the three concatenated parts. body MUST be the
// exact bytes produced by marshalPayload for this request - the same bytes
// that get sent as the HTTP body - since the server recomputes this hash over
// its own re-serialization of the parsed payload and expects a byte-identical
// match. Streaming the three parts into the MAC is equivalent to hashing
// their concatenation and avoids an extra allocation.
func sign(secret, projectID string, body []byte, tsMillis int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(projectID))
	mac.Write(body)
	mac.Write([]byte(strconv.FormatInt(tsMillis, 10)))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}
