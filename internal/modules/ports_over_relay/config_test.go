package ports_over_relay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
)

func testModule(t *testing.T) (*Module, *app.Manager) {
	t.Helper()
	mod := New().(*Module)
	m, e := app.New(filepath.Join(t.TempDir(), "config.json"), 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close() })
	return mod, m
}
func TestCloudSecretState(t *testing.T) {
	mod, _ := testModule(t)
	s, e := newIdentity()
	if e != nil {
		t.Fatal(e)
	}
	s.AccessToken, _ = protocol.Token()
	s.RelayURL = "ws://localhost:8090"
	if e = mod.store.update(context.Background(), feature.CommitOptions{}, func(v *Settings) error { *v = s; return nil }); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(mod.State())
	for _, secret := range []string{s.AccessToken, s.GroupToken, s.PrivateKey} {
		if strings.Contains(string(b), secret) {
			t.Fatal("secret in state")
		}
	}
	if mod.Descriptor().Exportable {
		t.Fatal("cloud imports are exportable")
	}
}
func TestCloudIdentityRecovery(t *testing.T) {
	mod, _ := testModule(t)
	s, e := newIdentity()
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Settings){func(v *Settings) { v.DeviceID = "" }, func(v *Settings) { v.PrivateKey = "invalid" }, func(v *Settings) { v.GroupToken = "short" }} {
		v := s
		change(&v)
		b, _ := json.Marshal(v)
		if mod.ValidateSettings(b) == nil {
			t.Fatal("corrupt identity accepted")
		}
	}
	if e = mod.ValidateSettings(json.RawMessage(`{}`)); e != nil {
		t.Fatal(e)
	}
}
func TestCloudSettingsWriteFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	mod := New().(*Module)
	m, e := app.New(path, 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	if e = mod.store.update(context.Background(), feature.CommitOptions{}, func(s *Settings) error { v, e := newIdentity(); *s = v; return e }); e == nil {
		t.Fatal("write accepted directory")
	}
	s, _ := mod.store.snapshot()
	if s.DeviceID != "" {
		t.Fatal("failed save changed identity")
	}
}

func TestAdmissionAndGroupTokensMustDiffer(t *testing.T) {
	mod, _ := testModule(t)
	s, _ := newIdentity()
	s.AccessToken = s.GroupToken
	b, _ := json.Marshal(s)
	if mod.ValidateSettings(b) == nil {
		t.Fatal("relay admission token reused as E2E secret")
	}
}

func TestModuleAvailabilityRequiresConnectionDisabled(t *testing.T) {
	mod := New().(*Module)
	if mod.UnavailableReason(json.RawMessage(`{"enabled":true}`)) == "" {
		t.Fatal("desired connection state ignored")
	}
	if mod.UnavailableReason(json.RawMessage(`{"enabled":false}`)) != "" {
		t.Fatal("disabled connection blocks availability")
	}
}

func TestStoredRelayURLSurvivesWireUpgrade(t *testing.T) {
	mod, _ := testModule(t)
	s, _ := newIdentity()
	s.RelayURL = "wss://relay.example/v2/ws"
	b, _ := json.Marshal(s)
	if e := mod.ValidateSettings(b); e != nil {
		t.Fatal("existing endpoint setting rejected", e)
	}
}

func TestDesktopConnectionSummary(t *testing.T) {
	mod, _ := testModule(t)
	v := mod.ConnectionSummaries()
	if len(v) != 1 || v[0].ID != "ports_over_relay" || v[0].State != "disconnected" || v[0].Label != "Ports over Relay" {
		t.Fatalf("%+v", v)
	}
}
