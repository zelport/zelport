package ports_over_ssh

import (
	"errors"
	"github.com/zelport/zelport/internal/configfile"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"net/http"
)

func (m *Module) serveConfigFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "PUT" {
		methodNotAllowed(w, "GET, PUT")
		return
	}
	var update configfile.Update
	var err error
	if r.Method == "PUT" {
		update, err = configfile.DecodeUpdate(w, r)
		if err != nil {
			sshAPIError(w, 400, err)
			return
		}
	}
	m.configMu.Lock()
	defer m.configMu.Unlock()
	if r.Context().Err() != nil {
		sshAPIError(w, 503, feature.ErrClosed)
		return
	}
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if m.Host == nil || closed {
		sshAPIError(w, 503, feature.ErrClosed)
		return
	}
	_, s, err := m.configSnapshot()
	var path string
	if err == nil {
		path, err = m.selectedConfig()
	}
	if err != nil {
		sshAPIError(w, 400, errors.New("SSH configuration path is unavailable."))
		return
	}
	var doc configfile.Document
	if r.Method == "GET" {
		doc, err = m.fileEditor.Read(path, string(s.Settings))
	} else {
		if update.Revision < s.EditRevision || update.Revision > s.Revision {
			sshAPIError(w, 409, feature.ErrConflict)
			return
		}
		doc, err = m.fileEditor.Write(r.Context(), path, string(s.Settings), update.Version, update.Content, nil)
		if err == nil {
			m.retirePools()
		}
	}
	if err != nil {
		code := 400
		if errors.Is(err, configfile.ErrConflict) {
			code = 409
		} else if errors.Is(err, configfile.ErrUnavailable) {
			code = 503
		}
		sshAPIError(w, code, err)
		return
	}
	modulekit.JSON(w, struct {
		configfile.Document
		Revision uint64 `json:"revision"`
	}{doc, s.Revision})
}
