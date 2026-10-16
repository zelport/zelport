package containers

import (
	"context"
	"encoding/json"
	engine "github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/feature"
	"testing"
)

type reviewBackend struct{ ports []engine.Port }

func (b reviewBackend) Scan(context.Context) engine.Inventory {
	return engine.Inventory{Engine: "docker", Connection: "local", Containers: []engine.Container{{ID: "stable", Status: "running", Ports: b.ports}}}
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
