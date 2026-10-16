package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/privatefile"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type configuration struct {
	ModuleAvailability map[string]bool            `json:"module_availability,omitempty"`
	Version            int                        `json:"version"`
	Revision           uint64                     `json:"revision"`
	Modules            map[string]json.RawMessage `json:"modules"`
	Routes             []feature.Route            `json:"routes"`
}

var domainPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+localhost$`)

func clone[T any](v T) T { b, _ := json.Marshal(v); var c T; json.Unmarshal(b, &c); return c }
func load(path string) (configuration, bool, error) {
	c := configuration{Version: 6, Modules: map[string]json.RawMessage{}, Routes: []feature.Route{}}
	if path == "" {
		return c, false, nil
	}
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return c, true, nil
	}
	if e != nil {
		return c, false, e
	}
	c.Version = 0
	if e = feature.Decode(b, &c); e != nil {
		return c, false, e
	}
	if c.Version != 6 {
		return c, false, fmt.Errorf("unsupported configuration version %d; create a fresh version 6 configuration", c.Version)
	}
	if c.Modules == nil {
		c.Modules = map[string]json.RawMessage{}
	}
	return c, false, nil
}
func persist(path string, c configuration) error {
	return writeConfiguration(path, c, false)
}

func writeConfiguration(path string, c configuration, createOnly bool) error {
	if path == "" {
		return nil
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := privatefile.CreateTemp(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(append(b, '\n')); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if createOnly {
		// Publish a complete file atomically without replacing a concurrently created file.
		return os.Link(f.Name(), path)
	}
	return privatefile.Replace(f.Name(), path)
}
func (m *Manager) validate(c configuration) error {
	for id, b := range c.Modules {
		if !moduleID.MatchString(id) {
			return fmt.Errorf("invalid module ID %q", id)
		}
		mod := m.modules[id]
		if mod == nil {
			continue
		}
		if e := mod.ValidateSettings(b); e != nil {
			return fmt.Errorf("%s settings: %w", id, e)
		}
	}
	ids, domains := map[string]bool{}, map[string]bool{}
	for _, r := range c.Routes {
		mod := m.modules[r.Module]
		if !moduleID.MatchString(r.Module) {
			return fmt.Errorf("invalid module ID %q", r.Module)
		}
		if !feature.ValidUUID(r.ID) {
			return fmt.Errorf("route ID must be UUIDv4")
		}
		if feature.ReservedDomain(r.Domain) {
			return fmt.Errorf("domain %s is reserved for Zelport administration", r.Domain)
		}
		if r.Domain != strings.ToLower(strings.TrimSpace(r.Domain)) || len(r.Domain) > 253 || !domainPattern.MatchString(r.Domain) {
			return fmt.Errorf("use a DNS name such as project.localhost")
		}
		if ids[r.ID] || domains[r.Domain] {
			return feature.ErrConflict
		}
		ids[r.ID] = true
		domains[r.Domain] = true
		if mod != nil {
			if e := mod.ValidateRoute(r); e != nil {
				return e
			}
		}
	}
	return nil
}
