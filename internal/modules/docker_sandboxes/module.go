package docker_sandboxes

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"github.com/zelport/zelport/internal/sbx"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

//go:embed assets/*
var assets embed.FS

type Settings struct {
	SandboxID   string    `json:"sandbox_id"`
	SandboxName string    `json:"sandbox_name"`
	SandboxPort int       `json:"sandbox_port"`
	Selected    *sbx.Port `json:"selected,omitempty"`
	Deleting    bool      `json:"deleting,omitempty"`
}
type inventory struct {
	PortErrors map[string]string `json:"port_errors,omitempty"`
	CLI        sbx.CLIInfo       `json:"cli"`
	Sandboxes  []sbx.Sandbox     `json:"sandboxes"`
	Stale      bool              `json:"stale"`
	Error      string            `json:"error,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}
type Module struct {
	modulekit.Base
	Backend     sbx.Backend
	op          sync.Mutex
	mu          sync.RWMutex
	inventory   inventory
	portErrors  map[string]string
	infoChecked bool
	Launch      func(context.Context, string) error
}

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	m := &Module{Backend: sbx.Client{}, Launch: openTerminal, portErrors: map[string]string{}, inventory: inventory{Sandboxes: []sbx.Sandbox{}, Stale: true}}
	m.Desc = feature.Descriptor{Maturity: feature.Stable, ID: "docker_sandboxes", Name: "Docker Sandboxes", TabID: "sandboxes", Order: 10, Exportable: true, PollInterval: 15 * time.Second, Assets: files}
	return m
}
func (m *Module) ValidateRoute(r feature.Route) error {
	var s Settings
	if e := feature.Decode(r.Settings, &s); e != nil {
		return e
	}
	if s.SandboxID == "" || strings.HasPrefix(s.SandboxID, "-") || s.SandboxPort < 1 || s.SandboxPort > 65535 {
		return fmt.Errorf("select a sandbox and a Sandbox port between 1 and 65535")
	}
	if s.Selected != nil && (!s.Selected.Usable() || s.Selected.SandboxPort != s.SandboxPort) {
		return fmt.Errorf("select a loopback TCP mapping for the Sandbox port")
	}
	if m.Host != nil {
		var previous *Settings
		for _, old := range m.Host.Snapshot().Routes {
			if old.ID == r.ID {
				var v Settings
				feature.Decode(old.Settings, &v)
				previous = &v
			}
		}
		if previous != nil {
			if previous.SandboxID != s.SandboxID {
				return errors.New("a route cannot change its sandbox identity")
			}
			if previous.Deleting && !s.Deleting {
				return errors.New("Route removal is pending. Finish removing it first.")
			}
		}
		if previous == nil {
			found := false
			m.mu.RLock()
			for _, box := range m.inventory.Sandboxes {
				if box.ID == s.SandboxID {
					found = true
				}
			}
			m.mu.RUnlock()
			if !found {
				return errors.New("The sandbox was not found. Refresh the list.")
			}
		}
	}
	return nil
}
func (m *Module) Start(ctx context.Context, h feature.Host) error {
	m.Base.Start(ctx, h)
	m.Run(ctx, m.Refresh, 15*time.Second)
	return nil
}
func choose(ports []sbx.Port, s Settings) *sbx.Port {
	if s.Selected != nil {
		for _, p := range ports {
			if p == *s.Selected && p.Usable() {
				v := p
				return &v
			}
		}
	}
	for _, p := range ports {
		if p.SandboxPort == s.SandboxPort && p.Usable() {
			v := p
			return &v
		}
	}
	return nil
}
func (m *Module) Refresh(ctx context.Context) { m.op.Lock(); defer m.op.Unlock(); m.refresh(ctx) }

// refreshCLI runs under op, separately from periodic inventory discovery.
func (m *Module) refreshCLI(ctx context.Context) {
	m.infoChecked = true
	info := sbx.CLIInfo{Status: "unknown"}
	if provider, ok := m.Backend.(interface {
		Info(context.Context) sbx.CLIInfo
	}); ok {
		info = provider.Info(ctx)
	}
	m.mu.Lock()
	m.inventory.CLI = info
	m.mu.Unlock()
}
func (m *Module) refresh(ctx context.Context) {
	if !m.infoChecked {
		m.refreshCLI(ctx)
	}
	boxes, e := m.Backend.List(ctx)
	if e != nil {
		m.mu.Lock()
		m.inventory.Stale = true
		m.inventory.Error = e.Error()
		m.mu.Unlock()
		return
	}
	snapshot := m.Host.Snapshot()
	errs := map[string]string{}
	for i, box := range boxes {
		if box.Status != "running" {
			continue
		}
		ports, e := m.Backend.Ports(ctx, box.ID)
		if e != nil {
			errs[box.ID] = e.Error()
			m.mu.RLock()
			for _, old := range m.inventory.Sandboxes {
				if old.ID == box.ID {
					ports = old.Ports
				}
			}
			m.mu.RUnlock()
			boxes[i].Ports = ports
			continue
		}
		for _, r := range snapshot.Routes {
			var s Settings
			feature.Decode(r.Settings, &s)
			if s.SandboxID != box.ID || s.Deleting || choose(ports, s) != nil {
				continue
			}
			lease, e := m.Host.Acquire(ctx, r.ID, snapshot.Revision)
			if e != nil {
				continue
			}
			e = m.Backend.Publish(lease.Context(), box.ID, s.SandboxPort)
			lease.Close()
			if e != nil {
				errs[box.ID] = e.Error()
				continue
			}
			nextPorts, readErr := m.Backend.Ports(ctx, box.ID)
			if readErr != nil {
				errs[box.ID] = readErr.Error()
				break
			}
			ports = nextPorts
		}
		boxes[i].Ports = ports
	}
	m.mu.Lock()
	m.inventory = inventory{CLI: m.inventory.CLI, PortErrors: errs, Sandboxes: boxes, UpdatedAt: time.Now(), Stale: len(errs) > 0}
	m.portErrors = errs
	m.mu.Unlock()
}
func (m *Module) view(r feature.Route) (feature.Summary, *sbx.Port) {
	var s Settings
	feature.Decode(r.Settings, &s)
	if s.Deleting {
		return feature.Summary{State: "waiting", Detail: "Removal is pending. Try removing the route again."}, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	summary := feature.Summary{State: "unavailable", Detail: "Sandbox was not found. Configuration is preserved.", Target: fmt.Sprintf("Sandbox port %d → Not mapped", s.SandboxPort)}
	for _, box := range m.inventory.Sandboxes {
		if box.ID != s.SandboxID {
			continue
		}
		if box.Status != "running" {
			summary.Detail = "Sandbox is stopped."
			return summary, nil
		}
		p := choose(box.Ports, s)
		if p != nil {
			summary.State = "ready"
			summary.Detail = ""
			summary.Target = fmt.Sprintf("Sandbox port %d → Aux port %d", s.SandboxPort, p.HostPort)
			if m.inventory.Stale {
				summary.State = "stale"
				summary.Detail = "Cannot verify sandbox state."
			}
			return summary, p
		}
		summary.State = "waiting"
		summary.Detail = "Port mapping is being prepared."
		if e := m.portErrors[box.ID]; e != "" {
			summary.State = "error"
			summary.Detail = e
		}
		return summary, nil
	}
	if m.inventory.Stale {
		summary.State = "stale"
		summary.Detail = "Cannot verify sandbox state."
	}
	return summary, nil
}
func (m *Module) State() feature.State {
	s := m.Host.Snapshot()
	v := feature.State{Revision: s.Revision, Routes: []feature.RouteView{}}
	for _, r := range s.Routes {
		summary, _ := m.view(r)
		v.Routes = append(v.Routes, feature.View(r, summary))
	}
	m.mu.RLock()
	v.Data, _ = json.Marshal(m.inventory)
	m.mu.RUnlock()
	return v
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	s, p := m.view(v)
	if p == nil {
		feature.Status(w, r, 503, "Sandbox service unavailable", s.Detail)
		return
	}
	modulekit.Proxy(w, r, &url.URL{Scheme: "http", Host: p.Address()}, modulekit.ProxyOptions{PreserveHost: true, ListenerAuthority: m.Host.ListenerAuthority("localhost")})
}
func (m *Module) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var req struct {
			Revision uint64 `json:"revision"`
			ID       string `json:"id"`
		}
		if e := feature.DecodeBody(w, r, &req); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		m.op.Lock()
		defer m.op.Unlock()
		var e error
		switch r.URL.Path {
		case "/refresh":
			m.refreshCLI(r.Context())
			m.refresh(r.Context())
		case "/terminal":
			boxes, err := m.Backend.List(r.Context())
			e = err
			if e == nil {
				e = feature.ErrNotFound
				for _, box := range boxes {
					if box.ID == req.ID {
						e = m.Launch(r.Context(), box.Name)
						break
					}
				}
			}
		case "/start", "/remove-mapping":
			snap := m.Host.Snapshot()
			if snap.Revision != req.Revision {
				e = feature.ErrConflict
				break
			}
			var route feature.Route
			for _, v := range snap.Routes {
				if v.ID == req.ID {
					route = v
				}
			}
			if route.ID == "" {
				e = feature.ErrNotFound
				break
			}
			var s Settings
			feature.Decode(route.Settings, &s)
			if r.URL.Path == "/start" {
				if s.Deleting {
					e = errors.New("route removal is pending")
					break
				}
				boxes, listErr := m.Backend.List(r.Context())
				e = listErr
				if e == nil {
					e = feature.ErrNotFound
					for _, box := range boxes {
						if box.ID != s.SandboxID {
							continue
						}
						e = nil
						if box.Status != "running" {
							lease, acquireErr := m.Host.Acquire(r.Context(), route.ID, snap.Revision)
							e = acquireErr
							if e == nil {
								e = m.Backend.Start(lease.Context(), s.SandboxID, s.SandboxPort)
								lease.Close()
							}
						}
						break
					}
				}
				if e == nil {
					m.refresh(r.Context())
				}
			} else {
				e = m.remove(r.Context(), snap, route, s)
			}
		default:
			http.NotFound(w, r)
			return
		}
		if e != nil {
			code := 400
			if errors.Is(e, feature.ErrConflict) {
				code = 409
			}
			http.Error(w, e.Error(), code)
			return
		}
		modulekit.JSON(w, map[string]any{"ok": true})
	})
}
func (m *Module) remove(ctx context.Context, snap feature.Snapshot, r feature.Route, s Settings) error {
	for _, other := range snap.Routes {
		var v Settings
		feature.Decode(other.Settings, &v)
		if other.ID != r.ID && s.SandboxID == v.SandboxID && s.SandboxPort == v.SandboxPort {
			return errors.New("Another domain uses this sandbox port. Remove that route first.")
		}
	}
	boxes, e := m.Backend.List(ctx)
	if e != nil {
		return e
	}
	var target *sbx.Port
	for _, box := range boxes {
		if box.ID != s.SandboxID {
			continue
		}
		ports, e := m.Backend.Ports(ctx, box.ID)
		if e != nil {
			return e
		}
		if s.Deleting {
			for _, p := range ports {
				if s.Selected != nil && p == *s.Selected {
					v := p
					target = &v
				}
			}
		} else {
			target = choose(ports, s)
		}
	}
	if !s.Deleting {
		s.Deleting = true
		s.Selected = target
		b, _ := json.Marshal(s)
		result, e := m.Host.Commit(ctx, snap.Revision, feature.CommitOptions{}, func(d *feature.Draft) error {
			for i := range d.Routes {
				if d.Routes[i].ID == r.ID {
					d.Routes[i].Settings = b
				}
			}
			return nil
		})
		if e != nil {
			return e
		}
		snap.Revision = result.Revision
	}
	if target != nil {
		if e = m.Backend.Unpublish(ctx, s.SandboxID, *target); e != nil {
			return fmt.Errorf("Removal is pending. The retry failed: %w", e)
		}
	}
	_, e = m.Host.Commit(ctx, snap.Revision, feature.CommitOptions{}, func(d *feature.Draft) error {
		v := []feature.Route{}
		for _, x := range d.Routes {
			if x.ID != r.ID {
				v = append(v, x)
			}
		}
		d.Routes = v
		return nil
	})
	return e
}
