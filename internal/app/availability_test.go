package app

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/feature"
	"os"
	"path/filepath"
	"testing"
)

type maturityModule struct {
	testModule
	maturity string
}

func (m *maturityModule) Descriptor() feature.Descriptor {
	d := m.testModule.Descriptor()
	d.Maturity = m.maturity
	return d
}
func (m *maturityModule) UnavailableReason(raw json.RawMessage) string {
	var v struct {
		Enabled bool `json:"enabled"`
	}
	json.Unmarshal(raw, &v)
	if v.Enabled {
		return "Turn off the connection before disabling this module."
	}
	return ""
}
func TestAvailabilityDefaultsPersistenceAndGuards(t *testing.T) {
	stable := &maturityModule{testModule: testModule{id: "stable"}, maturity: "stable"}
	experimental := &maturityModule{testModule: testModule{id: "experimental"}, maturity: "experimental"}
	m := setup(t, stable, experimental)
	if !m.Availability()["stable"].Enabled || m.Availability()["experimental"].Enabled {
		t.Fatal("maturity defaults incorrect")
	}
	rev, _ := m.Snapshot()
	if _, e := m.SetAvailability(context.Background(), rev, "stable", false); e != nil {
		t.Fatal(e)
	}
	if stable.starts != 1 || stable.closes != 0 {
		t.Fatal("visibility changed backend lifecycle")
	}
	rev, _ = m.Snapshot()
	if _, e := m.SetAvailability(context.Background(), rev, "experimental", true); e != nil {
		t.Fatal(e)
	}
	m.Close()
	experimental.maturity = "stable"
	stable.maturity = "experimental"
	restarted, e := New(m.path, 8080, stable, experimental)
	if e != nil {
		t.Fatal(e)
	}
	defer restarted.Close()
	if restarted.Availability()["stable"].Enabled || !restarted.Availability()["experimental"].Enabled {
		t.Fatal("explicit choices lost across maturity change/restart")
	}
}
func TestAvailabilityRoutesAndConnectionForceVisibility(t *testing.T) {
	mod := &maturityModule{testModule: testModule{id: "test"}, maturity: "experimental"}
	m := setup(t, mod)
	rev, _ := m.Snapshot()
	route, e := m.Save(context.Background(), rev, feature.Route{Module: "test", Domain: "unavailable.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	a := m.Availability()["test"]
	if !a.Enabled || a.CanDisable || a.Services != 1 || a.Reason == "" {
		t.Fatal(a)
	}
	rev, _ = m.Snapshot()
	if _, e = m.SetAvailability(context.Background(), rev, "test", false); e == nil {
		t.Fatal("route-bearing module disabled")
	}
	if e = m.Delete(context.Background(), rev, route.ID); e != nil {
		t.Fatal(e)
	}
	rev, _ = m.Snapshot()
	_, e = mod.host.Commit(context.Background(), rev, feature.CommitOptions{}, func(d *feature.Draft) error { d.Settings = json.RawMessage(`{"enabled":true}`); return nil })
	if e != nil {
		t.Fatal(e)
	}
	a = m.Availability()["test"]
	if !a.Enabled || a.CanDisable {
		t.Fatal("enabled connection not protected", a)
	}
	rev, _ = m.Snapshot()
	if _, e = m.SetAvailability(context.Background(), rev, "test", false); e == nil {
		t.Fatal("connected module disabled")
	}
}
func TestAvailabilityConflictAndPersistenceFailure(t *testing.T) {
	m := setup(t, &testModule{id: "test"})
	rev, _ := m.Snapshot()
	if _, e := m.SetAvailability(context.Background(), rev, "test", true); e != nil {
		t.Fatal(e)
	}
	if _, e := m.SetAvailability(context.Background(), rev, "test", false); e != feature.ErrConflict {
		t.Fatal("stale write accepted", e)
	}
	rev, _ = m.Snapshot()
	if e := os.Rename(m.path, m.path+".saved"); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(m.path, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := m.SetAvailability(context.Background(), rev, "test", false); e == nil {
		t.Fatal("write failure ignored")
	}
	if !m.Availability()["test"].Enabled {
		t.Fatal("failed write changed visibility")
	}
	info, e := os.Stat(filepath.Join(filepath.Dir(m.path), "config.json.saved"))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("permissions", e)
	}
}

func TestMaturityRequired(t *testing.T) {
	for _, value := range []string{"", "beta", "Stable"} {
		if _, e := New("", 8080, &maturityModule{testModule: testModule{id: "test"}, maturity: value}); e == nil {
			t.Fatal("invalid maturity accepted", value)
		}
	}
}

func TestAvailabilityConcurrentRouteCreation(t *testing.T) {
	m := setup(t, &testModule{id: "test"})
	rev, _ := m.Snapshot()
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; _, e := m.SetAvailability(context.Background(), rev, "test", false); results <- e }()
	go func() {
		<-start
		_, e := m.Save(context.Background(), rev, feature.Route{Module: "test", Domain: "race.localhost", Settings: json.RawMessage(`{}`)})
		results <- e
	}()
	close(start)
	var success, conflict int
	for range 2 {
		e := <-results
		if e == nil {
			success++
		} else if e == feature.ErrConflict {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent edits were not serialized")
	}
	_, routes := m.Snapshot()
	if len(routes) > 0 && !m.Availability()["test"].Enabled {
		t.Fatal("route created inside hidden module")
	}
}
