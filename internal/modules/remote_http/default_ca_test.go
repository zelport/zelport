package remote_http

import (
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/zelport/zelport/internal/userpaths"
)

func isolateCAHome(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("USERPROFILE", base)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("LOCALAPPDATA", filepath.Join(base, "local"))
	config, err := userpaths.Config()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(config), "additional-ca.pem")
}

func TestDefaultCAPathAndOptionalAbsence(t *testing.T) {
	path := isolateCAHome(t)
	m, manager, _ := networkManager(t)
	var state struct {
		Effective string `json:"effective_ca_cert_path"`
		Default   string `json:"default_ca_cert_path"`
		Error     string `json:"configuration_error"`
	}
	if err := json.Unmarshal(m.State().Data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Effective != path || state.Default != path || state.Error != "" {
		t.Fatalf("default state: %+v", state)
	}
	saveNetwork(t, m, manager, map[string]any{})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("default file was created: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid PEM"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(m.State().Data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Error == "" {
		t.Fatal("invalid existing default CA accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(m.State().Data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Error != "" {
		t.Fatalf("deleted default CA did not restore system trust: %s", state.Error)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), path); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(m.State().Data, &state); err != nil {
			t.Fatal(err)
		}
		if state.Error == "" {
			t.Fatal("dangling default CA symlink silently ignored")
		}
	}
}

func TestDefaultCABundleIsUsedAndDeletionRestoresSystemTrust(t *testing.T) {
	path := isolateCAHome(t)
	_, manager, _ := networkManager(t)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "trusted") }))
	defer origin.Close()
	networkRoute(t, manager, origin.URL, false)
	if response := networkRequest(manager); response.Code != 502 {
		t.Fatalf("untrusted origin: %d", response.Code)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: origin.Certificate().Raw})
	if err := os.WriteFile(path, certificate, 0600); err != nil {
		t.Fatal(err)
	}
	if response := networkRequest(manager); response.Code != 200 {
		t.Fatalf("default bundle not used: %d %s", response.Code, response.Body)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if response := networkRequest(manager); response.Code != 502 {
		t.Fatalf("removed default bundle retained trust: %d", response.Code)
	}
}
