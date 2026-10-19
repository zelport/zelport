// Package relay hosts the optional relay server without enabling a relay client.
package relay

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	server "github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
)

//go:embed assets/*
var assets embed.FS

type Settings struct {
	Enabled            bool   `json:"enabled"`
	RequireAccessToken bool   `json:"require_access_token"`
	Port               int    `json:"port"`
	BindAddress        string `json:"bind_address"`
	AccessToken        string `json:"access_token,omitempty"`
}

func settings(raw json.RawMessage) (Settings, error) {
	s := Settings{Port: 8090, RequireAccessToken: true}
	if err := feature.Decode(raw, &s); err != nil {
		return s, err
	}
	if s.Port < 1 || s.Port > 65535 {
		return s, errors.New("relay port must be between 1 and 65535")
	}
	if s.BindAddress != "" {
		addr, err := netip.ParseAddr(s.BindAddress)
		if err != nil || addr.IsUnspecified() || addr.IsMulticast() {
			return s, errors.New("The bind address must be a local IP address. Leave it blank to listen on all interfaces.")
		}
	}
	if s.AccessToken != "" {
		if _, err := protocol.DecodeKey(s.AccessToken); err != nil {
			return s, errors.New("invalid relay access token")
		}
	}
	if s.Enabled && s.RequireAccessToken && s.AccessToken == "" {
		return s, errors.New("enabled relay requires an access token")
	}
	return s, nil
}

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

func serverOptions(s Settings) (server.Options, error) {
	if !s.RequireAccessToken {
		return server.Options{AllowUnauthenticated: true}, nil
	}
	token, err := protocol.DecodeKey(s.AccessToken)
	return server.Options{AccessToken: token}, err
}
func prepare(s Settings) (*server.Runtime, error) {
	options, err := serverOptions(s)
	if err != nil {
		return nil, err
	}
	return server.Prepare(net.JoinHostPort(s.BindAddress, strconv.Itoa(s.Port)), options)
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

func (m *Module) save(ctx context.Context, revision uint64, next Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return feature.ErrClosed
	}
	snap := m.Host.Snapshot()
	if revision < snap.EditRevision || revision > snap.Revision {
		return feature.ErrConflict
	}
	old, err := settings(snap.Settings)
	if err != nil {
		return err
	}
	next.AccessToken = old.AccessToken
	if next.Enabled && next.RequireAccessToken && next.AccessToken == "" {
		next.AccessToken, err = protocol.Token()
		if err != nil {
			return err
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err = m.ValidateSettings(raw); err != nil {
		return err
	}
	keep := next.Enabled && m.runtime != nil && m.runtime.Status().State == "Listening" && old.Port == next.Port && old.BindAddress == next.BindAddress
	var candidate *server.Runtime
	if next.Enabled && !keep {
		candidate, err = prepare(next)
		if err != nil {
			if m.runtime != nil && old.Port == next.Port {
				return errors.New("The new bind address cannot be reserved. Disable the relay before changing overlapping listeners. Details: " + err.Error())
			}
			return err
		}
	}
	var replacement *server.Server
	if keep && old.RequireAccessToken != next.RequireAccessToken {
		options, e := serverOptions(next)
		if e != nil {
			return e
		}
		replacement, err = server.New(options)
		if err != nil {
			return err
		}
	}
	_, err = m.Host.Commit(ctx, revision, feature.CommitOptions{}, func(d *feature.Draft) error { d.Settings = raw; return nil })
	if err != nil {
		if replacement != nil {
			replacement.Close()
		}
		if candidate != nil {
			candidate.Close()
		}
		return err
	}
	if replacement != nil {
		m.runtime.ReplaceServer(replacement)
	}
	if !keep {
		if m.runtime != nil {
			m.runtime.Close()
		}
		m.runtime = candidate
		if candidate != nil {
			candidate.Start()
		}
	}
	m.failure = ""
	return nil
}

func (m *Module) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch {
		case r.Method == "GET" && r.URL.Path == "/settings":
			modulekit.JSON(w, m.State())
			return
		case r.Method == "PUT" && r.URL.Path == "/settings":
			var req struct {
				Revision           uint64 `json:"revision"`
				Enabled            bool   `json:"enabled"`
				Port               int    `json:"port"`
				BindAddress        string `json:"bind_address"`
				RequireAccessToken bool   `json:"require_access_token"`
			}
			req.RequireAccessToken = true
			err = feature.DecodeBody(w, r, &req)
			if err == nil {
				err = m.save(r.Context(), req.Revision, Settings{Enabled: req.Enabled, Port: req.Port, BindAddress: req.BindAddress, RequireAccessToken: req.RequireAccessToken})
			}
		case r.Method == "POST" && r.URL.Path == "/credentials":
			var req struct {
				Revision    uint64 `json:"revision"`
				Action      string `json:"action"`
				AccessToken string `json:"access_token"`
			}
			err = feature.DecodeBody(w, r, &req)
			if err == nil {
				m.mu.Lock()
				snap := m.Host.Snapshot()
				s, _ := settings(snap.Settings)
				if m.closed {
					err = feature.ErrClosed
				} else if req.Revision < snap.EditRevision || req.Revision > snap.Revision {
					err = feature.ErrConflict
				} else if req.Action == "reveal" && req.AccessToken == "" {
					if s.AccessToken == "" {
						err = errors.New("enable relay first to create its access token")
					} else {
						modulekit.JSON(w, map[string]string{"access_token": s.AccessToken})
					}
				} else if req.Action == "generate" || req.Action == "replace" {
					err = m.rotateToken(r.Context(), req.Revision, s, req.Action, req.AccessToken)
					if err == nil {
						modulekit.JSON(w, map[string]bool{"ok": true})
					}
				} else {
					err = errors.New("choose reveal, generate or replace")
				}
				m.mu.Unlock()
				if err == nil {
					return
				}
			}
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			code := 400
			if errors.Is(err, feature.ErrConflict) {
				code = 409
			}
			w.WriteHeader(code)
			modulekit.JSON(w, map[string]string{"error": err.Error()})
			return
		}
		modulekit.JSON(w, map[string]bool{"ok": true})
	})
}

