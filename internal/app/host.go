package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/zelport/zelport/internal/feature"
)

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

func (h *host) ListenerAuthority(domain string) string {
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
