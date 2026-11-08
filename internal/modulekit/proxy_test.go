package modulekit

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/zelport/zelport/internal/feature"
)

func TestProxyKeepsZelportMetadataOnlyWhereALoopIsPossible(t *testing.T) {
	var seen http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = r.Header.Clone() }))
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	for _, c := range []struct {
		name            string
		options         ProxyOptions
		trace, loopMeta bool
	}{
		{"ordinary service", ProxyOptions{}, false, true},
		{"relay leg", ProxyOptions{KeepRelayTrace: true}, true, true},
		{"external target", ProxyOptions{External: true, KeepRelayTrace: true}, false, false},
	} {
		c.options.ListenerAuthority = "localhost:1"
		r := httptest.NewRequest("GET", "http://app.localhost:7070/", nil)
		r.Header.Set(feature.RelayTraceHeader, "trace")
		r.Header.Set(feature.LoopHeader, "loop")
		r.Header.Set("Authorization", "Bearer t")
		Proxy(httptest.NewRecorder(), r, target, c.options)
		if (seen.Get(feature.RelayTraceHeader) != "") != c.trace || (seen.Get(feature.LoopHeader) != "") != c.loopMeta {
			t.Fatalf("%s: trace=%q loop=%q", c.name, seen.Get(feature.RelayTraceHeader), seen.Get(feature.LoopHeader))
		}
		if seen.Get("Authorization") != "Bearer t" {
			t.Fatalf("%s: other headers must pass", c.name)
		}
	}
	if ResponseHeaderTimeout.Minutes() != 5 {
		t.Fatalf("ResponseHeaderTimeout = %v, want 5m", ResponseHeaderTimeout)
	}
}
