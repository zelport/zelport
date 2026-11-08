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

func TestProxyAddsViaBothWaysAndKeepsTheRequestID(t *testing.T) {
	var seen http.Header
	answerID := ""
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		w.Header().Set("Via", "1.1 cache")
		if answerID != "" {
			w.Header().Set(feature.RequestIDHeader, answerID)
		}
	}))
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	for _, external := range []bool{false, true} {
		for _, own := range []string{"", "service-id"} {
			answerID = own
			r := httptest.NewRequest("GET", "http://app.localhost:7070/", nil)
			r.Header.Set("Via", "2 edge")
			w := httptest.NewRecorder()
			r = feature.AssignRequestID(w, r)
			id := r.Header.Get(feature.RequestIDHeader)
			Proxy(w, r, target, ProxyOptions{ListenerAuthority: "localhost:1", External: external})
			if got := seen.Get("Via"); got != "2 edge, 1.1 zelport" {
				t.Fatalf("request Via = %q", got)
			}
			if got := w.Header().Get("Via"); got != "1.1 cache, 1.1 zelport" {
				t.Fatalf("answer Via = %q", got)
			}
			if seen.Get(feature.RequestIDHeader) != id {
				t.Fatalf("service got request ID %q, want %q", seen.Get(feature.RequestIDHeader), id)
			}
			want := id
			if own != "" {
				want = own
			}
			if got := w.Header().Values(feature.RequestIDHeader); len(got) != 1 || got[0] != want {
				t.Fatalf("answer request ID = %q, want %q", got, want)
			}
		}
	}
}

func TestProxyErrorPageCarriesTheRequestID(t *testing.T) {
	r := httptest.NewRequest("GET", "http://app.localhost:7070/", nil)
	w := httptest.NewRecorder()
	r = feature.AssignRequestID(w, r)
	Proxy(w, r, &url.URL{Scheme: "http", Host: "127.0.0.1:1"}, ProxyOptions{ListenerAuthority: "localhost:2"})
	if w.Code != 502 || w.Header().Get(feature.RequestIDHeader) != r.Header.Get(feature.RequestIDHeader) {
		t.Fatalf("code %d, request ID %q", w.Code, w.Header().Get(feature.RequestIDHeader))
	}
	if w.Header().Get("Via") != "" {
		t.Fatal("Zelport's own answer is not forwarded and carries no Via")
	}
}
