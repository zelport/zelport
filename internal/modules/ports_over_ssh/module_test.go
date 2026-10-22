package ports_over_ssh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
)

func TestConfiguredHostsEndpointReturnsAliasesOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte("Host office-server\n  IdentityFile ~/.ssh/private-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New().Admin().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/hosts", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /hosts: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Hosts []string `json:"hosts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body.Hosts, []string{"office-server"}) {
		t.Fatalf("hosts = %q", body.Hosts)
	}
	if strings.Contains(response.Body.String(), "private-key") {
		t.Fatal("response exposed SSH key details")
	}
}

func TestPrimaryDevelopmentServerFlowIsAvailableByDefault(t *testing.T) {
	if descriptor := New().Descriptor(); !descriptor.DefaultEnabled {
		t.Fatalf("Ports over SSH must be available in the first use chooser: %+v", descriptor)
	}
}

func TestUnreachableHostDoesNotRejectRoute(t *testing.T) {
	module := New().(*Module)
	manager, err := app.New(filepath.Join(t.TempDir(), "config.json"), 8080, module)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })

	route, err := manager.Save(context.Background(), 0, feature.Route{
		Module:   "ports_over_ssh",
		Domain:   "agent.localhost",
		Settings: json.RawMessage(`{"ssh_host":"offline-server","ssh_port":3000}`),
	})
	if err != nil {
		t.Fatalf("save offline route: %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	response := httptest.NewRecorder()
	module.ServeRoute(response, httptest.NewRequest(http.MethodGet, "http://agent.localhost/", nil), route)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("offline response = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	state := module.State()
	if len(state.Routes) != 1 || state.Routes[0].Summary.State != "unavailable" {
		t.Fatalf("offline state = %#v", state.Routes)
	}
}
