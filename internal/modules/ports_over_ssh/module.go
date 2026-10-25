package ports_over_ssh

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/configfile"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"github.com/zelport/zelport/internal/sshpipe"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
)

//go:embed assets/*
var assets embed.FS
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,254}$`)

type Settings struct {
	SSHHost string `json:"ssh_host"`
	SSHPort int    `json:"ssh_port"`
}
type poolEntry struct {
	settings string
	pool     *sshpipe.Pool
}
type Module struct {
	modulekit.Base
	fileEditor configfile.Editor
	configMu   sync.RWMutex // Serializes saved path changes and editor launches with pool selection.
	mu         sync.Mutex
	pools      map[string]poolEntry
	closed     bool
}

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	m := &Module{pools: map[string]poolEntry{}}
	m.Desc = feature.Descriptor{Maturity: feature.Experimental, ID: "ports_over_ssh", Name: "Ports over SSH", TabID: "ssh", Order: 50, Exportable: true, PollInterval: 15e9, Assets: files}
	m.Summary = func(r feature.Route) feature.Summary {
		var s Settings
		feature.Decode(r.Settings, &s)
		return feature.Summary{State: "configured", Target: fmt.Sprintf("%s · port %d", s.SSHHost, s.SSHPort)}
	}
	return m
}
func (m *Module) ValidateRoute(r feature.Route) error {
	var s Settings
	if e := feature.Decode(r.Settings, &s); e != nil {
		return e
	}
	if !hostPattern.MatchString(s.SSHHost) || s.SSHPort < 1 || s.SSHPort > 65535 {
		return fmt.Errorf("use a valid SSH host alias and a target port between 1 and 65535")
	}
	return nil
}

func (m *Module) Start(ctx context.Context, h feature.Host) error {
	m.Base.Start(ctx, h)
	m.Run(ctx, func(context.Context) { m.prunePools() }, 15e9)
	return nil
}
func (m *Module) Close() error {
	m.Base.Close()
	m.configMu.Lock()
	defer m.configMu.Unlock()
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.retirePools()
	return nil
}
func (m *Module) retirePools() {
	m.mu.Lock()
	pools := m.pools
	m.pools = map[string]poolEntry{}
	m.mu.Unlock()
	for _, entry := range pools {
		entry.pool.Close()
	}
}
func poolKey(route feature.Route, path string) string {
	return string(route.Settings) + "\x00" + route.Domain + "\x00" + path
}
func (m *Module) prunePools() {
	m.configMu.RLock()
	_, snapshot, err := m.configSnapshot()
	path, pathErr := m.selectedConfig()
	keys := map[string]string{}
	if err == nil && pathErr == nil {
		for _, route := range snapshot.Routes {
			keys[route.ID] = poolKey(route, path)
		}
	}
	var retired []*sshpipe.Pool
	m.mu.Lock()
	for id, entry := range m.pools {
		if keys[id] != entry.settings {
			retired = append(retired, entry.pool)
			delete(m.pools, id)
		}
	}
	m.mu.Unlock()
	m.configMu.RUnlock()
	for _, pool := range retired {
		pool.Close()
	}
}

func (m *Module) State() feature.State {
	value, snapshot, configErr := m.configSnapshot()
	s := feature.State{Revision: snapshot.Revision, Routes: []feature.RouteView{}}
	statuses := map[string]sshpipe.Status{}
	m.mu.Lock()
	for id, entry := range m.pools {
		statuses[id] = entry.pool.Status()
	}
	m.mu.Unlock()
	for _, route := range snapshot.Routes {
		view := feature.View(route, m.Summary(route))
		if status, ok := statuses[route.ID]; ok && status.State == "error" {
			view.Summary.State = "unavailable"
			view.Summary.Detail = "SSH connection failed. Check the host in your SSH configuration."
		}
		s.Routes = append(s.Routes, view)
	}
	path, err := sshpipe.ResolveConfigPath(value.SSHConfigPath)
	if configErr != nil {
		err = configErr
	}
	exists := false
	if err == nil {
		info, statErr := os.Stat(path)
		exists = statErr == nil && info.Mode().IsRegular()
	}
	s.Data, _ = json.Marshal(map[string]any{"connections": statuses, "ssh_config_path": value.SSHConfigPath, "effective_config_path": path, "config_exists": exists})
	return s
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	var s Settings
	feature.Decode(v.Settings, &s)
	m.configMu.RLock()
	path, err := m.selectedConfig()
	if err != nil {
		m.configMu.RUnlock()
		feature.Status(w, r, 503, "SSH configuration unavailable", "Review the SSH configuration file in Configuration.")
		return
	}
	m.mu.Lock()
	if m.closed || r.Context().Err() != nil {
		m.mu.Unlock()
		m.configMu.RUnlock()
		feature.Status(w, r, 503, "SSH unavailable", "The route is closing.")
		return
	}
	key := poolKey(v, path)
	p, ok := m.pools[v.ID]
	if !ok || p.settings != key {
		if ok {
			p.pool.Close()
		}
		p = poolEntry{key, sshpipe.NewWithConfig(s.SSHHost, s.SSHPort, path)}
		m.pools[v.ID] = p
	}
	m.mu.Unlock()
	m.configMu.RUnlock()
	modulekit.Proxy(w, r, &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(s.SSHPort)}, modulekit.ProxyOptions{PreserveHost: true, Dial: p.pool.DialContext, Error: func(w http.ResponseWriter, r *http.Request, e error) {
		code := 502
		var failure *sshpipe.Error
		if errors.As(e, &failure) && !failure.Backend {
			code = 503
		}
		feature.StatusDiagnostic(w, r, code, "SSH service unavailable", "The SSH connection could not be established.", e.Error())
	}})
}
