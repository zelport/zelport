package docker_sandboxes

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/sbx"
	"testing"
)

type reviewPublicationBackend struct {
	fakeBackend
	afterPublish bool
}

func (b *reviewPublicationBackend) Ports(ctx context.Context, id string) ([]sbx.Port, error) {
	if b.afterPublish {
		return nil, errors.New("temporary port discovery failure")
	}
	return b.fakeBackend.Ports(ctx, id)
}
func (b *reviewPublicationBackend) Publish(context.Context, string, int) error {
	b.afterPublish = true
	return nil
}
func TestPostPublicationDiscoveryPreservesUsableMappings(t *testing.T) {
	b := &reviewPublicationBackend{fakeBackend: fakeBackend{running: true, ports: []sbx.Port{{HostIP: "127.0.0.1", HostPort: 45678, SandboxPort: 3000, Protocol: "tcp"}}}}
	mod := New().(*Module)
	mod.Backend = b
	m, e := app.New("", 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	mod.Refresh(context.Background())
	mod.op.Lock()
	defer mod.op.Unlock()
	var oldRoute feature.Route
	for i, port := range []int{3000, 4000} {
		s, _ := json.Marshal(Settings{SandboxID: "stable-id", SandboxPort: port})
		r, e := m.Save(context.Background(), uint64(i), feature.Route{Module: "docker_sandboxes", Domain: []string{"existing.localhost", "new.localhost"}[i], Settings: s})
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			oldRoute = r
		}
	}
	mod.refresh(context.Background())
	summary, mapping := mod.view(oldRoute)
	if mapping == nil {
		t.Fatalf("discarded last-known mapping after publication read failed: %+v", summary)
	}
	if summary.State != "stale" {
		t.Fatalf("expected stale retained mapping, got %+v", summary)
	}
}

func TestDiscoveryFailureSummaryKeepsDiagnosticsOutOfServiceRows(t *testing.T) {
	settings, _ := json.Marshal(Settings{SandboxID: "stable-id", SandboxPort: 3000})
	route := feature.Route{Settings: settings}
	port := sbx.Port{HostIP: "127.0.0.1", HostPort: 45678, SandboxPort: 3000, Protocol: "tcp"}
	for _, cached := range []bool{false, true} {
		mod := New().(*Module)
		mod.inventory.Stale = true
		mod.inventory.Error = "sbx ls: original multiline\nCLI diagnostic"
		if cached {
			mod.inventory.Sandboxes = []sbx.Sandbox{{ID: "stable-id", Status: "running", Ports: []sbx.Port{port}}}
		}
		summary, mapping := mod.view(route)
		if summary.State != "stale" || summary.Detail != "Cannot verify sandbox state." {
			t.Fatalf("cached=%v: unexpected summary: %+v", cached, summary)
		}
		if (mapping != nil) != cached {
			t.Fatalf("cached=%v: usable mapping changed", cached)
		}
		if mod.inventory.Error != "sbx ls: original multiline\nCLI diagnostic" {
			t.Fatal("raw discovery diagnostic was modified")
		}
	}
}
