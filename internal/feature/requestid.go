package feature

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// RequestIDHeader names one request across Zelport, the service and their logs.
//
// A Zelport keeps an ID the request already carries, so it stays the same over several
// Zelports and other proxies. Without one it takes the trace ID of a W3C traceparent, so
// the ID matches the trace in OpenTelemetry tools, and otherwise picks a random one.
// Nothing is stored. The traceparent itself passes unchanged, Zelport is not a trace span.
const RequestIDHeader = "X-Request-Id"

// TraceparentHeader is the W3C Trace Context header (https://www.w3.org/TR/trace-context/).
const TraceparentHeader = "traceparent"

// requestIDMaxLength bounds an accepted ID, so a client cannot fill logs or headers.
const requestIDMaxLength = 128

// RequestID returns the ID for a request with these headers.
func RequestID(h http.Header) string {
	if values := h.Values(RequestIDHeader); len(values) == 1 && validRequestID(values[0]) {
		return values[0]
	}
	if values := h.Values(TraceparentHeader); len(values) == 1 {
		if id, ok := traceID(values[0]); ok {
			return id
		}
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms.
	}
	return hex.EncodeToString(id)
}

// AssignRequestID puts the request's ID on the request toward the service and on the answer,
// so Zelport's own status pages carry it too. Calling it again keeps the same ID.
func AssignRequestID(w http.ResponseWriter, r *http.Request) *http.Request {
	id := RequestID(r.Header)
	w.Header().Set(RequestIDHeader, id)
	if r.Header.Get(RequestIDHeader) == id && len(r.Header.Values(RequestIDHeader)) == 1 {
		return r
	}
	r = r.Clone(r.Context())
	r.Header.Set(RequestIDHeader, id)
	return r
}

func validRequestID(v string) bool {
	if v == "" || len(v) > requestIDMaxLength {
		return false
	}
	for i := 0; i < len(v); i++ {
		if v[i] < 0x21 || v[i] > 0x7e { // Visible ASCII only: no spaces, controls or line breaks.
			return false
		}
	}
	return true
}

// traceID reads the trace ID of a traceparent: version-traceid-parentid-flags, lowercase hex.
// Versions above 00 may append fields, version ff is invalid, and an all-zero ID is invalid.
func traceID(v string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(v), "-")
	if len(parts) < 4 || len(parts[0]) != 2 || len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return "", false
	}
	if parts[0] == "ff" || parts[0] == "00" && len(parts) != 4 {
		return "", false
	}
	for _, p := range parts[:4] {
		if !lowerHex(p) {
			return "", false
		}
	}
	if strings.Trim(parts[1], "0") == "" || strings.Trim(parts[2], "0") == "" {
		return "", false
	}
	return parts[1], true
}

func lowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
	}
	return true
}
