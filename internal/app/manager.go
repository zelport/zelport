package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"sync"

	"github.com/zelport/zelport/internal/feature"
)

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
