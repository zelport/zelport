package local_ports

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
)

func discoveryRequest(m *Module, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	m.Admin().ServeHTTP(w, httptest.NewRequest(method, "http://localhost:8080"+path, strings.NewReader(body)))
	return w
}
func discoveryManager(t *testing.T, scan listenerScanner) (*Module, *app.Manager, string) {
	t.Helper()
	mod := newModule(scan)
	path := filepath.Join(t.TempDir(), "config.json")
	manager, err := app.New(path, 8080, mod)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	return mod, manager, path
}
func TestDiscoveryDefaultsFiltersAndConfiguredMetadata(t *testing.T) {
	scans := 0
	mod, manager, _ := discoveryManager(t, listenerScannerFunc(func(context.Context) ([]rawListener, error) {
		scans++
		var records []rawListener
		for _, v := range []struct {
			port int
			name string
		}{{22, "sshd"}, {3000, "node"}, {8001, "api"}, {9000, "SSHD"}, {9001, "sshd"}, {9001, "web"}, {9002, ""}, {9003, "sshd"}, {9003, ""}} {
			records = append(records, rawListener{Address: netip.IPv4Unspecified(), Port: v.port, Process: v.name})
		}
		return records, nil
	}))
	var defaults struct {
		Enabled bool `json:"discovery_enabled"`
	}
	json.Unmarshal(mod.State().Data, &defaults)
	if !defaults.Enabled {
		t.Fatal("default disabled")
	}
	rev, _ := manager.Snapshot()
	route, err := manager.Save(context.Background(), rev, feature.Route{Module: "local_ports", Domain: "kept.localhost", Settings: json.RawMessage(`{"aux_port":3000}`)})
	if err != nil {
		t.Fatal(err)
	}
	rev, _ = manager.Snapshot()
	raw, _ := json.Marshal(map[string]any{"revision": rev, "discovery_enabled": true, "ignored_ports": []string{"3000", "8000-8099"}, "ignored_processes": []string{"sshd"}})
	if w := discoveryRequest(mod, "PUT", "/settings", string(raw)); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	response := discoveryRequest(mod, "GET", "/listeners", "")
	var result struct {
		Listeners  []Listener `json:"listeners"`
		Configured []Listener `json:"configured_listeners"`
		Enabled    bool       `json:"discovery_enabled"`
	}
	json.Unmarshal(response.Body.Bytes(), &result)
	var ports []int
	for _, l := range result.Listeners {
		ports = append(ports, l.Port)
	}
	if !reflect.DeepEqual(ports, []int{9001, 9002, 9003}) || !result.Enabled || scans != 1 {
		t.Fatalf("filtered: %+v scans %d", result, scans)
	}
	if len(result.Configured) != 1 || result.Configured[0].Port != 3000 || !result.Configured[0].ListensBeyondLoopback {
		t.Fatalf("configured status lost: %+v", result.Configured)
	}
	rev, _ = manager.Snapshot()
	raw, _ = json.Marshal(map[string]any{"revision": rev, "discovery_enabled": false, "ignored_ports": []string{"3000"}, "ignored_processes": []string{"sshd"}})
	if w := discoveryRequest(mod, "PUT", "/settings", string(raw)); w.Code != 200 {
		t.Fatalf("disable: %d %s", w.Code, w.Body)
	}
	response = discoveryRequest(mod, "GET", "/listeners", "")
	json.Unmarshal(response.Body.Bytes(), &result)
	if scans != 1 || result.Enabled || len(result.Listeners) != 0 || len(result.Configured) != 0 {
		t.Fatalf("disabled scanner: %+v scans %d", result, scans)
	}
	_, routes := manager.Snapshot()
	if len(routes) != 1 || routes[0].ID != route.ID {
		t.Fatal("route changed")
	}
	// Manual service creation still works when its port is excluded and discovery is off.
	rev, _ = manager.Snapshot()
	if _, err = manager.Save(context.Background(), rev, feature.Route{Module: "local_ports", Domain: "manual.localhost", Settings: json.RawMessage(`{"aux_port":3000}`)}); err != nil {
		t.Fatal(err)
	}
}
func TestDiscoverySettingsValidatePersistAndRejectStaleWrites(t *testing.T) {
	mod, manager, path := discoveryManager(t, staticListenerScanner{})
	for _, bad := range []string{`{"ignored_ports":["0"]}`, `{"ignored_ports":["65536"]}`, `{"ignored_ports":["9000-8000"]}`, `{"ignored_ports":["1-2-3"]}`, `{"ignored_ports":["+80"]}`, `{"ignored_processes":["bad\nname"]}`, `{"ignored_processes":[""]}`, `{"other":true}`} {
		if err := mod.ValidateSettings(json.RawMessage(bad)); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	many := make([]string, 129)
	for i := range many {
		many[i] = "80"
	}
	raw, _ := json.Marshal(map[string]any{"ignored_ports": many})
	if err := mod.ValidateSettings(raw); err == nil {
		t.Fatal("oversize list accepted")
	}
	rev, _ := manager.Snapshot()
	raw, _ = json.Marshal(map[string]any{"revision": rev, "discovery_enabled": false, "ignored_ports": []string{" 80 ", "8000 - 8099"}, "ignored_processes": []string{" sshd "}})
	if w := discoveryRequest(mod, "PUT", "/settings", string(raw)); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	if w := discoveryRequest(mod, "PUT", "/settings", string(raw)); w.Code != 409 {
		t.Fatalf("stale save: %d %s", w.Code, w.Body)
	}
	if w := discoveryRequest(mod, "PUT", "/settings", `{"revision":1,"discovery_enabled":null}`); w.Code != 400 {
		t.Fatalf("null flag: %d", w.Code)
	}
	contents, _ := os.ReadFile(path)
	if !strings.Contains(string(contents), `"discovery_enabled": false`) {
		t.Fatalf("not persisted: %s", contents)
	}
	loaded := newModule(staticListenerScanner{})
	restart, err := app.New(path, 8080, loaded)
	if err != nil {
		t.Fatal(err)
	}
	defer restart.Close()
	if err = restart.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var state struct {
		Enabled   bool     `json:"discovery_enabled"`
		Ports     []string `json:"ignored_ports"`
		Processes []string `json:"ignored_processes"`
	}
	json.Unmarshal(loaded.State().Data, &state)
	if state.Enabled || !reflect.DeepEqual(state.Ports, []string{"80", "8000-8099"}) || !reflect.DeepEqual(state.Processes, []string{"sshd"}) {
		t.Fatalf("restart: %+v", state)
	}
}
