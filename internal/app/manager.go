package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/feature"
	"io"
	"net"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type tracked struct {
	c     io.Closer
	scope string
}
type active struct {
	lease  *feature.Lease
	scopes []string
}
type Manager struct {
	administration func(http.ResponseWriter, *http.Request, feature.RemoteAdminContext)
	op             sync.Mutex
	mu             sync.RWMutex
	path           string
	port           int
	config         configuration
	editRevision   uint64
	modules        map[string]feature.Module
	order          []feature.Module
	started        []feature.Module
	closed         bool
	cancel         context.CancelFunc
	watchers       map[chan struct{}]bool
	scopes         map[string]uint64
	resources      map[uint64]tracked
	leases         map[uint64]active
	next           uint64
	generations    map[string]uint64
	middleware     map[string]func(http.Handler) http.Handler
}

var moduleID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func New(path string, port int, mods ...feature.Module) (*Manager, error) {
	m := &Manager{path: path, port: port, modules: map[string]feature.Module{}, watchers: map[chan struct{}]bool{}, scopes: map[string]uint64{}, resources: map[uint64]tracked{}, leases: map[uint64]active{}, generations: map[string]uint64{}, middleware: map[string]func(http.Handler) http.Handler{}}
	tabs := map[string]bool{}
	for _, mod := range mods {
		d := mod.Descriptor()
		validUI := (d.UIPlacement == "" && moduleID.MatchString(d.TabID) && !tabs[d.TabID] && d.TabID != "dashboard" && d.TabID != "settings") || (d.UIPlacement == "settings" && d.TabID == "")
		if (d.Maturity != feature.Stable && d.Maturity != feature.Experimental) || !moduleID.MatchString(d.ID) || !validUI || m.modules[d.ID] != nil {
			return nil, fmt.Errorf("invalid or duplicate module descriptor: %s", d.ID)
		}
		if d.TabID != "" {
			tabs[d.TabID] = true
		}
		m.modules[d.ID] = mod
		m.order = append(m.order, mod)
	}
	sort.Slice(m.order, func(i, j int) bool {
		a, b := m.order[i].Descriptor(), m.order[j].Descriptor()
		if a.Order == b.Order {
			return a.ID < b.ID
		}
		return a.Order < b.Order
	})
	var e error
	var missing bool
	m.config, missing, e = load(path)
	if e != nil {
		return nil, e
	}
	m.retainUsedAvailability(&m.config)
	m.editRevision = m.config.Revision
	for _, mod := range m.order {
		if e = mod.ValidateSettings(m.settings(mod.Descriptor().ID)); e != nil {
			return nil, e
		}
	}
	if e = m.validate(m.config); e != nil {
		return nil, e
	}
	if missing {
		if e = writeConfiguration(path, m.config, true); e != nil {
			return nil, fmt.Errorf("create initial configuration: %w", e)
		}
	}
	for _, r := range m.config.Routes {
		m.generations[r.ID] = 1
	}
	return m, nil
}
func (m *Manager) settings(id string) json.RawMessage {
	b := m.config.Modules[id]
	if b == nil {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), b...)
}
func (m *Manager) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	for _, mod := range m.order {
		m.started = append(m.started, mod)
		if e := mod.Start(ctx, &host{m: m, id: mod.Descriptor().ID}); e != nil {
			m.Close()
			return e
		}
	}
	return nil
}
func (m *Manager) Close() error {
	m.op.Lock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.op.Unlock()
		return nil
	}
	m.closed = true
	var closers []io.Closer
	for _, l := range m.leases {
		closers = append(closers, l.lease)
	}
	for _, r := range m.resources {
		closers = append(closers, r.c)
	}
	for ch := range m.watchers {
		close(ch)
		delete(m.watchers, ch)
	}
	m.mu.Unlock()
	m.op.Unlock()
	for _, c := range closers {
		c.Close()
	}
	if m.cancel != nil {
		m.cancel()
	}
	for i := len(m.started) - 1; i >= 0; i-- {
		m.started[i].Close()
	}
	return nil
}
func (m *Manager) Descriptors() []feature.Descriptor {
	v := []feature.Descriptor{}
	for _, mod := range m.order {
		v = append(v, mod.Descriptor())
	}
	return v
}
func (m *Manager) Module(id string) feature.Module { return m.modules[id] }
func (m *Manager) Snapshot() (uint64, []feature.Route) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Revision, clone(m.config.Routes)
}
func (m *Manager) Save(ctx context.Context, rev uint64, r feature.Route) (feature.Route, error) {
	if m.modules[r.Module] == nil {
		return r, fmt.Errorf("module %q is not included in this build", r.Module)
	}
	r.Domain = strings.ToLower(strings.TrimSpace(r.Domain))
	create := r.ID == ""
	if create {
		var e error
		r.ID, e = feature.UUID()
		if e != nil {
			return r, e
		}
	}
	_, e := m.commit(ctx, rev, nil, false, func(c *configuration) error {
		found := false
		for i, old := range c.Routes {
			if old.ID == r.ID {
				found = true
				if create || old.Module != r.Module {
					return fmt.Errorf("route owner cannot change")
				}
				c.Routes[i] = r
			}
		}
		if !found {
			if !create {
				return feature.ErrNotFound
			}
			c.Routes = append(c.Routes, r)
		}
		return nil
	})
	return r, e
}
func (m *Manager) Delete(ctx context.Context, rev uint64, id string) error {
	_, e := m.commit(ctx, rev, nil, false, func(c *configuration) error {
		for i, r := range c.Routes {
			if r.ID == id {
				c.Routes = append(c.Routes[:i], c.Routes[i+1:]...)
				return nil
			}
		}
		return feature.ErrNotFound
	})
	return e
}
func (m *Manager) commit(ctx context.Context, rev uint64, resets []string, metadata bool, mutate func(*configuration) error) (feature.CommitResult, error) {
	m.op.Lock()
	defer m.op.Unlock()
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return feature.CommitResult{}, feature.ErrClosed
	}
	old := clone(m.config)
	m.mu.RUnlock()
	if rev > old.Revision || rev < m.editRevision || (metadata && rev != old.Revision) {
		return feature.CommitResult{}, feature.ErrConflict
	}
	if e := ctx.Err(); e != nil {
		return feature.CommitResult{}, e
	}
	next := clone(old)
	if e := mutate(&next); e != nil {
		return feature.CommitResult{}, e
	}
	if !metadata {
		m.retainUsedAvailability(&next)
	}
	if e := m.validate(next); e != nil {
		return feature.CommitResult{}, e
	}
	if metadata && (len(resets) != 0 || !reflect.DeepEqual(old.Routes, next.Routes)) {
		return feature.CommitResult{}, errors.New("metadata commit cannot change routes or reset scopes")
	}
	next.Revision++
	if e := persist(m.path, next); e != nil {
		return feature.CommitResult{}, e
	}
	changed := map[string]bool{}
	for _, r := range old.Routes {
		same := false
		for _, n := range next.Routes {
			if reflect.DeepEqual(r, n) {
				same = true
				break
			}
		}
		if !same {
			changed[r.ID] = true
		}
	}
	reset := map[string]bool{}
	for _, s := range resets {
		reset[s] = true
	}
	var closers []io.Closer
	m.mu.Lock()
	m.config = next
	if !metadata {
		m.editRevision = next.Revision
	}
	for _, r := range next.Routes {
		if m.generations[r.ID] == 0 {
			m.generations[r.ID] = 1
		}
	}
	for id := range changed {
		m.generations[id]++
	}
	for s := range reset {
		m.scopes[s]++
	}
	for _, a := range m.leases {
		closeIt := changed[a.lease.Route().ID]
		for _, s := range a.scopes {
			closeIt = closeIt || reset[s]
		}
		if closeIt {
			closers = append(closers, a.lease)
		}
	}
	for id, r := range m.resources {
		if reset[r.scope] {
			closers = append(closers, r.c)
			delete(m.resources, id)
		}
	}
	for ch := range m.watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	m.mu.Unlock()
	for _, c := range closers {
		c.Close()
	}
	return feature.CommitResult{Revision: next.Revision}, nil
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
		return nil, fmt.Errorf("module %q is not included in this build; configuration is preserved", route.Module)
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
				feature.Status(w, r, 503, "Service unavailable", e.Error())
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

