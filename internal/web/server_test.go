package web

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"github.com/zelport/zelport/internal/modules/local_ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func request(s http.Handler, method, host, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+host+path, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestCoreAndAbsentModuleAssets(t *testing.T) {
	m, e := app.New("", 8080)
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	s, e := New(m, 8080)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"/api/modules/local_ports/refresh", "/assets/modules/local_ports/module.js"} {
		w := request(s, "GET", "localhost:8080", p, "", nil)
		if w.Code != 404 {
			t.Fatal(p, w.Code)
		}
	}
	for _, host := range []string{"127.0.0.1:8080", "evil.test:8080", "localhost:8081"} {
		if w := request(s, "GET", host, "/api/state", "", nil); w.Code != 404 {
			t.Fatal(host, w.Code)
		}
	}
	if w := request(s, "GET", "localhost:8080", "/api/state", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"modules":[]`) || strings.Contains(w.Body.String(), "unsupported_modules") || strings.Contains(w.Body.String(), `"state":"unsupported"`) {
		t.Fatal(w.Code, w.Body)
	}
}
func TestModuleMutationSecurityAndConflict(t *testing.T) {
	m, _ := app.New("", 8080, local_ports.New())
	m.Start(context.Background())
	defer m.Close()
	s, _ := New(m, 8080)
	body := `{"revision":0,"route":{"module":"local_ports","domain":"foo.localhost","settings":{"aux_port":3000}}}`
	for _, path := range []string{"/api/routes", "/api/modules/local_ports/anything"} {
		for _, h := range []map[string]string{nil, {"Origin": "http://evil.test", "X-CSRF-Token": s.token}, {"Origin": s.origin()}} {
			if w := request(s, "POST", "localhost:8080", path, body, h); w.Code != 403 {
				t.Fatal(w.Code)
			}
		}
	}
	headers := map[string]string{"Origin": s.origin(), "X-CSRF-Token": s.token}
	if w := request(s, "POST", "localhost:8080", "/api/routes", body, headers); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(s, "POST", "localhost:8080", "/api/routes", body, headers); w.Code != 409 {
		t.Fatal("stale revision accepted", w.Code, w.Body)
	}
}

func TestCoreAssetsContainNoModuleActions(t *testing.T) {
	for _, name := range []string{"assets/index.html", "assets/app.js", "assets/ui.js"} {
		data, err := assets.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"Start sandbox", "Open terminal", "/remove-mapping", "sandbox_id", "ssh_host", "target_url"} {
			if strings.Contains(string(data), action) {
				t.Errorf("core asset %s contains module action %q", name, action)
			}
		}
	}
}

type sanitizedModule struct {
	modulekit.Base
	state feature.State
}

func (m *sanitizedModule) ValidateRoute(feature.Route) error      { return nil }
func (m *sanitizedModule) ValidateSettings(json.RawMessage) error { return nil }
func (m *sanitizedModule) State() feature.State                   { return m.state }
func (m *sanitizedModule) ServeRoute(w http.ResponseWriter, r *http.Request, _ feature.Route) {
	http.NotFound(w, r)
}
func TestStateFilteringAndPrivateSettings(t *testing.T) {
	mod := &sanitizedModule{Base: modulekit.Base{Desc: feature.Descriptor{Maturity: feature.Stable, ID: "test_module", Name: "Test", TabID: "test"}}}
	m, err := app.New("", 8080, mod)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	route, err := m.Save(context.Background(), 0, feature.Route{Module: "test_module", Domain: "current.localhost", Settings: json.RawMessage(`{"private":"private-route-secret"}`)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = mod.Host.Commit(context.Background(), 1, feature.CommitOptions{}, func(d *feature.Draft) error {
		d.Settings = json.RawMessage(`{"token":"private-module-secret"}`)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mod.state = feature.State{Revision: 1, Routes: []feature.RouteView{{ID: route.ID, Module: "test_module", Domain: "outdated.localhost", Summary: feature.Summary{State: "ready"}, Editor: json.RawMessage(`{}`)}, {ID: "e2c53e9d-7c98-4004-ad09-f8b3eed14b7c", Module: "test_module", Domain: "removed.localhost"}}}
	server, err := New(m, 8080)
	if err != nil {
		t.Fatal(err)
	}
	w := request(server, "GET", "localhost:8080", "/api/state", "", nil)
	body := w.Body.String()
	if w.Code != 200 {
		t.Fatal(w.Code, body)
	}
	for _, hidden := range []string{"private-route-secret", "private-module-secret", "removed.localhost", "outdated.localhost"} {
		if strings.Contains(body, hidden) {
			t.Errorf("state leaked %s", hidden)
		}
	}
	if !strings.Contains(body, `"domain":"current.localhost"`) || !strings.Contains(body, `"state":"stale"`) {
		t.Fatal(body)
	}
}

func TestAvailabilityAPIProtectionAndValidation(t *testing.T) {
	m, e := app.New("", 8080, local_ports.New())
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	s, _ := New(m, 8080)
	path := "/api/module-availability"
	body := `{"revision":0,"module":"local_ports","enabled":false}`
	for _, headers := range []map[string]string{nil, {"Origin": "http://evil.test", "X-CSRF-Token": s.token}, {"Origin": s.origin()}} {
		if w := request(s, "PUT", "localhost:8080", path, body, headers); w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
	headers := map[string]string{"Origin": s.origin(), "X-CSRF-Token": s.token}
	if w := request(s, "PUT", "localhost:8080", path, `{"revision":0,"module":"local_ports"}`, headers); w.Code != 400 {
		t.Fatal("missing enabled accepted")
	}
	if w := request(s, "PUT", "localhost:8080", path, `{"revision":0,"module":"not_compiled","enabled":true}`, headers); w.Code != 404 {
		t.Fatal("unknown module accepted")
	}
	if w := request(s, "PUT", "localhost:8080", path, body, headers); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(s, "PUT", "localhost:8080", path, body, headers); w.Code != 409 {
		t.Fatal("stale write accepted")
	}
	if m.Availability()["local_ports"].Enabled {
		t.Fatal("module not disabled")
	}
	// Hidden interfaces do not disable their administration APIs or services.
	if w := request(s, "POST", "localhost:8080", "/api/routes", `{"revision":1,"route":{"module":"local_ports","domain":"foo.localhost","settings":{"aux_port":3000}}}`, headers); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !m.Availability()["local_ports"].Enabled {
		t.Fatal("configured module hidden")
	}
	if w := request(s, "PUT", "localhost:8080", path, `{"revision":2,"module":"local_ports","enabled":false}`, headers); w.Code != 400 {
		t.Fatal("configured module disabled")
	}
}
