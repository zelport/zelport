package ports_over_ssh

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"github.com/zelport/zelport/internal/sshpipe"
	"io/fs"
	"net/http"
	"net/url"
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
	mu     sync.Mutex
	pools  map[string]poolEntry
	closed bool
}

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	m := &Module{pools: map[string]poolEntry{}}
	m.Desc = feature.Descriptor{Maturity: feature.Experimental, ID: "ports_over_ssh", Name: "Ports over SSH", TabID: "ssh", Order: 50, Exportable: true, PollInterval: 15e9, Assets: files}
	m.Summary = func(r feature.Route) feature.Summary {
		var s Settings
		feature.Decode(r.Settings, &s)
		return feature.Summary{State: "configured", Target: fmt.Sprintf("%s → 127.0.0.1:%d", s.SSHHost, s.SSHPort)}
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
	m.Run(ctx, func(context.Context) {
		rs := h.Snapshot().Routes
		m.mu.Lock()
		defer m.mu.Unlock()
		for id, p := range m.pools {
			found := false
			for _, r := range rs {
				if id == r.ID && p.settings == string(r.Settings)+r.Domain {
					found = true
				}
			}
			if !found {
				p.pool.Close()
				delete(m.pools, id)
			}
		}
	}, 15e9)
	return nil
}
func (m *Module) Close() error {
	m.Base.Close()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for _, p := range m.pools {
		p.pool.Close()
	}
	return nil
}
func (m *Module) State() feature.State {
	s := m.Base.State()
	m.mu.Lock()
	defer m.mu.Unlock()
	statuses := map[string]sshpipe.Status{}
	for id, p := range m.pools {
		statuses[id] = p.pool.Status()
	}
	s.Data, _ = json.Marshal(map[string]any{"connections": statuses})
	return s
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	var s Settings
	feature.Decode(v.Settings, &s)
	m.mu.Lock()
	if m.closed || r.Context().Err() != nil {
		m.mu.Unlock()
		feature.Status(w, r, 503, "SSH unavailable", "The route is closing.")
		return
	}
	key := string(v.Settings) + v.Domain
	p, ok := m.pools[v.ID]
	if !ok || p.settings != key {
		if ok {
			p.pool.Close()
		}
		p = poolEntry{key, sshpipe.New(s.SSHHost, s.SSHPort)}
		m.pools[v.ID] = p
	}
	m.mu.Unlock()
	modulekit.Proxy(w, r, &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(s.SSHPort)}, modulekit.ProxyOptions{PreserveHost: true, Dial: p.pool.DialContext, Error: func(w http.ResponseWriter, r *http.Request, e error) {
		code := 502
		var failure *sshpipe.Error
		if errors.As(e, &failure) && !failure.Backend {
			code = 503
		}
		feature.Status(w, r, code, "SSH service unavailable", e.Error())
	}})
}
