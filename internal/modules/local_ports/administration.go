package local_ports

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
)

func localAPIError(w http.ResponseWriter, code int, err error) {
	if errors.Is(err, feature.ErrConflict) {
		code = 409
	}
	w.WriteHeader(code)
	modulekit.JSON(w, map[string]string{"error": err.Error()})
}
func (m *Module) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/settings":
			m.settings(w, r)
		case "/listeners/refresh":
			if r.Method != "POST" {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", 405)
				return
			}
			m.sandboxes.Invalidate()
			modulekit.JSON(w, map[string]any{})
		case "/listeners":
			if r.Method != "GET" {
				w.Header().Set("Allow", "GET")
				http.Error(w, "method not allowed", 405)
				return
			}
			m.configMu.RLock()
			defer m.configMu.RUnlock()
			value, err := m.configuration()
			if err != nil {
				localAPIError(w, 503, err)
				return
			}
			result := struct {
				Listeners  []Listener `json:"listeners"`
				Configured []Listener `json:"configured_listeners"`
				Enabled    bool       `json:"discovery_enabled"`
			}{[]Listener{}, []Listener{}, value.DiscoveryEnabled}
			if value.DiscoveryEnabled {
				ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
				defer cancel()
				all, err := discoverListeners(ctx, m.scanner, authorityPort(m.Host))
				if err != nil {
					localAPIError(w, 503, errors.New("Listening ports couldn't be detected. Enter the service port manually."))
					return
				}
				labelSandboxes(ctx, m.sandboxes, all)
				configured := map[int]bool{}
				if m.Host != nil {
					for _, route := range m.Host.Snapshot().Routes {
						var s Settings
						if feature.Decode(route.Settings, &s) == nil {
							configured[s.AuxPort] = true
						}
					}
				}
				for _, listener := range all {
					if !value.exclude(listener) {
						result.Listeners = append(result.Listeners, listener)
					}
					if configured[listener.Port] {
						result.Configured = append(result.Configured, listener)
					}
				}
			}
			modulekit.JSON(w, result)
		default:
			http.NotFound(w, r)
		}
	})
}
func (m *Module) settings(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		modulekit.JSON(w, m.State())
		return
	}
	if r.Method != "PUT" {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", 405)
		return
	}
	var request struct {
		Revision  uint64   `json:"revision"`
		Enabled   *bool    `json:"discovery_enabled"`
		Ports     []string `json:"ignored_ports"`
		Processes []string `json:"ignored_processes"`
	}
	if err := feature.DecodeBody(w, r, &request); err != nil || request.Enabled == nil {
		localAPIError(w, 400, errors.New("Invalid Local Ports settings request."))
		return
	}
	raw, _ := json.Marshal(Configuration{*request.Enabled, request.Ports, request.Processes})
	value, err := configuration(raw)
	if err != nil {
		localAPIError(w, 400, err)
		return
	}
	raw, _ = json.Marshal(value)
	m.configMu.Lock()
	defer m.configMu.Unlock()
	if m.Host == nil {
		localAPIError(w, 503, feature.ErrClosed)
		return
	}
	result, err := m.Host.Commit(r.Context(), request.Revision, feature.CommitOptions{}, func(draft *feature.Draft) error { draft.Settings = raw; return nil })
	if err != nil {
		localAPIError(w, 400, err)
		return
	}
	modulekit.JSON(w, result)
}
