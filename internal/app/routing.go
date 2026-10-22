package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/zelport/zelport/internal/feature"
)

func (m *Manager) Snapshot() (uint64, []feature.Route) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Revision, clone(m.config.Routes)
}

func (m *Manager) acquire(ctx context.Context, id string, rev *uint64, export bool, scopes []feature.Scope) (*feature.Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, feature.ErrClosed
	}
	if rev != nil && *rev != m.config.Revision {
		return nil, feature.ErrConflict
	}
	var route feature.Route
	for _, r := range m.config.Routes {
		if r.ID == id {
			route = r
			break
		}
	}
	if route.ID == "" {
		return nil, feature.ErrNotFound
	}
	if m.modules[route.Module] == nil {
		return nil, fmt.Errorf("Module %q is not included in this build. Its configuration is preserved.", route.Module)
	}
	if export && !m.modules[route.Module].Descriptor().Exportable {
		return nil, feature.ErrNotFound
	}
	keys := []string{}
	for _, s := range scopes {
		if m.scopes[s.Name] != s.Generation {
			return nil, feature.ErrClosed
		}
		keys = append(keys, s.Name)
	}
	m.next++
	n := m.next
	l := feature.NewLease(ctx, clone(route), m.generations[id], func() { m.mu.Lock(); delete(m.leases, n); m.mu.Unlock() })
	m.leases[n] = active{l, keys}
	return l, nil
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, e := net.SplitHostPort(host); e == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if feature.ReservedDomain(host) {
		for _, mod := range m.order {
			if provider, ok := mod.(feature.AdministrationProvider); ok && provider.ServeAdministration(w, r) {
				return
			}
		}
		feature.Status(w, r, 404, "Unknown address", "This administration address is not known.")
		return
	}
	revision, routes := m.Snapshot()
	for _, v := range routes {
		if v.Domain == host {
			l, e := m.acquire(r.Context(), v.ID, &revision, false, nil)
			if e != nil {
				feature.StatusDiagnostic(w, r, 503, "Service unavailable", "The service is temporarily unavailable.", e.Error())
				return
			}
			m.dispatch(w, r, l, true)
			return
		}
	}
	feature.Status(w, r, 404, "Unknown address", "This domain is not configured.")
}

func (m *Manager) dispatch(w http.ResponseWriter, r *http.Request, l *feature.Lease, release bool) {
	if l.Context().Err() != nil {
		feature.Status(w, r, 503, "Service unavailable", "The route has changed.")
		return
	}
	v := l.Route()
	rw := &feature.Writer{ResponseWriter: w, Lease: l}
	defer func() {
		if release && !rw.Hijacked {
			l.Close()
		}
	}()
	r = r.WithContext(feature.WithLease(l.Context(), l))
	handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { m.modules[v.Module].ServeRoute(w, r, v) }))
	m.mu.RLock()
	for i := len(m.order) - 1; i >= 0; i-- {
		if wrap := m.middleware[m.order[i].Descriptor().ID]; wrap != nil {
			handler = wrap(handler)
		}
	}
	m.mu.RUnlock()
	handler.ServeHTTP(rw, r)
}
