package relay

import "github.com/zelport/zelport/internal/feature"

// ConnectionSummaries exposes the same visibility-independent state as the header.
func (m *Module) ConnectionSummaries() []feature.ConnectionSummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings, _ := settings(m.Host.Snapshot().Settings)
	state := "stopped"
	if settings.Enabled {
		if m.failure != "" {
			state = "error"
		} else if m.runtime != nil {
			switch m.runtime.Status().State {
			case "Listening":
				state = "listening"
			case "Error":
				state = "error"
			case "Stopped":
				state = "stopped"
			default:
				state = "starting"
			}
		} else {
			state = "starting"
		}
	}
	return []feature.ConnectionSummary{{ID: "built_in_relay", Label: "Built-in Relay", State: state}}
}
