package integration

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	relayserver "github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport-sdk-go/client"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport-sdk-go/transport"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	relaymodule "github.com/zelport/zelport/internal/modules/built_in_relay"
	"github.com/zelport/zelport/internal/web"
)

// A consumer seeing a provider does not mean the provider has authenticated the
// consumer's catalog yet. Wait for both directions before testing tunnel behavior.
func waitPeerCatalogs(t *testing.T, a, b *instance) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for {
		var left, right struct {
			DeviceID string        `json:"device_id"`
			Status   client.Status `json:"status"`
		}
		json.Unmarshal(a.mod.State().Data, &left)
		json.Unmarshal(b.mod.State().Data, &right)
		has := func(status client.Status, id string) bool {
			if !status.Connected {
				return false
			}
			for _, r := range status.Records {
				if r.DeviceID == id {
					return true
				}
			}
			return false
		}
		if has(left.Status, right.DeviceID) && has(right.Status, left.DeviceID) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("mutual authenticated catalogs unavailable")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Both deployment modes use the production listener lifecycle, not httptest serving.
func hostedRelay(t *testing.T, mode string) (url, access string, restart func()) {
	t.Helper()
	if mode == "standalone-runtime" {
		access, _ = protocol.Token()
		token, _ := protocol.DecodeKey(access)
		rt, e := relayserver.Prepare("127.0.0.1:0", relayserver.Options{AccessToken: token})
		if e != nil {
			t.Fatal(e)
		}
		rt.Start()
		address := rt.Status().Listeners[0]
		url = "ws://" + address
		t.Cleanup(func() { rt.Close() })
		return url, access, func() {
			rt.Close()
			var e error
			rt, e = relayserver.Prepare(address, relayserver.Options{AccessToken: token})
			if e != nil {
				t.Fatal(e)
			}
			rt.Start()
		}
	}
	mod := relaymodule.New()
	m, e := app.New(filepath.Join(t.TempDir(), "relay.json"), 18222, mod)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close() })
	w, e := web.New(m, 18222)
	if e != nil {
		t.Fatal(e)
	}
	inst := &instance{m: m, web: w, port: 18222}
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	configure := func(enabled bool) {
		response := inst.api(t, "PUT", "/api/modules/built_in_relay/settings", map[string]any{"enabled": enabled, "port": port, "bind_address": "127.0.0.1", "require_access_token": mode != "embedded-open"})
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	configure(true)
	if mode == "embedded-open" {
		return "ws://127.0.0.1:" + strconv.Itoa(port), "", func() { configure(false); configure(true) }
	}
	response := inst.api(t, "POST", "/api/modules/built_in_relay/credentials", map[string]any{"action": "reveal"})
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var v struct {
		Access string `json:"access_token"`
	}
	json.Unmarshal(response.Body.Bytes(), &v)
	return "ws://127.0.0.1:" + strconv.Itoa(port), v.Access, func() { configure(false); configure(true) }
}

func TestHostedRelayRejectsWrongTokenAndReconnects(t *testing.T) {
	for _, mode := range []string{"standalone-runtime", "embedded", "embedded-open"} {
		t.Run(mode, func(t *testing.T) {
			url, access, restart := hostedRelay(t, mode)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if c, e := transport.Dial(ctx, url, make([]byte, 32), "control"); e == nil {
				c.Close()
				t.Fatal("wrong token accepted")
			}
			group, _ := protocol.Token()
			provider, _ := newInstance(t, 18090)
			consumer, _ := newInstance(t, 18091)
			source := save(t, provider, feature.Route{Module: "test_source", Domain: "reconnect.localhost", Settings: json.RawMessage(`{}`)})
			provider.settings(t, url, access, group, true)
			consumer.settings(t, url, access, group, false)
			first := waitOffer(t, consumer, source.ID)
			restart()
			// Wait for a newly published record, not the last-known cached catalog.
			deadline := time.Now().Add(10 * time.Second)
			for {
				next := waitOffer(t, consumer, source.ID)
				if next.Sequence > first.Sequence {
					return
				}
				if time.Now().After(deadline) {
					t.Fatal("catalog did not recover after restart")
				}
				time.Sleep(20 * time.Millisecond)
			}
		})
	}
}
