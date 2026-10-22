package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/zelport/zelport/internal/feature"
)

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
