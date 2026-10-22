package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	server "github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
)

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
