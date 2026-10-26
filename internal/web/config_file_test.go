package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modules/ports_over_ssh"
	"github.com/zelport/zelport/internal/modules/remote_http"
)

func TestRemoteFileAccessPolicyForConfigurationFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("LOCALAPPDATA", home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	const original = "Host unique-editor-fixture\n"
	path := filepath.Join(home, ".ssh", "config")
	os.WriteFile(path, []byte(original), 0600)
	manager, err := app.New("", 8080, remote_http.New(), ports_over_ssh.New())
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	server, err := New(manager, 8080)
	if err != nil {
		t.Fatal(err)
	}
	remote := feature.RemoteAdminContext{ConsumerID: "consumer", ProviderID: "provider", Authority: "peer.zelport.localhost:8080"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { server.ServeRemoteAdministration(w, r, remote) })
	var state struct {
		CSRF     string `json:"csrf"`
		Revision uint64 `json:"revision"`
	}
	w := request(handler, "GET", remote.Authority, "/api/state", "", nil)
	json.Unmarshal(w.Body.Bytes(), &state)
	if state.CSRF == "" || strings.Contains(w.Body.String(), "unique-editor-fixture") {
		t.Fatal("polling leaked content or missing remote token")
	}
	endpoint := "/api/modules/ports_over_ssh/config-file"
	headers := map[string]string{"Origin": "http://" + remote.Authority, "X-CSRF-Token": state.CSRF}
	local := map[string]string{"Origin": server.origin(), "X-CSRF-Token": server.token}
	// Remote file access is off by default: no reads, writes or path changes.
	for _, path := range []string{endpoint, "/api/modules/remote_http/ca-file"} {
		if w = request(handler, "GET", remote.Authority, path, "", nil); w.Code != 403 || strings.Contains(w.Body.String(), "unique-editor-fixture") {
			t.Fatalf("default remote read %s: %d %s", path, w.Code, w.Body)
		}
	}
	if w = request(handler, "PUT", remote.Authority, "/api/modules/ports_over_ssh/settings", fmt.Sprintf(`{"revision":%d,"ssh_config_path":"~/.zshrc"}`, state.Revision), headers); w.Code != 403 {
		t.Fatalf("default remote SSH path change: %d %s", w.Code, w.Body)
	}
	if w = request(handler, "PUT", remote.Authority, "/api/modules/remote_http/settings", fmt.Sprintf(`{"revision":%d,"ca_cert_path":"~/.zshrc"}`, state.Revision), headers); w.Code != 403 {
		t.Fatalf("default remote CA path change: %d %s", w.Code, w.Body)
	}
	if w = request(handler, "PUT", remote.Authority, "/api/modules/remote_http/settings", fmt.Sprintf(`{"revision":%d,"proxy_url":"http://proxy.example.test:8080"}`, state.Revision), headers); w.Code != 200 {
		t.Fatalf("remote non-file setting: %d %s", w.Code, w.Body)
	}
	// Only this computer can allow remote file access.
	revision, _ := manager.Snapshot()
	toggle := fmt.Sprintf(`{"revision":%d,"enabled":true}`, revision)
	if w = request(handler, "PUT", remote.Authority, "/api/remote-file-access", toggle, headers); w.Code != 403 || manager.RemoteFileAccess() {
		t.Fatalf("remote widened its own access: %d", w.Code)
	}
	if w = request(server, "PUT", "localhost:8080", "/api/remote-file-access", toggle, local); w.Code != 200 || !manager.RemoteFileAccess() {
		t.Fatalf("local enable: %d %s", w.Code, w.Body)
	}
	w = request(handler, "GET", remote.Authority, endpoint, "", nil)
	var doc struct {
		Content, Version string
		Revision         uint64
	}
	json.Unmarshal(w.Body.Bytes(), &doc)
	if w.Code != 200 || doc.Content != original || doc.Version == "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("remote read: %d %s", w.Code, w.Body)
	}
	raw, _ := json.Marshal(map[string]any{"revision": doc.Revision, "version": doc.Version, "content": "Host remotely-edited\n"})
	for _, headers := range []map[string]string{nil, {"Origin": "http://evil.test", "X-CSRF-Token": state.CSRF}, {"Origin": "http://" + remote.Authority, "X-CSRF-Token": server.token}} {
		if w = request(handler, "PUT", remote.Authority, endpoint, string(raw), headers); w.Code != 403 {
			t.Fatalf("unprotected save: %d", w.Code)
		}
	}
	if w = request(handler, "PUT", remote.Authority, endpoint, string(raw), headers); w.Code != 200 {
		t.Fatalf("remote save: %d %s", w.Code, w.Body)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "Host remotely-edited\n" {
		t.Fatalf("remote bytes %q", data)
	}
	if w = request(server, "GET", remote.Authority, endpoint, "", nil); w.Code == 200 {
		t.Fatal("unauthenticated remote authority accepted")
	}
	if w = request(handler, "GET", remote.Authority, "/api/modules/remote_http/ca-file", "", nil); w.Code != 200 {
		t.Fatalf("remote CA read: %d %s", w.Code, w.Body)
	}
	// The existing local protected shell exposes the same editor endpoint.
	if w = request(server, "GET", "localhost:8080", endpoint, "", nil); w.Code != 200 {
		t.Fatalf("local read: %d", w.Code)
	}
}
