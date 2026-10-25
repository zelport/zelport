package web

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/modules/remote_http"
)

func TestRemoteHTTPNetworkSettingsProtectMutationsAndHidePassword(t *testing.T) {
	manager, err := app.New("", 8080, remote_http.New())
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
	const endpoint = "/api/modules/remote_http/settings"
	const password = "synthetic-proxy-secret"
	body := `{"revision":0,"proxy_url":"http://proxy.example.test:3128","proxy_username":"test-account","proxy_password":"` + password + `"}`
	for _, headers := range []map[string]string{nil, {"Origin": server.origin()}, {"Origin": "http://evil.example.test", "X-CSRF-Token": server.token}} {
		if response := request(server, http.MethodPut, "localhost:8080", endpoint, body, headers); response.Code != http.StatusForbidden {
			t.Fatalf("unprotected mutation returned %d", response.Code)
		}
	}
	if response := request(server, http.MethodPut, "localhost:8080", endpoint, body, map[string]string{"Origin": server.origin(), "X-CSRF-Token": server.token}); response.Code != http.StatusOK {
		t.Fatalf("protected save: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{endpoint, "/api/state"} {
		response := request(server, http.MethodGet, "localhost:8080", path, "", nil)
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), password) || strings.Contains(response.Body.String(), `"proxy_password"`) {
			t.Fatalf("settings response exposed a proxy password at %s", path)
		}
		if !strings.Contains(response.Body.String(), `"has_proxy_password":true`) {
			t.Fatalf("missing stored-password flag at %s", path)
		}
	}
}
