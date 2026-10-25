package remote_http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
)

func (m *Module) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/ca-file" {
			m.serveCAFile(w, r)
			return
		}
		if r.URL.Path != "/settings" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet {
			modulekit.JSON(w, m.State())
			return
		}
		if r.Method != http.MethodPut {
			w.Header().Set("Allow", "GET, PUT")
			http.Error(w, "method not allowed", 405)
			return
		}
		var request struct {
			Revision           uint64 `json:"revision"`
			CACertPath         string `json:"ca_cert_path"`
			ProxyURL           string `json:"proxy_url"`
			ProxyUsername      string `json:"proxy_username"`
			ProxyPassword      string `json:"proxy_password"`
			ClearProxyPassword bool   `json:"clear_proxy_password"`
		}
		if err := feature.DecodeBody(w, r, &request); err != nil {
			networkAPIError(w, 400, errors.New("Invalid Remote HTTP settings request"))
			return
		}
		m.configMu.Lock()
		defer m.configMu.Unlock()
		if m.Host == nil {
			networkAPIError(w, 503, feature.ErrClosed)
			return
		}
		value := Configuration{CACertPath: request.CACertPath, ProxyURL: request.ProxyURL, ProxyUsername: request.ProxyUsername, ProxyPassword: request.ProxyPassword}
		// Derive password preservation from the current draft under Host.Commit's
		// revision check, so stale edits never change or resurrect credentials.
		result, err := m.Host.Commit(r.Context(), request.Revision, feature.CommitOptions{ResetScopes: []string{"outbound"}}, func(draft *feature.Draft) error {
			previous, err := configuration(draft.Settings)
			if err != nil {
				return err
			}
			if value.ProxyURL == "" {
				value.ProxyUsername = ""
				value.ProxyPassword = ""
			} else if request.ClearProxyPassword {
				value.ProxyPassword = ""
			} else if value.ProxyPassword == "" && previous.ProxyPassword != "" {
				if value.ProxyURL != previous.ProxyURL || value.ProxyUsername != previous.ProxyUsername {
					return errors.New("Re-enter or explicitly clear the stored proxy password when changing the proxy URL or username")
				}
				value.ProxyPassword = previous.ProxyPassword
			}
			raw, _ := json.Marshal(value)
			if err := m.ValidateSettings(raw); err != nil {
				return err
			}
			path, err := resolveCAPath(value.CACertPath)
			if err != nil {
				return err
			}
			if _, err = m.loadRoots(path, value.CACertPath == ""); err != nil {
				return err
			}
			draft.Settings = raw
			return nil
		})
		if err != nil {
			networkAPIError(w, 400, err)
			return
		}
		// Return only the revision; the settings state deliberately excludes secrets.
		modulekit.JSON(w, result)
	})
}
func networkAPIError(w http.ResponseWriter, code int, err error) {
	if errors.Is(err, feature.ErrConflict) {
		code = http.StatusConflict
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
