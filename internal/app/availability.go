package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zelport/zelport/internal/feature"
)

// Availability describes navigation only; module execution and services are unchanged.
type Availability struct {
	Enabled    bool   `json:"enabled"`
	CanDisable bool   `json:"can_disable"`
	Services   int    `json:"services"`
	Reason     string `json:"reason,omitempty"`
}

func (m *Manager) availability(c configuration, mod feature.Module) Availability {
	d := mod.Descriptor()
	a := Availability{Enabled: d.Maturity == feature.Stable, CanDisable: true}
	if enabled, ok := c.ModuleAvailability[d.ID]; ok {
		a.Enabled = enabled
	}
	for _, r := range c.Routes {
		if r.Module == d.ID {
			a.Services++
		}
	}
	if a.Services > 0 {
		noun := "services"
		if a.Services == 1 {
			noun = "service"
		}
		a.Reason = fmt.Sprintf("Remove %d configured %s before disabling this module.", a.Services, noun)
	}
	if guard, ok := mod.(feature.AvailabilityGuard); ok {
		raw := c.Modules[d.ID]
		if raw == nil {
			raw = json.RawMessage(`{}`)
		}
		if reason := guard.UnavailableReason(raw); reason != "" {
			if a.Reason != "" {
				a.Reason += " "
			}
			a.Reason += reason
		}
	}
	if a.Reason != "" {
		a.CanDisable = false
		a.Enabled = true
	}
	return a
}

func (m *Manager) Availability() map[string]Availability {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := map[string]Availability{}
	for _, mod := range m.order {
		if mod.Descriptor().UIPlacement == "" {
			result[mod.Descriptor().ID] = m.availability(m.config, mod)
		}
	}
	return result
}

func (m *Manager) SetAvailability(ctx context.Context, revision uint64, id string, enabled bool) (feature.CommitResult, error) {
	return m.commit(ctx, revision, nil, false, func(c *configuration) error {
		mod := m.modules[id]
		if mod == nil || mod.Descriptor().UIPlacement != "" {
			return feature.ErrNotFound
		}
		a := m.availability(*c, mod)
		if !enabled && !a.CanDisable {
			return errors.New(a.Reason)
		}
		if c.ModuleAvailability == nil {
			c.ModuleAvailability = map[string]bool{}
		}
		c.ModuleAvailability[id] = enabled
		return nil
	})
}

// Once a module is used, retain its interface even after its last service is removed.
func (m *Manager) retainUsedAvailability(c *configuration) {
	for _, mod := range m.order {
		if mod.Descriptor().UIPlacement != "" {
			continue
		}
		if !m.availability(*c, mod).CanDisable {
			if c.ModuleAvailability == nil {
				c.ModuleAvailability = map[string]bool{}
			}
			c.ModuleAvailability[mod.Descriptor().ID] = true
		}
	}
}
