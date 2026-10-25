package ports_over_ssh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/sshpipe"
	"net"
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

func sshTestManager(t *testing.T) (*Module, *app.Manager, string) {
	t.Helper()
	module := New().(*Module)
	path := filepath.Join(t.TempDir(), "zelport.json")
	manager, err := app.New(path, 8080, module)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	return module, manager, path
}
func sshRequest(module *Module, method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	module.Admin().ServeHTTP(response, httptest.NewRequest(method, "http://localhost:8080"+path, strings.NewReader(body)))
	return response
}
func TestSSHConfigSelectionPersistsAndControlsHostDiscovery(t *testing.T) {
	module, manager, configPath := sshTestManager(t)
	selected := filepath.Join(t.TempDir(), "other config")
	if err := os.WriteFile(selected, []byte("Host chosen-server\n  IdentityFile ~/.ssh/private-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	revision, _ := manager.Snapshot()
	body, _ := json.Marshal(map[string]any{"revision": revision, "ssh_config_path": selected})
	if response := sshRequest(module, http.MethodPut, "/settings", string(body)); response.Code != http.StatusOK {
		t.Fatalf("save: %d %s", response.Code, response.Body.String())
	}
	response := sshRequest(module, http.MethodGet, "/hosts", "")
	var hosts struct {
		Hosts      []string `json:"hosts"`
		ConfigPath string   `json:"config_path"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &hosts); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hosts.Hosts, []string{"chosen-server"}) || hosts.ConfigPath != selected {
		t.Fatalf("discovery: %+v", hosts)
	}
	if strings.Contains(response.Body.String(), "private-key") {
		t.Fatal("file contents escaped into host response")
	}
	manager.Close()
	restarted := New().(*Module)
	loaded, err := app.New(configPath, 8080, restarted)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	if err = loaded.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Path string `json:"ssh_config_path"`
	}
	if err = json.Unmarshal(restarted.State().Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Path != selected {
		t.Fatalf("saved path not restored: %q", data.Path)
	}
}

func TestSSHConfigSettingsRejectInvalidPathsAndStaleWrites(t *testing.T) {
	module, manager, _ := sshTestManager(t)
	revision, _ := manager.Snapshot()
	directory := t.TempDir()
	for _, path := range []string{"relative/config", "~/bad\nconfig", directory} {
		body, _ := json.Marshal(map[string]any{"revision": revision, "ssh_config_path": path})
		response := sshRequest(module, http.MethodPut, "/settings", string(body))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid path %q: %d %s", path, response.Code, response.Body.String())
		}
	}
	missing := filepath.Join(t.TempDir(), "missing config")
	body, _ := json.Marshal(map[string]any{"revision": revision, "ssh_config_path": missing})
	if response := sshRequest(module, http.MethodPut, "/settings", string(body)); response.Code != http.StatusOK {
		t.Fatalf("missing path: %d %s", response.Code, response.Body.String())
	}
	if response := sshRequest(module, http.MethodPut, "/settings", fmt.Sprintf(`{"revision":%d,"ssh_config_path":""}`, revision)); response.Code != http.StatusConflict {
		t.Fatalf("stale settings: %d %s", response.Code, response.Body.String())
	}
	if err := module.ValidateSettings(json.RawMessage(`{"unexpected":true}`)); err == nil {
		t.Fatal("unknown setting accepted")
	}
	var state struct {
		Exists bool   `json:"config_exists"`
		Path   string `json:"effective_config_path"`
	}
	if err := json.Unmarshal(module.State().Data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Exists || state.Path != missing {
		t.Fatalf("missing config state: %+v", state)
	}
}

func TestSSHSettingsKeepRouteIdentities(t *testing.T) {
	module, manager, _ := sshTestManager(t)
	revision, _ := manager.Snapshot()
	route, err := manager.Save(context.Background(), revision, feature.Route{Module: "ports_over_ssh", Domain: "fixture.localhost", Settings: json.RawMessage(`{"ssh_host":"office","ssh_port":3000}`)})
	if err != nil {
		t.Fatal(err)
	}
	revision, _ = manager.Snapshot()
	path := filepath.Join(t.TempDir(), "config")
	body, _ := json.Marshal(map[string]any{"revision": revision, "ssh_config_path": path})
	if response := sshRequest(module, http.MethodPut, "/settings", string(body)); response.Code != http.StatusOK {
		t.Fatalf("save: %d %s", response.Code, response.Body.String())
	}
	_, routes := manager.Snapshot()
	if len(routes) != 1 || routes[0].ID != route.ID {
		t.Fatalf("route identity changed: %+v", routes)
	}
}

func TestSSHChangingConfigRetiresExistingPool(t *testing.T) {
	module, manager, _ := sshTestManager(t)
	revision, _ := manager.Snapshot()
	route, err := manager.Save(context.Background(), revision, feature.Route{Module: "ports_over_ssh", Domain: "pool.localhost", Settings: json.RawMessage(`{"ssh_host":"office","ssh_port":3000}`)})
	if err != nil {
		t.Fatal(err)
	}
	module.configMu.Lock()
	path, err := module.selectedConfig()
	if err != nil {
		t.Fatal(err)
	}
	pool := sshpipe.New("office", 3000)
	module.mu.Lock()
	module.pools[route.ID] = poolEntry{settings: poolKey(route, path), pool: pool}
	module.mu.Unlock()
	module.configMu.Unlock()
	revision, _ = manager.Snapshot()
	body, _ := json.Marshal(map[string]any{"revision": revision, "ssh_config_path": filepath.Join(t.TempDir(), "other")})
	if response := sshRequest(module, http.MethodPut, "/settings", string(body)); response.Code != 200 {
		t.Fatalf("save: %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.DialContext(context.Background(), "tcp", ""); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("old SSH pool remains usable: %v", err)
	}
}
func TestSSHCanReplacePathWhoseFileBecameADirectory(t *testing.T) {
	module, manager, _ := sshTestManager(t)
	path := filepath.Join(t.TempDir(), "config")
	revision, _ := manager.Snapshot()
	body, _ := json.Marshal(map[string]any{"revision": revision, "ssh_config_path": path})
	if response := sshRequest(module, http.MethodPut, "/settings", string(body)); response.Code != 200 {
		t.Fatalf("save: %d", response.Code)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	revision, _ = manager.Snapshot()
	response := sshRequest(module, http.MethodPut, "/settings", fmt.Sprintf(`{"revision":%d,"ssh_config_path":""}`, revision))
	if response.Code != 200 {
		t.Fatalf("cannot recover from invalid saved path: %d %s", response.Code, response.Body.String())
	}
}
