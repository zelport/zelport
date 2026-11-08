package feature

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestRequestIDKeepsAnIDThenTakesTheTraceIDThenPicksOne(t *testing.T) {
	const trace = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	random := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for _, c := range []struct {
		name      string
		ids       []string
		parent    []string
		want      string
		generated bool
	}{
		{"own id", []string{"req-42"}, nil, "req-42", false},
		{"own id wins over trace", []string{"req-42"}, []string{trace}, "req-42", false},
		{"trace id", nil, []string{trace}, "4bf92f3577b34da6a3ce929d0e0e4736", false},
		{"future version with more fields", nil, []string{"cc-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra"}, "4bf92f3577b34da6a3ce929d0e0e4736", false},
		{"invalid id falls to trace", []string{"two words"}, []string{trace}, "4bf92f3577b34da6a3ce929d0e0e4736", false},
		{"too long id", []string{strings.Repeat("a", 129)}, nil, "", true},
		{"line break", []string{"a\nb"}, nil, "", true},
		{"conflicting ids", []string{"a", "b"}, nil, "", true},
		{"nothing", nil, nil, "", true},
		{"zero trace id", nil, []string{"00-00000000000000000000000000000000-00f067aa0ba902b7-01"}, "", true},
		{"zero parent id", nil, []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01"}, "", true},
		{"version ff", nil, []string{"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"}, "", true},
		{"uppercase", nil, []string{"00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01"}, "", true},
		{"version 00 with more fields", nil, []string{trace + "-x"}, "", true},
		{"two traceparents", nil, []string{trace, trace}, "", true},
	} {
		h := http.Header{}
		for _, v := range c.ids {
			h.Add(RequestIDHeader, v)
		}
		for _, v := range c.parent {
			h.Add(TraceparentHeader, v)
		}
		got := RequestID(h)
		if c.generated && !random.MatchString(got) || !c.generated && got != c.want {
			t.Fatalf("%s: got %q", c.name, got)
		}
	}
	if RequestID(http.Header{}) == RequestID(http.Header{}) {
		t.Fatal("generated IDs must differ")
	}
}

func TestAssignRequestIDSetsRequestAndAnswerAndKeepsIt(t *testing.T) {
	r := httptest.NewRequest("GET", "http://app.localhost/", nil)
	w := httptest.NewRecorder()
	r = AssignRequestID(w, r)
	id := r.Header.Get(RequestIDHeader)
	if id == "" || w.Header().Get(RequestIDHeader) != id {
		t.Fatalf("request %q answer %q", id, w.Header().Get(RequestIDHeader))
	}
	if again := AssignRequestID(w, r); again != r || w.Header().Get(RequestIDHeader) != id {
		t.Fatal("a second call must keep the request and its ID")
	}
}
