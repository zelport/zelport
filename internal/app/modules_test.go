package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/zelport/zelport/internal/feature"
)

type testModule struct {
	id             string
	host           feature.Host
	starts, closes int
	fail           bool
}

func (m *testModule) Descriptor() feature.Descriptor {
	return feature.Descriptor{Maturity: feature.Stable, ID: m.id, Name: m.id, TabID: m.id, Exportable: true}
}
func (m *testModule) ValidateRoute(r feature.Route) error {
	var v map[string]any
	return feature.Decode(r.Settings, &v)
}
func (m *testModule) ValidateSettings(b json.RawMessage) error {
	var v map[string]any
	return feature.Decode(b, &v)
}
func (m *testModule) Start(_ context.Context, h feature.Host) error {
	m.starts++
	m.host = h
	if m.fail {
		return errors.New("start failed")
	}
	return nil
}
func (m *testModule) Close() error         { m.closes++; return nil }
func (m *testModule) State() feature.State { return feature.State{} }
func (m *testModule) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	io.WriteString(w, v.Domain)
}
func (m *testModule) Admin() http.Handler { return http.NotFoundHandler() }
func setup(t *testing.T, modules ...feature.Module) *Manager {
	t.Helper()
	m, e := New(filepath.Join(t.TempDir(), "config.json"), 8080, modules...)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close() })
	return m
}
func TestEmptyRegistry(t *testing.T) {
	m := setup(t)
	if len(m.Descriptors()) != 0 {
		t.Fatal("core includes modules")
	}
}
func TestDuplicateModule(t *testing.T) {
	_, e := New("", 8080, &testModule{id: "x"}, &testModule{id: "x"})
	if e == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestModuleInitRollback(t *testing.T) {
	a, b := &testModule{id: "a"}, &testModule{id: "b", fail: true}
	m, e := New("", 8080, a, b)
	if e != nil {
		t.Fatal(e)
	}
	if m.Start(context.Background()) == nil {
		t.Fatal("accepted failed module")
	}
	m.Close()
	if a.closes != 1 || b.closes != 1 {
		t.Fatalf("cleanup: %d %d", a.closes, b.closes)
	}
}

func TestCommitRevisionConflict(t *testing.T) {
	a := &testModule{id: "x"}
	m := setup(t, a)
	r, e := m.Save(context.Background(), 0, feature.Route{Module: "x", Domain: "FOO.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	if r.Domain != "foo.localhost" || !feature.ValidUUID(r.ID) {
		t.Fatal(r)
	}
	if _, e = m.Save(context.Background(), 0, r); !errors.Is(e, feature.ErrConflict) {
		t.Fatal(e)
	}
	if _, e = m.Save(context.Background(), 1, feature.Route{Module: "x", Domain: r.Domain, Settings: json.RawMessage(`{}`)}); !errors.Is(e, feature.ErrConflict) {
		t.Fatal("collision", e)
	}
}
func TestScopeResetAcquireRace(t *testing.T) {
	a := &testModule{id: "x"}
	m := setup(t, a)
	r, e := m.Save(context.Background(), 0, feature.Route{Module: "x", Domain: "foo.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	scope := a.host.Scope("sharing")
	l, e := a.host.Acquire(context.Background(), r.ID, 1, scope)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.host.Commit(context.Background(), 1, feature.CommitOptions{ResetScopes: []string{"sharing"}}, func(d *feature.Draft) error { d.Settings = json.RawMessage(`{"publish":false}`); return nil }); e != nil {
		t.Fatal(e)
	}
	if l.Context().Err() == nil {
		t.Fatal("lease survived revocation")
	}
	if _, e = a.host.Acquire(context.Background(), r.ID, 2, scope); e == nil {
		t.Fatal("old scope acquired")
	}
}
func TestWatchCoalescing(t *testing.T) {
	a := &testModule{id: "x"}
	setup(t, a)
	ch, stop := a.host.Watch()
	defer stop()
	for i := uint64(0); i < 3; i++ {
		_, e := a.host.Commit(context.Background(), i, feature.CommitOptions{}, func(d *feature.Draft) error { return nil })
		if e != nil {
			t.Fatal(e)
		}
	}
	select {
	case <-ch:
	default:
		t.Fatal("missing notification")
	}
	select {
	case <-ch:
		t.Fatal("not coalesced")
	default:
	}
}
func TestDuplicateJSONKeys(t *testing.T) {
	var x any
	if feature.Decode([]byte(`{"x":1,"x":2}`), &x) == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestAtomicWriteFailureAndPersistence(t *testing.T) {
	a := &testModule{id: "x"}
	m := setup(t, a)
	r, e := m.Save(context.Background(), 0, feature.Route{Module: "x", Domain: "one.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	restored, e := New(m.path, 8080, &testModule{id: "x"})
	if e != nil {
		t.Fatal(e)
	}
	_, rs := restored.Snapshot()
	if len(rs) != 1 || rs[0].ID != r.ID {
		t.Fatal("identity not persisted")
	}
	m.path = filepath.Join(m.path, "invalid")
	r.Domain = "changed.localhost"
	if _, e = m.Save(context.Background(), 1, r); e == nil {
		t.Fatal("write unexpectedly succeeded")
	}
	rev, rs := m.Snapshot()
	if rev != 1 || rs[0].Domain != "one.localhost" {
		t.Fatal("failed write changed state")
	}
}
func TestSettingsCounterKeepsStreams(t *testing.T) {
	a := &testModule{id: "x"}
	m := setup(t, a)
	r, _ := m.Save(context.Background(), 0, feature.Route{Module: "x", Domain: "one.localhost", Settings: json.RawMessage(`{}`)})
	l, e := a.host.Acquire(context.Background(), r.ID, 1)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if _, e = a.host.Commit(context.Background(), 1, feature.CommitOptions{}, func(d *feature.Draft) error { d.Settings = json.RawMessage(`{"counter":1}`); return nil }); e != nil {
		t.Fatal(e)
	}
	if l.Context().Err() != nil {
		t.Fatal("counter revoked route")
	}
}
func TestInvalidAndLegacyConfig(t *testing.T) {
	for _, content := range []string{`{"version":5,"routes":[]}`, `{"version":6,"revision":0,"modules":{},"routes":[],"extra":true}`, `{"version":6,"version":6}`, `{"version":6} {}`} {
		p := filepath.Join(t.TempDir(), "c")
		os.WriteFile(p, []byte(content), 0600)
		if _, e := New(p, 8080); e == nil {
			t.Fatal("accepted", content)
		}
		b, _ := os.ReadFile(p)
		if string(b) != content {
			t.Fatal("overwritten")
		}
	}
}

func TestMetadataCommitsDoNotInvalidateUserEdits(t *testing.T) {
	a := &testModule{id: "x"}
	m := setup(t, a)
	r, e := m.Save(context.Background(), 0, feature.Route{Module: "x", Domain: "one.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.host.Commit(context.Background(), 1, feature.CommitOptions{MetadataOnly: true}, func(d *feature.Draft) error { d.Settings = json.RawMessage(`{"counter":1}`); return nil })
	if e != nil {
		t.Fatal(e)
	}
	// Internal writers still require exact CAS; user edits can cross metadata-only commits.
	if _, e = a.host.Commit(context.Background(), 1, feature.CommitOptions{MetadataOnly: true}, func(d *feature.Draft) error { return nil }); !errors.Is(e, feature.ErrConflict) {
		t.Fatal("internal stale write accepted", e)
	}
	if _, e = a.host.Commit(context.Background(), 1, feature.CommitOptions{StrictRevision: true}, func(d *feature.Draft) error { t.Fatal("stale effective snapshot entered mutation"); return nil }); !errors.Is(e, feature.ErrConflict) {
		t.Fatal("strict effective write crossed metadata", e)
	}
	r.Domain = "two.localhost"
	if _, e = m.Save(context.Background(), 1, r); e != nil {
		t.Fatal("background metadata blocked editor", e)
	}
	if _, e = m.Save(context.Background(), 1, r); !errors.Is(e, feature.ErrConflict) {
		t.Fatal("concurrent user edit accepted", e)
	}
}
