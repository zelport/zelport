// Package relay hosts the optional relay server without enabling a relay client.
package relay

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"sync"
	"time"

	server "github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
)

//go:embed assets/*
var assets embed.FS

type Module struct {
	modulekit.Base
	mu      sync.Mutex // Serializes persistence, listener replacement, status and shutdown.
	runtime *server.Runtime
	failure string
	closed  bool
}

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	return &Module{Base: modulekit.Base{Desc: feature.Descriptor{Maturity: feature.Experimental, ID: "built_in_relay", Name: "Built-in Relay", UIPlacement: "settings", Order: 70, PollInterval: 15 * time.Second, Assets: files}}}
}
func (m *Module) ValidateSettings(raw json.RawMessage) error { _, err := settings(raw); return err }
func (m *Module) ValidateRoute(feature.Route) error {
	return errors.New("relay hosting does not provide application routes")
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, _ feature.Route) {
	http.NotFound(w, r)
}

func (m *Module) Start(ctx context.Context, h feature.Host) error {
	m.Base.Start(ctx, h)
	s, err := settings(h.Snapshot().Settings)
	if err != nil {
		return err
	}
	if s.Enabled {
		m.runtime, err = prepare(s)
		if err != nil {
			m.failure = err.Error()
		} else {
			m.runtime.Start()
		}
	}
	return nil
}
func (m *Module) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.runtime != nil {
		m.runtime.Close()
		m.runtime = nil
	}
	return nil
}
func (m *Module) State() feature.State {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap := m.Host.Snapshot()
	s, _ := settings(snap.Settings)
	status := server.RuntimeStatus{State: "Stopped", Listeners: []string{}}
	if m.runtime != nil {
		status = m.runtime.Status()
	}
	if m.failure != "" {
		status.State = "Error"
		status.Error = m.failure
	}
	data, _ := json.Marshal(struct {
		Enabled            bool                 `json:"enabled"`
		Port               int                  `json:"port"`
		BindAddress        string               `json:"bind_address"`
		HasToken           bool                 `json:"has_token"`
		RequireAccessToken bool                 `json:"require_access_token"`
		Status             server.RuntimeStatus `json:"status"`
	}{s.Enabled, s.Port, s.BindAddress, s.AccessToken != "", s.RequireAccessToken, status})
	return feature.State{Revision: snap.Revision, Routes: []feature.RouteView{}, Data: data}
}
