package app

import (
	"github.com/zelport/zelport/internal/feature"
	"sort"
)

// UnsupportedModules lists configured modules absent from this build. Their raw
// settings and routes stay in configuration but are never executed or exported.
func (m *Manager) UnsupportedModules() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	missing := map[string]bool{}
	for id := range m.config.Modules {
		if m.modules[id] == nil {
			missing[id] = true
		}
	}
	for _, r := range m.config.Routes {
		if m.modules[r.Module] == nil {
			missing[r.Module] = true
		}
	}
	ids := make([]string, 0, len(missing))
	for id := range missing {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// UnsupportedStates exposes only generic identity and status, never opaque settings.
func (m *Manager) UnsupportedStates(revision uint64, routes []feature.Route) map[string]feature.State {
	states := map[string]feature.State{}
	for _, r := range routes {
		if m.modules[r.Module] != nil {
			continue
		}
		state := states[r.Module]
		state.Revision = revision
		state.Routes = append(state.Routes, feature.RouteView{ID: r.ID, Module: r.Module, Domain: r.Domain, Summary: feature.Summary{State: "unsupported", Detail: "Module is not included in this build. Configuration is preserved."}})
		states[r.Module] = state
	}
	return states
}
