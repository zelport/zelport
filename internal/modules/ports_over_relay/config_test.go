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
func TestRelaySecretState(t *testing.T) {
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
		t.Fatal("relay imports are exportable")
	}
}
func TestRelayIdentityRecovery(t *testing.T) {
	mod, _ := testModule(t)
	s, e := newIdentity()
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Settings){func(v *Settings) { v.DeviceID = "" }, func(v *Settings) { v.PrivateKey = "invalid" }, func(v *Settings) { v.GroupToken = "short" }, func(v *Settings) { v.AdminServiceID = "" }} {
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

func TestRelayProtocolVersionIsRequiredForEstablishedState(t *testing.T) {
	mod, _ := testModule(t)
	s, err := newIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if s.ProtocolVersion != 1 {
		t.Fatalf("protocol version = %d, want 1", s.ProtocolVersion)
	}

	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "protocol_version")
	missing, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	invalid := []json.RawMessage{missing}
	for _, version := range []int{0, 2, 3, 6} {
		candidate := s
		candidate.ProtocolVersion = version
		body, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		invalid = append(invalid, body)
	}
	for _, body := range invalid {
		if err := mod.ValidateSettings(body); err == nil || err.Error() != "Relay protocol_version must be 1. Create a fresh Ports over Relay configuration." {
			t.Fatalf("unexpected validation result for %s: %v", body, err)
		}
	}
}

func TestRelayWatermarksRequireDigestVersionOne(t *testing.T) {
	mod, _ := testModule(t)
	s, _ := newIdentity()
	key := strings.Repeat("a", 64) + "/" + protocol.ID()
	for _, version := range []int{0, 2, 3, 6} {
		s.Watermarks = map[string]Watermark{key: {DigestVersion: version, Sequence: 1, Digest: strings.Repeat("b", 64)}}
		body, _ := json.Marshal(s)
		if err := mod.ValidateSettings(body); err == nil {
			t.Fatalf("accepted digest version %d", version)
		}
	}
}
func TestRelaySettingsWriteFailure(t *testing.T) {
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

func TestRelayURLAcceptsOnlyCurrentEndpoint(t *testing.T) {
	mod, _ := testModule(t)
	s, _ := newIdentity()
	for _, relayURL := range []string{"wss://relay.example", "wss://relay.example/", "wss://relay.example/v1/ws"} {
		s.RelayURL = relayURL
		body, _ := json.Marshal(s)
		if err := mod.ValidateSettings(body); err != nil {
			t.Fatalf("current endpoint %q rejected: %v", relayURL, err)
		}
	}
	for _, relayURL := range []string{"wss://relay.example/v0/ws", "wss://relay.example/v2/ws", "wss://relay.example/v99/ws"} {
		s.RelayURL = relayURL
		body, _ := json.Marshal(s)
		if err := mod.ValidateSettings(body); err == nil {
			t.Fatalf("retired endpoint %q accepted", relayURL)
		}
	}
}

func TestDesktopConnectionSummary(t *testing.T) {
	mod, _ := testModule(t)
	v := mod.ConnectionSummaries()
	if len(v) != 1 || v[0].ID != "ports_over_relay" || v[0].State != "disconnected" || v[0].Label != "Ports over Relay" {
		t.Fatalf("%+v", v)
	}
}
