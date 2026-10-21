package local_ports

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/app"
	engine "github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/portorigin"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type listenerScannerFunc func(context.Context) ([]rawListener, error)

func (f listenerScannerFunc) Scan(ctx context.Context) ([]rawListener, error) { return f(ctx) }

type fakeAttributionCatalog struct {
	mu          sync.Mutex
	snapshot    portorigin.Snapshot
	refreshDone chan struct{}
	starts      int
	refreshes   int
	closes      int
}

func (c *fakeAttributionCatalog) Start(context.Context, time.Duration) {
	c.mu.Lock()
	c.starts++
	c.mu.Unlock()
}

func (c *fakeAttributionCatalog) Snapshot() portorigin.Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot
}

func (c *fakeAttributionCatalog) RequestRefresh() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshes++
	return c.refreshDone
}

func (c *fakeAttributionCatalog) Close() error {
	c.mu.Lock()
	c.closes++
	c.mu.Unlock()
	return nil
}

func TestLocalPortAttributionLifecycleUsesCachedSnapshotWithoutWaitingForRefresh(t *testing.T) {
	refreshDone := make(chan struct{})
	catalog := &fakeAttributionCatalog{
		refreshDone: refreshDone,
		snapshot: portorigin.Snapshot{
			Health: portorigin.Ready, Generation: 7, UpdatedAt: time.Unix(1700000000, 0).UTC(),
			Containers: []engine.Inventory{{
				Engine: "podman", Connection: "podman-machine-default", CLIPath: "/opt/podman/bin/podman", RuntimeStatus: "available",
				Containers: []engine.Container{{ID: "podman-id", Name: "demo", Status: "running", Ports: []engine.Port{
					{HostIP: "127.0.0.1", PublishedIP: "0.0.0.0", HostPort: 1212, ContainerPort: 80, Protocol: "tcp"},
					{HostIP: "127.0.0.1", PublishedIP: "0.0.0.0", HostPort: 1313, ContainerPort: 81, Protocol: "tcp"},
				}}},
			}},
		},
	}
	mod := newModule(listenerScannerFunc(func(context.Context) ([]rawListener, error) {
		return []rawListener{
			{Address: netip.IPv4Unspecified(), Port: 1212, PID: 4973, Process: "gvproxy"},
			{Address: netip.IPv4Unspecified(), Port: 1313, PID: 4973, Process: "gvproxy"},
		}, nil
	}))
	mod.catalog = catalog
	manager, err := app.New("", 9090, mod)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	response := httptest.NewRecorder()
	mod.Admin().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/listeners", nil))
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("cached GET took %v", elapsed)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Listeners             []Listener `json:"listeners"`
		AttributionHealth     string     `json:"attribution_health"`
		AttributionGeneration uint64     `json:"attribution_generation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.AttributionHealth != SourceReady || body.AttributionGeneration != 7 || len(body.Listeners) != 2 {
		t.Fatalf("body = %#v", body)
	}
	for _, listener := range body.Listeners {
		if listener.Kind != KindContainer || len(listener.Containers) != 1 || listener.Containers[0].Engine != "podman" {
			t.Fatalf("listener = %#v", listener)
		}
	}

	postDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		refresh := httptest.NewRecorder()
		mod.Admin().ServeHTTP(refresh, httptest.NewRequest(http.MethodPost, "/listeners/refresh", nil))
		postDone <- refresh
	}()
	select {
	case <-postDone:
		t.Fatal("refresh response returned before the catalog generation completed")
	case <-time.After(25 * time.Millisecond):
	}
	close(refreshDone)
	refresh := <-postDone
	if refresh.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %s", refresh.Code, refresh.Body.String())
	}

	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if catalog.starts != 1 || catalog.refreshes != 1 || catalog.closes != 1 {
		t.Fatalf("lifecycle starts=%d refreshes=%d closes=%d", catalog.starts, catalog.refreshes, catalog.closes)
	}
}

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
	want := "{\"attribution_health\":\"ready\",\"attribution_generation\":0,\"listeners\":[{\"port\":3000,\"processes\":[],\"kind\":\"local_process\",\"source_status\":\"ready\",\"listens_beyond_loopback\":true}]}\n"
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
