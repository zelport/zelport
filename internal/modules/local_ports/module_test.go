package local_ports

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

type listenerScannerFunc func(context.Context) ([]rawListener, error)

func (f listenerScannerFunc) Scan(ctx context.Context) ([]rawListener, error) { return f(ctx) }

func TestLocalApplicationResponse(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "demo.localhost:8080" || r.URL.RequestURI() != "/x?q=1" {
			t.Errorf("request: %s %s", r.Host, r.URL)
		}
		w.WriteHeader(403)
		io.WriteString(w, "application response")
	}))
	defer backend.Close()
	_, port, _ := net.SplitHostPort(backend.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	m, e := app.New("", 8080, New())
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	b, _ := json.Marshal(map[string]int{"aux_port": n})
	_, e = m.Save(context.Background(), 0, feature.Route{Module: "local_ports", Domain: "demo.localhost", Settings: b})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "http://demo.localhost:8080/x?q=1", nil)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 403 || w.Body.String() != "application response" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestListenersAPIReturnsStableInventoryAndExcludesZelport(t *testing.T) {
	type contextKey struct{}
	requestContext := context.WithValue(context.Background(), contextKey{}, "present")
	mod := newModule(listenerScannerFunc(func(ctx context.Context) ([]rawListener, error) {
		if ctx.Value(contextKey{}) != "present" {
			t.Fatal("request context value was not passed to scanner")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 2*time.Second {
			t.Fatalf("scanner deadline = %v, present = %v", deadline, ok)
		}
		return []rawListener{
			{Address: netip.IPv4Unspecified(), Port: 3000},
			{Address: netip.MustParseAddr("127.0.0.1"), Port: 9090, PID: 1, Process: "zelport"},
		}, nil
	}))
	manager, err := app.New("", 9090, mod)
	if err != nil {
		t.Fatal(err)
	}
	manager.Start(context.Background())
	defer manager.Close()

	request := httptest.NewRequest(http.MethodGet, "/listeners", nil).WithContext(requestContext)
	response := httptest.NewRecorder()
	mod.Admin().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	want := "{\"listeners\":[{\"port\":3000,\"processes\":[],\"listens_beyond_loopback\":true}]}\n"
	if response.Body.String() != want {
		t.Fatalf("body = %q, want %q", response.Body.String(), want)
	}
}

func TestListenersAPIRejectsMutationMethods(t *testing.T) {
	mod := newModule(staticListenerScanner{})
	response := httptest.NewRecorder()
	mod.Admin().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/listeners", strings.NewReader("{}")))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestListenersAPIReturnsSafeUnavailableResponse(t *testing.T) {
	mod := newModule(staticListenerScanner{err: errors.New("secret proc diagnostic")})
	response := httptest.NewRecorder()
	mod.Admin().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/listeners", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if body := response.Body.String(); body != "{\"error\":\"Listening ports couldn't be detected. Enter the Aux port manually.\"}\n" || strings.Contains(body, "secret") {
		t.Fatalf("unsafe or unexpected body: %q", body)
	}
}
