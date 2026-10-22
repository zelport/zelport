package web

import (
	"github.com/zelport/zelport/internal/feature"
	"net/http"
	"strconv"
	"strings"
)

type integrationService struct {
	ID         string `json:"id"`
	Domain     string `json:"domain"`
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`
	State      string `json:"state"`
	URL        string `json:"url"`
	DataFlow   string `json:"data_flow"`
}
type integrationState struct {
	Product      string                      `json:"product"`
	APIVersion   int                         `json:"api_version"`
	Capabilities []string                    `json:"capabilities"`
	Version      string                      `json:"version"`
	AdminURL     string                      `json:"admin_url"`
	Services     []integrationService        `json:"services"`
	Connections  []feature.ConnectionSummary `json:"connections"`
}

func (s *Server) serveIntegration(w http.ResponseWriter, r *http.Request, remote *feature.RemoteAdminContext) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if remote != nil || r.Host != strings.TrimPrefix(s.origin(), "http://") {
		http.NotFound(w, r)
		return
	}
	for key := range r.Header {
		if strings.EqualFold(key, "Origin") {
			http.Error(w, "origin is not allowed", 403)
			return
		}
	}
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", 405)
		return
	}
	v := integrationState{Product: "zelport", APIVersion: 1, Capabilities: []string{"services", "relay_status"}, Version: s.runtime.Version, AdminURL: s.origin(), Services: []integrationService{}, Connections: []feature.ConnectionSummary{}}
	rev, routes := s.manager.Snapshot()
	availability := s.manager.Availability()
	for _, d := range s.manager.Descriptors() {
		mod := s.manager.Module(d.ID)
		st := mod.State()
		for _, r := range routes {
			if r.Module != d.ID {
				continue
			}
			state := "stale"
			dataFlow := ""
			if st.Revision == rev {
				for _, view := range st.Routes {
					if view.ID == r.ID {
						state = view.Summary.State
						dataFlow = view.Summary.Target
						break
					}
				}
			}
			url := "http://" + r.Domain
			if s.port != 80 {
				url += ":" + strconv.Itoa(s.port)
			}
			v.Services = append(v.Services, integrationService{ID: r.ID, Domain: r.Domain, SourceID: d.ID, SourceName: d.Name, State: state, URL: url, DataFlow: dataFlow})
		}
		a, hasAvailability := availability[d.ID]
		if provider, ok := mod.(feature.ConnectionSummaryProvider); ok && (!hasAvailability || a.Enabled) {
			v.Connections = append(v.Connections, provider.ConnectionSummaries()...)
		}
	}
	reply(w, 200, v)
}
