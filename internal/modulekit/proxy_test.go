package modulekit

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/zelport/zelport/internal/feature"
)

func TestProxyStripsRelayTraceOnlyWhenAsked(t *testing.T) {
	var seen http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = r.Header.Clone() }))
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	for _, strip := range []bool{false, true} {
		r := httptest.NewRequest("GET", "http://app.localhost:7070/", nil)
		r.Header.Set(feature.RelayTraceHeader, "trace")
		r.Header.Set("Authorization", "Bearer t")
		Proxy(httptest.NewRecorder(), r, target, ProxyOptions{ListenerAuthority: "localhost:1", StripRelayTrace: strip})
		if got := seen.Get(feature.RelayTraceHeader); (got == "") != strip {
			t.Fatalf("strip=%v: trace header = %q", strip, got)
		}
		if seen.Get("Authorization") != "Bearer t" {
			t.Fatalf("strip=%v: other headers must pass, got %v", strip, seen)
		}
	}
	if ResponseHeaderTimeout.Minutes() != 5 {
		t.Fatalf("ResponseHeaderTimeout = %v, want 5m", ResponseHeaderTimeout)
	}
}
