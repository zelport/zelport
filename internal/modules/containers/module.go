package containers

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	engine "github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"io/fs"
	"net/http"
	"net/url"
	"time"
)

//go:embed assets/*
var assets embed.FS

type Settings struct {
	Container *engine.Reference `json:"container"`
}
type Module struct {
	modulekit.Base
	Catalog *engine.Catalog
}

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	m := &Module{Catalog: engine.NewCatalog(engine.Client{Engine: "docker"}, engine.Client{Engine: "podman"}, engine.Client{Engine: "nerdctl"})}
	m.Desc = feature.Descriptor{Maturity: feature.Experimental, ID: "containers", Name: "Containers", TabID: "containers", Order: 20, Exportable: true, PollInterval: 15 * time.Second, Assets: files}
	return m
}
func (m *Module) ValidateRoute(r feature.Route) error {
	var s Settings
	if e := feature.Decode(r.Settings, &s); e != nil {
		return e
	}
	c := s.Container
	if c == nil || (c.Engine != "docker" && c.Engine != "podman" && c.Engine != "nerdctl") || c.Connection == "" || c.ID == "" || c.Port < 1 || c.Port > 65535 || !c.Selected.Usable() || c.Selected.ContainerPort != c.Port {
		return fmt.Errorf("select an existing published loopback TCP port")
	}
	if m.Host != nil && modulekit.SelfTarget(&url.URL{Scheme: "http", Host: c.Selected.Address()}, m.Host.ListenerAuthority("localhost")) {
		return fmt.Errorf("Aux port cannot be the Zelport Localhost port")
	}
	if m.Host != nil {
		unchanged := false
		for _, old := range m.Host.Snapshot().Routes {
			if old.ID == r.ID {
				var previous Settings
				feature.Decode(old.Settings, &previous)
				p := previous.Container
				if p.Engine != c.Engine || p.Connection != c.Connection || p.ID != c.ID {
					return fmt.Errorf("a route cannot change its container identity")
				}
				unchanged = p.Port == c.Port && p.Selected == c.Selected
			}
		}
		if !unchanged {
			found := false
			for _, inv := range m.Catalog.Snapshot() {
				if inv.Engine != c.Engine || inv.Connection != c.Connection || inv.Stale {
					continue
				}
				for _, box := range inv.Containers {
					if box.ID != c.ID {
						continue
					}
					for _, p := range box.Ports {
						if p == c.Selected {
							found = true
						}
					}
				}
			}
			if !found {
				return fmt.Errorf("The selected container mapping is unavailable. Refresh the list.")
			}
		}
	}
	return nil
}
func (m *Module) Start(ctx context.Context, h feature.Host) error {
	m.Base.Start(ctx, h)
	m.Run(ctx, m.Catalog.Refresh, 15*time.Second)
	return nil
}
func (m *Module) view(r feature.Route) (feature.Summary, *engine.Port) {
	var s Settings
	feature.Decode(r.Settings, &s)
	c := s.Container
	v := feature.Summary{State: "unavailable", Detail: "Container was not found. Configuration is preserved.", Target: fmt.Sprintf("Container port %d → Not mapped", c.Port)}
	for _, inv := range m.Catalog.Snapshot() {
		if inv.Engine != c.Engine || inv.Connection != c.Connection {
			continue
		}
		detail := inv.Error
		if inv.RuntimeStatus == "permission_denied" {
			detail = "Container engine access denied. Open Containers in the header for help."
		}
		for _, box := range inv.Containers {
			if box.ID != c.ID {
				continue
			}
			if box.Status != "running" {
				v.Detail = "Container is stopped."
				return v, nil
			}
			var target *engine.Port
			var candidates []engine.Port
			for _, p := range box.Ports {
				if !p.Usable() {
					continue
				}
				if p == c.Selected {
					copy := p
					target = &copy
					break
				}
				if p.ContainerPort == c.Port && p.HostIP == c.Selected.HostIP {
					candidates = append(candidates, p)
				}
			}
			if target == nil && len(candidates) == 1 {
				target = &candidates[0]
			}
			if target != nil {
				v.State = "ready"
				v.Detail = ""
				v.Target = fmt.Sprintf("Container port %d → Aux port %d", c.Port, target.HostPort)
				if inv.Stale {
					v.State = "stale"
					v.Detail = detail
				}
				return v, target
			}
			v.Detail = "Published mapping is missing."
			return v, nil
		}
		if inv.Stale {
			v.State = "stale"
			v.Detail = detail
		}
	}
	return v, nil
}
func (m *Module) State() feature.State {
	s := m.Host.Snapshot()
	v := feature.State{Revision: s.Revision, Routes: []feature.RouteView{}}
	for _, r := range s.Routes {
		summary, _ := m.view(r)
		v.Routes = append(v.Routes, feature.View(r, summary))
	}
	v.Data, _ = json.Marshal(map[string]any{"engines": m.Catalog.Snapshot()})
	return v
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	s, p := m.view(v)
	if p == nil {
		feature.Status(w, r, 503, "Container service unavailable", s.Detail)
		return
	}
	if modulekit.SelfTarget(&url.URL{Scheme: "http", Host: p.Address()}, m.Host.ListenerAuthority("localhost")) {
		feature.Status(w, r, 503, "Invalid Aux port", "The Aux port cannot be the Zelport Localhost port.")
		return
	}
	modulekit.Proxy(w, r, &url.URL{Scheme: "http", Host: p.Address()}, modulekit.ProxyOptions{PreserveHost: true, ListenerAuthority: m.Host.ListenerAuthority("localhost")})
}
func (m *Module) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/refresh" {
			http.NotFound(w, r)
			return
		}
		m.Catalog.Refresh(r.Context())
		modulekit.JSON(w, map[string]bool{"ok": true})
	})
}