// rotateToken runs under m.mu. Persistence must succeed before revoking any connection.
func (m *Module) rotateToken(ctx context.Context, revision uint64, next Settings, action, token string) error {
	if action == "generate" {
		if token != "" {
			return errors.New("generate does not accept an access token")
		}
		var err error
		token, err = protocol.Token()
		if err != nil {
			return err
		}
	}
	key, err := protocol.DecodeKey(token)
	if err != nil {
		return errors.New("access token must be a base64url-encoded 32-byte key")
	}
	oldKey, _ := protocol.DecodeKey(next.AccessToken)
	if bytes.Equal(key, oldKey) {
		return errors.New("choose a different access token")
	}
	next.AccessToken = base64.RawURLEncoding.EncodeToString(key)
	options, err := serverOptions(next)
	if err != nil {
		return err
	}
	candidate, err := server.New(options)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(next)
	if err != nil {
		candidate.Close()
		return err
	}
	_, err = m.Host.Commit(ctx, revision, feature.CommitOptions{}, func(d *feature.Draft) error { d.Settings = raw; return nil })
	if err != nil {
		candidate.Close()
		return err
	}
	if m.runtime != nil {
		m.runtime.ReplaceServer(candidate)
	} else {
		candidate.Close()
	}
	return nil
}

// ConnectionSummaries exposes the same visibility-independent state as the header.
func (m *Module) ConnectionSummaries() []feature.ConnectionSummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings, _ := settings(m.Host.Snapshot().Settings)
	state := "stopped"
	if settings.Enabled {
		if m.failure != "" {
			state = "error"
		} else if m.runtime != nil {
			switch m.runtime.Status().State {
			case "Listening":
				state = "listening"
			case "Error":
				state = "error"
			case "Stopped":
				state = "stopped"
			default:
				state = "starting"
			}
		} else {
			state = "starting"
		}
	}
	return []feature.ConnectionSummary{{ID: "built_in_relay", Label: "Built-in Relay", State: state}}
}
