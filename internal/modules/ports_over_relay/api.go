package ports_over_relay

import (
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"net/http"
)

type settingsRequest struct {
	Revision         uint64 `json:"revision"`
	Enabled          bool   `json:"enabled"`
	Publish          bool   `json:"publish"`
	RelayURL         string `json:"relay_url"`
	AccessToken      string `json:"access_token"`
	ClearAccessToken bool   `json:"clear_access_token"`
	GroupToken       string `json:"group_token"`
}

func (m *Module) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var err error
		switch {
		case r.Method == "GET" && r.URL.Path == "/settings":
			modulekit.JSON(w, m.State())
			return
		case r.Method == "PUT" && r.URL.Path == "/settings":
			var req settingsRequest
			if err = feature.DecodeBody(w, r, &req); err != nil {
				break
			}
			old, rev := m.store.snapshot()
			if req.Revision > rev || req.Revision < m.Host.Snapshot().EditRevision {
				err = feature.ErrConflict
				break
			}
			s := old
			if s.DeviceID == "" {
				s, err = newIdentity()
				if err != nil {
					break
				}
			}
			s.Enabled = req.Enabled
			s.Publish = req.Publish
			s.RelayURL = req.RelayURL
			if req.ClearAccessToken && req.AccessToken != "" {
				err = errors.New("cannot set and clear the access token together")
				break
			}
			if req.ClearAccessToken {
				s.AccessToken = ""
			}
			if req.AccessToken != "" {
				s.AccessToken = req.AccessToken
			}
			if req.GroupToken != "" {
				s.GroupToken = req.GroupToken
			}
			resets := []string{}
			if s.Enabled != old.Enabled || s.RelayURL != old.RelayURL || s.AccessToken != old.AccessToken || s.GroupToken != old.GroupToken {
				resets = []string{"client", "sharing"}
			} else if s.Publish != old.Publish {
				resets = []string{"sharing"}
			}
			_, err = m.Host.Commit(r.Context(), req.Revision, feature.CommitOptions{ResetScopes: resets}, func(d *feature.Draft) error {
				var latest Settings
				if e := feature.Decode(d.Settings, &latest); e != nil {
					return e
				}
				s.Sequence = latest.Sequence
				s.Watermarks = latest.Watermarks
				s.Peers = latest.Peers
				if latest.AdminServiceID != "" {
					s.AdminServiceID = latest.AdminServiceID
				}
				b, e := json.Marshal(s)
				d.Settings = b
				return e
			})
		case r.Method == "POST" && r.URL.Path == "/credentials":
			var req struct {
				Revision uint64 `json:"revision"`
				Action   string `json:"action"`
			}
			if err = feature.DecodeBody(w, r, &req); err != nil {
				break
			}
			s, rev := m.store.snapshot()
			if req.Revision > rev || req.Revision < m.Host.Snapshot().EditRevision {
				err = feature.ErrConflict
				break
			}
			switch req.Action {
			case "generate":
				if s.DeviceID != "" {
					err = errors.New("E2E secret already exists; reveal it or replace it explicitly")
					break
				}
				s, err = newIdentity()
				if err == nil {
					_, err = m.Host.Commit(r.Context(), rev, feature.CommitOptions{}, func(d *feature.Draft) error {
						var latest Settings
						if e := feature.Decode(d.Settings, &latest); e != nil {
							return e
						}
						s.Sequence = latest.Sequence
						s.Watermarks = latest.Watermarks
						s.Peers = latest.Peers
						if latest.AdminServiceID != "" {
							s.AdminServiceID = latest.AdminServiceID
						}
						b, e := json.Marshal(s)
						d.Settings = b
						return e
					})
				}
			case "reveal":
				if s.GroupToken == "" {
					err = errors.New("relay identity has not been created")
				}
			default:
				err = errors.New("choose generate or reveal")
			}
			if err == nil {
				modulekit.JSON(w, map[string]string{"group_token": s.GroupToken})
				return
			}
		case r.Method == "POST" && r.URL.Path == "/refresh":
			m.mu.Lock()
			c := m.client
			m.mu.Unlock()
			if c != nil {
				c.Refresh()
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
			moduleError(w, code, err)
			return
		}
		modulekit.JSON(w, map[string]bool{"ok": true})
	})
}
func moduleError(w http.ResponseWriter, code int, e error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
}
