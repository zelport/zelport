package remote_http

import (
	"errors"
	"github.com/zelport/zelport/internal/configfile"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"net/http"
)

func (m *Module) serveCAFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "PUT" {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", 405)
		return
	}
	var update configfile.Update
	var err error
	if r.Method == "PUT" {
		update, err = configfile.DecodeUpdate(w, r)
		if err != nil {
			networkAPIError(w, 400, err)
			return
		}
	}
	m.configMu.Lock()
	defer m.configMu.Unlock()
	if r.Context().Err() != nil {
		networkAPIError(w, 503, feature.ErrClosed)
		return
	}
	if m.Host == nil || m.closed {
		networkAPIError(w, 503, feature.ErrClosed)
		return
	}
	s := m.Host.Snapshot()
	c, err := configuration(s.Settings)
	var path string
	if err == nil {
		path, err = resolveCAPath(c.CACertPath)
	}
	if err != nil {
		networkAPIError(w, 400, err)
		return
	}
	var doc configfile.Document
	if r.Method == "GET" {
		doc, err = m.fileEditor.Read(path, string(s.Settings))
	} else {
		if update.Revision < s.EditRevision || update.Revision > s.Revision {
			networkAPIError(w, 409, feature.ErrConflict)
			return
		}
		doc, err = m.fileEditor.Write(r.Context(), path, string(s.Settings), update.Version, update.Content, func(data []byte) error { _, err := parseCABundle(data); return err })
	}
	if err != nil {
		code := 400
		if errors.Is(err, configfile.ErrConflict) {
			code = 409
		} else if errors.Is(err, configfile.ErrUnavailable) {
			code = 503
		}
		networkAPIError(w, code, err)
		return
	}
	modulekit.JSON(w, struct {
		configfile.Document
		Revision uint64 `json:"revision"`
	}{doc, s.Revision})
}
