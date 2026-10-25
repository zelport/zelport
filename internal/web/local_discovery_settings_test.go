package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modules/local_ports"
)

func TestLocalDiscoverySettingsProtectedLocallyAndRemotely(t *testing.T) {
	manager, err := app.New("", 8080, local_ports.New())
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	server, _ := New(manager, 8080)
	path := "/api/modules/local_ports/settings"
	body := `{"revision":0,"discovery_enabled":false,"ignored_ports":["22"],"ignored_processes":["sshd"]}`
	for _, headers := range []map[string]string{nil, {"Origin": "http://evil.test", "X-CSRF-Token": server.token}} {
		if w := request(server, "PUT", "localhost:8080", path, body, headers); w.Code != 403 {
			t.Fatalf("unprotected: %d", w.Code)
		}
	}
	if w := request(server, "PUT", "localhost:8080", path, body, map[string]string{"Origin": server.origin(), "X-CSRF-Token": server.token}); w.Code != 200 {
		t.Fatalf("local save: %d %s", w.Code, w.Body)
	}
	remote := feature.RemoteAdminContext{ConsumerID: "consumer", ProviderID: "provider", Authority: "peer.zelport.localhost:8080"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { server.ServeRemoteAdministration(w, r, remote) })
	state := request(handler, "GET", remote.Authority, "/api/state", "", nil)
	var credentials struct {
		CSRF     string `json:"csrf"`
		Revision uint64 `json:"revision"`
	}
	json.Unmarshal(state.Body.Bytes(), &credentials)
	raw, _ := json.Marshal(map[string]any{"revision": credentials.Revision, "discovery_enabled": true, "ignored_ports": []string{}, "ignored_processes": []string{}})
	if w := request(handler, "PUT", remote.Authority, path, string(raw), map[string]string{"Origin": "http://" + remote.Authority, "X-CSRF-Token": credentials.CSRF}); w.Code != 200 {
		t.Fatalf("remote save: %d %s", w.Code, w.Body)
	}
}