type host struct {
	m  *Manager
	id string
}

func (h *host) Snapshot() feature.Snapshot {
	h.m.mu.RLock()
	defer h.m.mu.RUnlock()
	s := feature.Snapshot{EditRevision: h.m.editRevision, Revision: h.m.config.Revision, Routes: []feature.Route{}, Settings: h.m.settings(h.id)}
	for _, r := range h.m.config.Routes {
		if r.Module == h.id {
			s.Routes = append(s.Routes, clone(r))
		}
	}
	return s
}
func (h *host) OwnServices() []feature.Route {
	_, rs := h.m.Snapshot()
	v := []feature.Route{}
	for _, r := range rs {
		if mod := h.m.modules[r.Module]; mod != nil && mod.Descriptor().Exportable {
			r.Settings = nil
			v = append(v, r)
		}
	}
	return v
}
func (h *host) GatewayAuthority(domain string) string {
	if h.m.port == 80 {
		return domain
	}
	return net.JoinHostPort(domain, strconv.Itoa(h.m.port))
}
func (h *host) Watch() (<-chan struct{}, func()) {
	h.m.mu.Lock()
	ch := make(chan struct{}, 1)
	if h.m.closed {
		close(ch)
	} else {
		h.m.watchers[ch] = true
	}
	h.m.mu.Unlock()
	return ch, func() {
		h.m.mu.Lock()
		if h.m.watchers[ch] {
			delete(h.m.watchers, ch)
			close(ch)
		}
		h.m.mu.Unlock()
	}
}
func (h *host) Commit(ctx context.Context, rev uint64, opt feature.CommitOptions, mutate func(*feature.Draft) error) (feature.CommitResult, error) {
	var resets []string
	for _, s := range opt.ResetScopes {
		if !moduleID.MatchString(s) {
			return feature.CommitResult{}, errors.New("invalid scope")
		}
		resets = append(resets, h.id+"/"+s)
	}
	return h.m.commit(ctx, rev, resets, opt.MetadataOnly, func(c *configuration) error {
		if opt.StrictRevision && c.Revision != rev {
			return feature.ErrConflict
		}
		d := feature.Draft{Routes: []feature.Route{}, Settings: json.RawMessage(`{}`)}
		if b := c.Modules[h.id]; b != nil {
			d.Settings = b
		}
		for _, r := range c.Routes {
			if r.Module == h.id {
				d.Routes = append(d.Routes, r)
			}
		}
		if e := mutate(&d); e != nil {
			return e
		}
		others := []feature.Route{}
		for _, r := range c.Routes {
			if r.Module != h.id {
				others = append(others, r)
			}
		}
		for _, r := range d.Routes {
			if r.Module != h.id {
				return errors.New("module cannot modify another module's routes")
			}
		}
		if opt.MetadataOnly {
			original := []feature.Route{}
			for _, r := range c.Routes {
				if r.Module == h.id {
					original = append(original, r)
				}
			}
			if !reflect.DeepEqual(original, d.Routes) {
				return errors.New("metadata commit cannot change routes")
			}
		} else {
			c.Routes = append(others, d.Routes...)
		}
		c.Modules[h.id] = d.Settings
		return nil
	})
}
func (h *host) Scope(s string) feature.Scope {
	h.m.mu.RLock()
	defer h.m.mu.RUnlock()
	return feature.Scope{Name: h.id + "/" + s, Generation: h.m.scopes[h.id+"/"+s]}
}
func (h *host) Acquire(ctx context.Context, id string, rev uint64, ss ...feature.Scope) (*feature.Lease, error) {
	for _, s := range ss {
		if !strings.HasPrefix(s.Name, h.id+"/") {
			return nil, errors.New("invalid scope owner")
		}
	}
	return h.m.acquire(ctx, id, &rev, true, ss)
}
func (h *host) Track(rev uint64, s feature.Scope, c io.Closer) (func(), error) {
	h.m.mu.Lock()
	if h.m.closed || rev != h.m.config.Revision || !strings.HasPrefix(s.Name, h.id+"/") || h.m.scopes[s.Name] != s.Generation {
		h.m.mu.Unlock()
		c.Close()
		return func() {}, feature.ErrClosed
	}
	h.m.next++
	n := h.m.next
	h.m.resources[n] = tracked{c, s.Name}
	h.m.mu.Unlock()
	return func() { h.m.mu.Lock(); delete(h.m.resources, n); h.m.mu.Unlock() }, nil
}
func (h *host) Dispatch(w http.ResponseWriter, r *http.Request, l *feature.Lease) {
	h.m.dispatch(w, r, l, false)
}
func (h *host) AddMiddleware(wrap func(http.Handler) http.Handler) func() {
	h.m.mu.Lock()
	h.m.middleware[h.id] = wrap
	h.m.mu.Unlock()
	return func() { h.m.mu.Lock(); delete(h.m.middleware, h.id); h.m.mu.Unlock() }
}
