// Package aksk defines the AK/SK request-signing contract shared by the server
// (signature verification) and the MCP client binary (request signing).
//
// A signed request carries three headers:
//
//	X-Api-Key:    <ak>
//	X-Timestamp:  <unix seconds>
//	X-Signature:  <lowercase hex HMAC-SHA256>
//
// The signature is computed over the canonical string:
//
//	<ak>\n<timestamp>\n<METHOD>\n<path>\n<body-sha256-hex>
//
// where <path> is the request path WITHOUT the query string, <METHOD> is the
// uppercase HTTP verb, and <body-sha256-hex> is the hex SHA-256 of the raw
// request body (the empty string when there is no body). Timestamps must fall
// within TimestampWindow seconds of the server clock to defeat replay.
package aksk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Header names.
const (
	HeaderKey       = "X-Api-Key"
	HeaderTimestamp = "X-Timestamp"
	HeaderSignature = "X-Signature"
)

// TimestampWindow is the allowed clock skew in seconds.
const TimestampWindow = 300

// Canonical builds the string that is signed (client) and verified (server).
func Canonical(ak, timestamp, method, path, bodySHA256Hex string) string {
	return strings.Join([]string{ak, timestamp, method, path, bodySHA256Hex}, "\n")
}

// Sign returns the lowercase hex HMAC-SHA256 signature over Canonical.
func Sign(ak, timestamp, method, path, bodySHA256Hex, sk string) string {
	mac := hmac.New(sha256.New, []byte(sk))
	mac.Write([]byte(Canonical(ak, timestamp, method, path, bodySHA256Hex)))
	return hex.EncodeToString(mac.Sum(nil))
}

// SHA256Hex returns the lowercase hex SHA-256 of b.
func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
