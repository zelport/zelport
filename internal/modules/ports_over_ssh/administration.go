package ports_over_ssh

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"github.com/zelport/zelport/internal/sshpipe"
)

func (m *Module) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		switch r.URL.Path {
		case "/config-file":
			if !feature.HostFileAccess(r) {
				sshAPIError(w, http.StatusForbidden, feature.ErrRemoteFileAccess)
				return
			}
			m.serveConfigFile(w, r)
		case "/hosts":
			if r.Method != http.MethodGet {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			m.configMu.RLock()
			path, err := m.selectedConfig()
			m.configMu.RUnlock()
			if err != nil {
				sshAPIError(w, http.StatusServiceUnavailable, errors.New("SSH configuration couldn't be read. Review the file path in Configuration."))
				return
			}
			hosts, err := sshpipe.HostAliases(path)
			if err != nil {
				sshAPIError(w, http.StatusServiceUnavailable, errors.New("SSH hosts couldn't be read. Enter the SSH host manually."))
				return
			}
			modulekit.JSON(w, map[string]any{"hosts": hosts, "config_path": path})
		case "/settings":
			if r.Method == http.MethodGet {
				modulekit.JSON(w, m.State())
				return
			}
			if r.Method != http.MethodPut {
				methodNotAllowed(w, "GET, PUT")
				return
			}
			var request struct {
				Revision      uint64 `json:"revision"`
				SSHConfigPath string `json:"ssh_config_path"`
			}
			if err := feature.DecodeBody(w, r, &request); err != nil {
				sshAPIError(w, 400, err)
				return
			}
			value := Configuration{SSHConfigPath: request.SSHConfigPath}
			raw, _ := json.Marshal(value)
			if err := m.ValidateSettings(raw); err != nil {
				sshAPIError(w, 400, err)
				return
			}
			m.configMu.Lock()
			defer m.configMu.Unlock()
			m.mu.Lock()
			closed := m.closed
			m.mu.Unlock()
			if closed || m.Host == nil {
				sshAPIError(w, 503, feature.ErrClosed)
				return
			}
			previous, _, _ := m.configSnapshot()
			if !feature.HostFileAccess(r) && previous.SSHConfigPath != value.SSHConfigPath {
				sshAPIError(w, http.StatusForbidden, feature.ErrRemoteFileAccess)
				return
			}
			result, err := m.Host.Commit(r.Context(), request.Revision, feature.CommitOptions{}, func(draft *feature.Draft) error { draft.Settings = raw; return nil })
			if err != nil {
				sshAPIError(w, 400, err)
				return
			}
			if previous.SSHConfigPath != value.SSHConfigPath {
				m.retirePools()
			}
			modulekit.JSON(w, result)
		default:
			http.NotFound(w, r)
		}
	})
}
func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
func sshAPIError(w http.ResponseWriter, code int, err error) {
	if errors.Is(err, feature.ErrConflict) {
		code = 409
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
