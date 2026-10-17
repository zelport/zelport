package containers

import (
	"context"
	"encoding/json"
	engine "github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/feature"
	"strings"
	"testing"
)

type reviewBackend struct{ ports []engine.Port }

func (b reviewBackend) Scan(context.Context) engine.Inventory {
	return engine.Inventory{Engine: "docker", Connection: "local", Containers: []engine.Container{{ID: "stable", Status: "running", Ports: b.ports}}}
}

type statusBackend struct{ inventory engine.Inventory }

func (b *statusBackend) Scan(context.Context) engine.Inventory { return b.inventory }

func TestPermissionFailureKeepsRoutesAndMovesRawErrorToDiagnostics(t *testing.T) {
	selected := engine.Port{HostIP: "127.0.0.1", HostPort: 12000, ContainerPort: 3000, Protocol: "tcp"}
	for _, cached := range []bool{false, true} {
		backend := &statusBackend{inventory: engine.Inventory{Engine: "docker", Connection: "local"}}
		if cached {
			backend.inventory.Containers = []engine.Container{{ID: "stable", Status: "running", Ports: []engine.Port{selected}}}
		}
		m := New().(*Module)
		m.Catalog = engine.NewCatalog(backend)
		m.Catalog.Refresh(context.Background())
		backend.inventory = engine.Inventory{Engine: "docker", Connection: "local", RuntimeStatus: "permission_denied", Stale: true, Error: "permission denied while trying to connect to the docker API at unix:///var/run/docker.sock"}
		m.Catalog.Refresh(context.Background())
		settings, _ := json.Marshal(Settings{Container: &engine.Reference{Engine: "docker", Connection: "local", ID: "stable", Port: 3000, Selected: selected}})
		view, target := m.view(feature.Route{Settings: settings})
		if view.State != "stale" || (target != nil) != cached || !strings.Contains(view.Detail, "Containers") || strings.Contains(view.Detail, "unix://") {
			t.Fatalf("cached=%v: view=%+v target=%v", cached, view, target)
		}
		if m.Catalog.Snapshot()[0].Error != backend.inventory.Error {
			t.Fatal("raw diagnostics were lost")
		}
	}
}
func TestMappingFallbackMustBeUniqueAndSameAddress(t *testing.T) {
	selected := engine.Port{HostIP: "127.0.0.1", HostPort: 12000, ContainerPort: 3000, Protocol: "tcp"}
	a := selected
	a.HostPort = 12001
	b := selected
	b.HostPort = 12002
	c := selected
	c.HostIP = "::1"
	for _, tc := range []struct {
		name  string
		ports []engine.Port
		want  bool
	}{{"ambiguous", []engine.Port{a, b}, false}, {"different address", []engine.Port{c}, false}, {"unique", []engine.Port{a}, true}, {"exact", []engine.Port{a, selected, b}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			m := New().(*Module)
			m.Catalog = engine.NewCatalog(reviewBackend{tc.ports})
			m.Catalog.Refresh(context.Background())
			settings, _ := json.Marshal(Settings{Container: &engine.Reference{Engine: "docker", Connection: "local", ID: "stable", Port: 3000, Selected: selected}})
			_, got := m.view(feature.Route{Settings: settings})
			if (got != nil) != tc.want {
				t.Fatalf("mapping %v, want usable=%v", got, tc.want)
			}
		})
	}
}
