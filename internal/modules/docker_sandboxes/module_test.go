package docker_sandboxes

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/sbx"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeBackend struct {
	mu                 sync.Mutex
	running            bool
	fail               bool
	failUnpublish      bool
	published, started int
	ports              []sbx.Port
}

func (f *fakeBackend) List(context.Context) ([]sbx.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return nil, errors.New("offline")
	}
	state := "stopped"
	if f.running {
		state = "running"
	}
	return []sbx.Sandbox{{ID: "stable-id", Name: "example", Status: state}}, nil
}
func (f *fakeBackend) Ports(context.Context, string) ([]sbx.Port, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sbx.Port(nil), f.ports...), nil
}
func (f *fakeBackend) Publish(context.Context, string, int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published++
	f.ports = []sbx.Port{{HostIP: "127.0.0.1", HostPort: 45678, SandboxPort: 3000, Protocol: "tcp"}}
	return nil
}
func (f *fakeBackend) Start(ctx context.Context, id string, p int) error {
	f.mu.Lock()
	f.running = true
	f.started++
	f.mu.Unlock()
	return f.Publish(ctx, id, p)
}
func (f *fakeBackend) Unpublish(context.Context, string, sbx.Port) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failUnpublish {
		return errors.New("unpublish failed")
	}
	f.ports = nil
	return nil
}
func TestStoppedSaveThenReconcileWithoutDuplicates(t *testing.T) {
	b := &fakeBackend{}
	mod := New().(*Module)
	mod.Backend = b
	m, e := app.New("", 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	mod.Refresh(context.Background())
	s, _ := json.Marshal(Settings{SandboxID: "stable-id", SandboxPort: 3000})
	rev, _ := m.Snapshot()
	_, e = m.Save(context.Background(), rev, feature.Route{Module: "docker_sandboxes", Domain: "box.localhost", Settings: s})
	if e != nil {
		t.Fatal(e)
	}
	mod.Refresh(context.Background())
	b.mu.Lock()
	if b.published != 0 || b.started != 0 {
		t.Fatal("saved stopped sandbox started")
	}
	b.running = true
	b.mu.Unlock()
	mod.Refresh(context.Background())
	mod.Refresh(context.Background())
	b.mu.Lock()
	if b.published != 1 {
		t.Fatalf("publications %d", b.published)
	}
	b.fail = true
	b.mu.Unlock()
	mod.Refresh(context.Background())
	if mod.State().Routes[0].Summary.State != "stale" {
		t.Fatal("outage discarded mapping")
	}
}
func TestSavedSourceCannotBeReboundOrRevivePendingRemoval(t *testing.T) {
	b := &fakeBackend{}
	mod := New().(*Module)
	mod.Backend = b
	m, e := app.New("", 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	mod.Refresh(context.Background())
	settings := Settings{SandboxID: "stable-id", SandboxPort: 3000}
	data, _ := json.Marshal(settings)
	r, e := m.Save(context.Background(), 0, feature.Route{Module: "docker_sandboxes", Domain: "box.localhost", Settings: data})
	if e != nil {
		t.Fatal(e)
	}
	settings.Deleting = true
	r.Settings, _ = json.Marshal(settings)
	r, e = m.Save(context.Background(), 1, r)
	if e != nil {
		t.Fatal(e)
	}
	settings.Deleting = false
	r.Settings, _ = json.Marshal(settings)
	if _, e = m.Save(context.Background(), 2, r); e == nil {
		t.Fatal("revived pending removal")
	}
	settings.Deleting = true
	settings.SandboxID = "another-id"
	r.Settings, _ = json.Marshal(settings)
	if _, e = m.Save(context.Background(), 2, r); e == nil {
		t.Fatal("changed saved sandbox identity")
	}
}

func TestExplicitStartDoesNotRepublishRunningSandbox(t *testing.T) {
	b := &fakeBackend{}
	mod := New().(*Module)
	mod.Backend = b
	m, err := app.New("", 8080, mod)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	mod.Refresh(context.Background())
	data, _ := json.Marshal(Settings{SandboxID: "stable-id", SandboxPort: 3000})
	route, err := m.Save(context.Background(), 0, feature.Route{Module: "docker_sandboxes", Domain: "box.localhost", Settings: data})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		req := httptest.NewRequest("POST", "/start", strings.NewReader(`{"revision":1,"id":"`+route.ID+`"}`))
		w := httptest.NewRecorder()
		mod.Admin().ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started != 1 || b.published != 1 {
		t.Fatalf("started %d, published %d", b.started, b.published)
	}
}

func TestSharedMappingAndFailedRemoval(t *testing.T) {
	b := &fakeBackend{running: true, ports: []sbx.Port{{HostIP: "127.0.0.1", HostPort: 45678, SandboxPort: 3000, Protocol: "tcp"}}}
	mod := New().(*Module)
	mod.Backend = b
	m, err := app.New("", 8080, mod)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	mod.Refresh(context.Background())
	data, _ := json.Marshal(Settings{SandboxID: "stable-id", SandboxPort: 3000})
	a, err := m.Save(context.Background(), 0, feature.Route{Module: "docker_sandboxes", Domain: "one.localhost", Settings: data})
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.Save(context.Background(), 1, feature.Route{Module: "docker_sandboxes", Domain: "two.localhost", Settings: data})
	if err != nil {
		t.Fatal(err)
	}
	remove := func(rev uint64) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(map[string]any{"revision": rev, "id": a.ID})
		w := httptest.NewRecorder()
		mod.Admin().ServeHTTP(w, httptest.NewRequest("POST", "/remove-mapping", strings.NewReader(string(payload))))
		return w
	}
	if w := remove(2); w.Code != 400 || !strings.Contains(w.Body.String(), "another domain") {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := m.Delete(context.Background(), 2, other.ID); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	b.failUnpublish = true
	b.mu.Unlock()
	if w := remove(3); w.Code != 400 || !strings.Contains(w.Body.String(), "removal pending") {
		t.Fatal(w.Code, w.Body.String())
	}
	rev, routes := m.Snapshot()
	if len(routes) != 1 {
		t.Fatal(routes)
	}
	var pending Settings
	json.Unmarshal(routes[0].Settings, &pending)
	if !pending.Deleting {
		t.Fatal("intent was not persisted")
	}
	mod.Refresh(context.Background())
	b.mu.Lock()
	if b.published != 0 {
		t.Fatal("pending route republished")
	}
	b.failUnpublish = false
	b.mu.Unlock()
	if w := remove(rev); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	_, routes = m.Snapshot()
	if len(routes) != 0 {
		t.Fatal("route retained after successful removal")
	}
	mod.Refresh(context.Background())
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.published != 0 || len(b.ports) != 0 {
		t.Fatal("removed mapping reappeared")
	}
}
