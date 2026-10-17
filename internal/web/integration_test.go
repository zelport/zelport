package web

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"github.com/zelport/zelport/internal/modules/local_ports"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestIntegrationSecurity(t *testing.T) {
	m, _ := app.New("", 8080)
	defer m.Close()
	s, _ := New(m, 8080)
	const path = "/api/integration/v1/state"
	for _, tc := range []struct {
		method, host string
		origin       *string
		code         int
	}{
		{"GET", "localhost:8080", nil, 200},
		{"GET", "127.0.0.1:8080", nil, 404},
		{"GET", "foo.localhost:8080", nil, 404},
		{"POST", "localhost:8080", nil, 405},
		{"HEAD", "localhost:8080", nil, 405},
		{"GET", "localhost:8080", new(string), 403},
	} {
		h := map[string]string{}
		if tc.origin != nil {
			h["Origin"] = *tc.origin
		}
		w := request(s, tc.method, tc.host, path, "", h)
		if w.Code != tc.code {
			t.Errorf("%+v: %d", tc, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("CORS granted")
		}
	}

	r := httptest.NewRequest("GET", "http://peer.zelport.localhost"+path, nil)
	w := httptest.NewRecorder()
	s.ServeRemoteAdministration(w, r, feature.RemoteAdminContext{Authority: r.Host, ConsumerID: "c", ProviderID: "p"})
	if w.Code != 404 {
		t.Fatal("remote integration", w.Code)
	}
}

func TestIntegrationCommittedProjection(t *testing.T) {
	mod := &sanitizedModule{Base: modulekit.Base{Desc: feature.Descriptor{Maturity: feature.Stable, ID: "test", Name: "Test", TabID: "test"}}}
	m, e := app.New("", 80, mod)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	m.Start(context.Background())
	route, e := m.Save(context.Background(), 0, feature.Route{Module: "test", Domain: "app.localhost", Settings: json.RawMessage(`{"secret":"do-not-leak"}`)})
	if e != nil {
		t.Fatal(e)
	}
	mod.state = feature.State{Revision: 0, Routes: []feature.RouteView{{ID: route.ID, Domain: "old.localhost", Summary: feature.Summary{State: "ready", Detail: "do-not-leak", Target: "do-not-leak"}, Editor: json.RawMessage(`{"secret":"do-not-leak"}`)}, {ID: "removed"}}, Data: json.RawMessage(`{"secret":"do-not-leak"}`)}
	s, _ := New(m, 80, RuntimeInfo{Version: "test", ConfigPath: "do-not-leak"})

	w := request(s, "GET", "localhost", "/api/integration/v1/state", "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var v integrationState
	if e = json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	if len(v.Services) != 1 || v.Services[0].ID != route.ID || v.Services[0].State != "stale" || v.Services[0].URL != "http://app.localhost" || v.AdminURL != "http://localhost" {
		t.Fatalf("%+v", v)
	}
	if strings.Contains(w.Body.String(), "do-not-leak") || strings.Contains(w.Body.String(), "private-token") || v.Connections == nil || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Body)
	}
}

