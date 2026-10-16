package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zelport/zelport/internal/feature"
)

func TestMissingModulePreservedAndRestored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	a, b := &testModule{id: "present"}, &testModule{id: "missing"}
	full, err := New(path, 8080, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = full.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved, err := full.Save(context.Background(), 0, feature.Route{Module: "missing", Domain: "saved.localhost", Settings: json.RawMessage(`{"pin":"stable","secret":"preserve"}`)})
	if err != nil {
		t.Fatal(err)
	}
	rev, _ := full.Snapshot()
	if _, err = b.host.Commit(context.Background(), rev, feature.CommitOptions{}, func(d *feature.Draft) error { d.Settings = json.RawMessage(`{"credential":"keep"}`); return nil }); err != nil {
		t.Fatal(err)
	}
	full.Close()
	before, _ := os.ReadFile(path)
	reduced, err := New(path, 8080, &testModule{id: "present"})
	if err != nil {
		t.Fatal(err)
	}
	defer reduced.Close()
	if err = reduced.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("startup rewrote configuration")
	}
	req := httptest.NewRequest("GET", "http://saved.localhost:8080/", nil)
	w := httptest.NewRecorder()
	reduced.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("inactive route: %d", w.Code)
	}
	rev, _ = reduced.Snapshot()
	if _, err = reduced.acquire(context.Background(), saved.ID, &rev, true, nil); err == nil {
		t.Fatal("exported inactive route")
	}
	if len((&host{m: reduced, id: "present"}).OwnServices()) != 0 {
		t.Fatal("advertised inactive route")
	}
	if _, err = reduced.Save(context.Background(), rev, feature.Route{Module: "present", Domain: saved.Domain, Settings: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("inactive domain was reused")
	}
	if _, err = reduced.Save(context.Background(), rev, feature.Route{Module: "missing", Domain: "new.localhost", Settings: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("created unvalidated missing route")
	}
	if _, err = reduced.Save(context.Background(), rev, saved); err == nil {
		t.Fatal("edited missing route")
	}
	if _, err = reduced.Save(context.Background(), rev, feature.Route{Module: "present", Domain: "active.localhost", Settings: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	c, _, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(c.Modules["missing"]) != `{"credential":"keep"}` { // Compare JSON semantically below; persistence indents values.
		var got map[string]string
		json.Unmarshal(c.Modules["missing"], &got)
		if got["credential"] != "keep" {
			t.Fatal("lost settings")
		}
	}
	var found feature.Route
	for _, r := range c.Routes {
		if r.ID == saved.ID {
			found = r
		}
	}
	var wantSettings, gotSettings any
	json.Unmarshal(saved.Settings, &wantSettings)
	json.Unmarshal(found.Settings, &gotSettings)
	if found.ID != saved.ID || found.Domain != saved.Domain || !reflect.DeepEqual(wantSettings, gotSettings) {
		t.Fatal("lost inactive route")
	}
	restored, err := New(path, 8080, &testModule{id: "present"}, &testModule{id: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.Start(context.Background())
	w = httptest.NewRecorder()
	restored.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != saved.Domain {
		t.Fatal("route did not recover", w.Code, w.Body)
	}
}

func TestMissingModuleStillValidatesConfigurationEnvelope(t *testing.T) {
	for _, raw := range []string{
		`{"version":5,"modules":{"missing":{}},"routes":[]}`,
		`{"version":6,"modules":{"bad-id":{}},"routes":[]}`,
		`{"version":6,"modules":{"missing":{}},"routes":[{"id":"bad","module":"missing","domain":"valid.localhost","settings":{}}]}`,
		`{"version":6,"modules":{"present":[]},"routes":[]}`,
		`{"version":6,`,
	} {
		t.Run(raw, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "c.json")
			os.WriteFile(path, []byte(raw), 0600)
			m, err := New(path, 8080, &testModule{id: "present"})
			if err == nil {
				m.Close()
				t.Fatal("accepted invalid configuration")
			}
		})
	}
}
