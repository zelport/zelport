package feature

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestStatusDiagnosticKeepsRawErrorOutOfSummary(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	StatusDiagnostic(w, r, 502, "SSH service unavailable", "The SSH connection could not be established.", "SSH: workspace ocpp-ws-server doesn't exist")

	if w.Code != 502 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"SSH service unavailable", "The SSH connection could not be established.", "<details", "Technical details", "SSH: workspace ocpp-ws-server"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "<p>SSH: workspace") {
		t.Fatalf("raw diagnostic is visible in main message: %s", body)
	}
}

func TestStatusDiagnosticEscapesTechnicalOutput(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	StatusDiagnostic(w, r, 502, "Service unavailable", "The service could not be reached.", "<script>alert(1)</script>")
	body := w.Body.String()
	if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("unsafe diagnostic HTML: %s", body)
	}
}

func TestStatusDiagnosticRemovesTerminalSequencesFromHTMLAndJSON(t *testing.T) {
	for _, accept := range []string{"text/html", "application/json"} {
		t.Run(accept, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Accept", accept)
			StatusDiagnostic(w, r, http.StatusServiceUnavailable, "SSH service unavailable", "The SSH connection could not be established.", "\x1b[31mfatal\x1b[0m workspace missing")
			body := w.Body.String()
			if strings.Contains(body, "[31m") || strings.Contains(body, "[0m") || !strings.Contains(body, "fatal workspace missing") {
				t.Fatalf("terminal escapes in %s response: %q", accept, body)
			}
		})
	}
}

type pipeWriter struct {
	http.ResponseWriter
	c net.Conn
}

func (w pipeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.c, bufio.NewReadWriter(bufio.NewReader(w.c), bufio.NewWriter(w.c)), nil
}
func TestHijackCloseRace(t *testing.T) {
	for i := 0; i < 10000; i++ {
		a, b := net.Pipe()
		l := NewLease(context.Background(), Route{}, 1, nil)
		w := Writer{ResponseWriter: pipeWriter{httptest.NewRecorder(), a}, Lease: l}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); l.Close() }()
		c, _, _ := w.Hijack()
		if c != nil {
			c.Close()
		}
		wg.Wait()
		b.Close()
	}
}
