package web

import (
	"context"
	"encoding/json"
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

func TestConfigurationFilesThroughAuthenticatedRemoteAdministration(t *testing.T) {
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
		CSRF string `json:"csrf"`
	}
	w := request(handler, "GET", remote.Authority, "/api/state", "", nil)
	json.Unmarshal(w.Body.Bytes(), &state)
	if state.CSRF == "" || strings.Contains(w.Body.String(), "unique-editor-fixture") {
		t.Fatal("polling leaked content or missing remote token")
	}
	endpoint := "/api/modules/ports_over_ssh/config-file"
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
	headers := map[string]string{"Origin": "http://" + remote.Authority, "X-CSRF-Token": state.CSRF}
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