func TestIntegrationDataFlowUsesCommittedSummary(t *testing.T) {
	mod := &sanitizedModule{Base: modulekit.Base{Desc: feature.Descriptor{Maturity: feature.Stable, ID: "test", Name: "Test", TabID: "test"}}}
	m, err := app.New("", 8080, mod)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.Start(context.Background())
	route, err := m.Save(context.Background(), 0, feature.Route{Module: "test", Domain: "app.localhost", Settings: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	rev, _ := m.Snapshot()
	mod.state = feature.State{Revision: rev, Routes: []feature.RouteView{{ID: route.ID, Summary: feature.Summary{State: "ready", Target: "Sandbox port 3000 → Aux port 5173", Detail: "private-detail"}}}}
	s, _ := New(m, 8080)

	for _, current := range []bool{true, false} {
		if !current {
			mod.state.Revision = rev + 1
		}
		w := request(s, "GET", "localhost:8080", "/api/integration/v1/state", "", nil)
		var v struct {
			Services []struct {
				DataFlow string `json:"data_flow"`
				State    string `json:"state"`
			} `json:"services"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if len(v.Services) != 1 {
			t.Fatal(w.Body)
		}
		if current && v.Services[0].DataFlow != "Sandbox port 3000 → Aux port 5173" {
			t.Fatal(w.Body)
		}
		if !current && (v.Services[0].DataFlow != "" || v.Services[0].State != "stale") {
			t.Fatal(w.Body)
		}
		if strings.Contains(w.Body.String(), "private-detail") {
			t.Fatal("detail leaked")
		}
	}
}

func TestIntegrationNoStore(t *testing.T) {
	m, _ := app.New("", 8080)
	defer m.Close()
	s, _ := New(m, 8080)
	w := request(s, "GET", "localhost:8080", "/api/integration/v1/state", "", nil)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(w.Code, w.Header())
	}
}

type connectionModule struct{ sanitizedModule }

func (m *connectionModule) ConnectionSummaries() []feature.ConnectionSummary {
	return []feature.ConnectionSummary{{ID: "test", Label: "Test", State: "connected"}}
}
func TestIntegrationConnectionVisibility(t *testing.T) {
	mod := &connectionModule{sanitizedModule{Base: modulekit.Base{Desc: feature.Descriptor{ID: "test", Name: "Test", TabID: "test", Maturity: feature.Experimental}}}}
	m, e := app.New("", 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	m.Start(context.Background())
	s, _ := New(m, 8080)

	read := func() integrationState {
		w := request(s, "GET", "localhost:8080", "/api/integration/v1/state", "", nil)
		var v integrationState
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	if len(read().Connections) != 0 {
		t.Fatal("hidden module exposed")
	}
	rev, _ := m.Snapshot()
	if _, e = m.SetAvailability(context.Background(), rev, "test", true); e != nil {
		t.Fatal(e)
	}
	v := read()
	if len(v.Connections) != 1 || v.Connections[0].State != "connected" {
		t.Fatal(v)
	}
}

func TestIntegrationSettingsConnectionRemainsVisible(t *testing.T) {
	mod := &connectionModule{sanitizedModule{Base: modulekit.Base{Desc: feature.Descriptor{ID: "test", Name: "Test", UIPlacement: "settings", Maturity: feature.Stable}}}}
	m, e := app.New("", 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	m.Start(context.Background())
	s, _ := New(m, 8080)

	w := request(s, "GET", "localhost:8080", "/api/integration/v1/state", "", nil)
	var v integrationState
	json.Unmarshal(w.Body.Bytes(), &v)
	if len(v.Connections) != 1 {
		t.Fatal("settings module indicator missing", w.Body)
	}
}

func TestIntegrationPathBelongsToApplicationOnServiceHost(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/integration/v1/state" {
			t.Errorf("unexpected application path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("application-owned response"))
	}))
	defer backend.Close()
	_, portText, e := net.SplitHostPort(strings.TrimPrefix(backend.URL, "http://"))
	if e != nil {
		t.Fatal(e)
	}
	port, e := strconv.Atoi(portText)
	if e != nil {
		t.Fatal(e)
	}
	m, e := app.New("", 8080, local_ports.New())
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	settings, _ := json.Marshal(map[string]int{"aux_port": port})
	if _, e = m.Save(context.Background(), 0, feature.Route{Module: "local_ports", Domain: "app.localhost", Settings: settings}); e != nil {
		t.Fatal(e)
	}
	s, e := New(m, 8080)
	if e != nil {
		t.Fatal(e)
	}
	w := request(s, "GET", "app.localhost:8080", "/api/integration/v1/state", "", nil)
	if w.Code != http.StatusAccepted || w.Body.String() != "application-owned response" {
		t.Fatalf("application route intercepted: %d %s", w.Code, w.Body)
	}
}

func TestUnsupportedConfigurationProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	raw := `{"version":1,"modules":{"missing":{"secret":"opaque-secret"}},"routes":[{"id":"b60d7e28-d39e-4ad4-9a36-50f9158f35cb","module":"missing","domain":"saved.localhost","settings":{"secret":"opaque-secret"}}]}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := app.New(path, 8080)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	s, _ := New(m, 8080)

	for _, endpoint := range []string{"/api/state", "/api/integration/v1/state"} {
		w := request(s, "GET", "localhost:8080", endpoint, "", nil)
		if w.Code != 200 || strings.Contains(w.Body.String(), "opaque-secret") {
			t.Fatal(w.Code, w.Body)
		}
		if !strings.Contains(w.Body.String(), `"state":"unsupported"`) || !strings.Contains(w.Body.String(), `"domain":"saved.localhost"`) {
			t.Fatal(w.Body)
		}
		if endpoint == "/api/state" && !strings.Contains(w.Body.String(), `"unsupported_modules":["missing"]`) {
			t.Fatal(w.Body)
		}
	}
}
