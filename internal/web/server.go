package web

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
)

//go:embed assets/*
var assets embed.FS

type Server struct {
	manager *app.Manager
	port    int
	token   string
	runtime RuntimeInfo
	files   http.Handler
}

func New(m *app.Manager, port int, info ...RuntimeInfo) (*Server, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	files, e := fs.Sub(assets, "assets")
	if e != nil {
		return nil, e
	}
	return &Server{manager: m, port: port, token: hex.EncodeToString(b), runtime: newRuntimeInfo(info), files: http.FileServer(http.FS(files))}, nil
}
func (s *Server) Close() { s.manager.Close() }
func (s *Server) origin() string {
	if s.port == 80 {
		return "http://localhost"
	}
	return "http://localhost:" + strconv.Itoa(s.port)
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authority := strings.TrimPrefix(s.origin(), "http://")
	if r.Host != authority {
		s.manager.ServeHTTP(w, r)
		return
	}
	s.admin(w, r)
}
func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, e error) {
	code := 400
	name := "invalid_request"
	if errors.Is(e, feature.ErrConflict) {
		code = 409
		name = "conflict"
	}
	if errors.Is(e, feature.ErrNotFound) {
		code = 404
		name = "not_found"
	}
	reply(w, code, map[string]string{"error": e.Error(), "code": name})
}
func (s *Server) admin(w http.ResponseWriter, r *http.Request) {
	s.adminWithContext(w, r, s.origin(), s.token, nil)
}
func (s *Server) adminWithContext(w http.ResponseWriter, r *http.Request, origin, token string, remote *feature.RemoteAdminContext) {
	if r.URL.Path == "/api/integration/v1/state" {
		s.serveIntegration(w, r, remote)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	if r.Method != "GET" && r.Method != "HEAD" {
		if len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != origin || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(token)) != 1 {
			reply(w, 403, map[string]string{"error": "request origin or CSRF token is invalid", "code": "forbidden"})
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/modules/") {
		rest := strings.TrimPrefix(r.URL.Path, "/api/modules/")
		id, _, ok := strings.Cut(rest, "/")
		mod := s.manager.Module(id)
		if !ok || mod == nil {
			http.NotFound(w, r)
			return
		}
		http.StripPrefix("/api/modules/"+id, mod.Admin()).ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/modules/") {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method not allowed", 405)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/assets/modules/")
		id, file, ok := strings.Cut(rest, "/")
		mod := s.manager.Module(id)
		if !ok || mod == nil || file == "" || strings.Contains(file, "..") || strings.HasSuffix(file, "/") || mod.Descriptor().Assets == nil {
			http.NotFound(w, r)
			return
		}
		http.StripPrefix("/assets/modules/"+id+"/", http.FileServer(http.FS(mod.Descriptor().Assets))).ServeHTTP(w, r)
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/state":
		rev, routes := s.manager.Snapshot()
		states := map[string]feature.State{}
		for _, d := range s.manager.Descriptors() {
			st := s.manager.Module(d.ID).State()
			filtered := []feature.RouteView{}
			for _, v := range st.Routes {
				for _, route := range routes {
					if route.ID == v.ID && route.Module == d.ID {
						v.Domain = route.Domain
						if st.Revision != rev {
							v.Summary.State = "stale"
							v.Summary.Detail = "Updating configuration"
						}
						filtered = append(filtered, v)
					}
				}
			}
			st.Routes = filtered
			states[d.ID] = st
		}
		reply(w, 200, map[string]any{"revision": rev, "modules": s.manager.Descriptors(), "module_availability": s.manager.Availability(), "states": states, "runtime": s.runtime.current(), "csrf": token, "port": s.port, "remote_admin": remote})
		return
	case r.Method == "PUT" && r.URL.Path == "/api/module-availability":
		var body struct {
			Revision uint64 `json:"revision"`
			Module   string `json:"module"`
			Enabled  *bool  `json:"enabled"`
		}
		if e := feature.DecodeBody(w, r, &body); e != nil {
			apiError(w, e)
			return
		}
		if body.Enabled == nil {
			apiError(w, errors.New("enabled is required"))
			return
		}
		result, e := s.manager.SetAvailability(r.Context(), body.Revision, body.Module, *body.Enabled)
		if e != nil {
			apiError(w, e)
			return
		}
		reply(w, 200, result)
		return
	case r.Method == "POST" && r.URL.Path == "/api/routes":
		var body struct {
			Revision uint64        `json:"revision"`
			Route    feature.Route `json:"route"`
		}
		if e := feature.DecodeBody(w, r, &body); e != nil {
			apiError(w, e)
			return
		}
		v, e := s.manager.Save(r.Context(), body.Revision, body.Route)
		if e != nil {
			apiError(w, e)
			return
		}
		rev, _ := s.manager.Snapshot()
		reply(w, 201, map[string]any{"route": v, "revision": rev})
		return
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/routes/"):
		rev, e := strconv.ParseUint(r.URL.Query().Get("revision"), 10, 64)
		if e != nil {
			apiError(w, errors.New("expected configuration revision is required"))
			return
		}
		if e = s.manager.Delete(r.Context(), rev, strings.TrimPrefix(r.URL.Path, "/api/routes/")); e != nil {
			apiError(w, e)
			return
		}
		next, _ := s.manager.Snapshot()
		reply(w, 200, map[string]any{"revision": next})
		return
	}
	if (r.Method == "GET" || r.Method == "HEAD") && (r.URL.Path == "/" || r.URL.Path == "/app.js" || r.URL.Path == "/ui.js" || r.URL.Path == "/app.css" || r.URL.Path == "/favicon.svg" || r.URL.Path == "/favicon-dark.svg") {
		s.files.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}
