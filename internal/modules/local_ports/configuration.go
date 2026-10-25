package local_ports

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/zelport/zelport/internal/feature"
)

type Configuration struct {
	DiscoveryEnabled bool     `json:"discovery_enabled"`
	IgnoredPorts     []string `json:"ignored_ports"`
	IgnoredProcesses []string `json:"ignored_processes"`
}
type portRange struct{ first, last int }

func parsePortRange(text string) (portRange, error) {
	invalid := errors.New("Ignored ports must be between 1 and 65535 or ascending ranges, for example 8000-8099.")
	parts := strings.Split(text, "-")
	if len(parts) > 2 {
		return portRange{}, invalid
	}
	parse := func(text string) (int, error) {
		text = strings.TrimSpace(text)
		if len(text) < 1 || len(text) > 5 {
			return 0, invalid
		}
		for _, c := range text {
			if c < '0' || c > '9' {
				return 0, invalid
			}
		}
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 || n > 65535 {
			return 0, invalid
		}
		return n, nil
	}
	first, err := parse(parts[0])
	if err != nil {
		return portRange{}, err
	}
	last := first
	if len(parts) == 2 {
		last, err = parse(parts[1])
		if err != nil || last < first {
			return portRange{}, invalid
		}
	}
	return portRange{first, last}, nil
}
func configuration(raw json.RawMessage) (Configuration, error) {
	value := Configuration{DiscoveryEnabled: true, IgnoredPorts: []string{}, IgnoredProcesses: []string{}}
	if err := feature.Decode(raw, &value); err != nil {
		return value, errors.New("Invalid Local Ports settings.")
	}
	if len(value.IgnoredPorts) > 128 || len(value.IgnoredProcesses) > 128 {
		return value, errors.New("Use at most 128 entries in each ignore list.")
	}
	for i, text := range value.IgnoredPorts {
		r, err := parsePortRange(text)
		if err != nil {
			return value, err
		}
		text = strconv.Itoa(r.first)
		if r.first != r.last {
			text += "-" + strconv.Itoa(r.last)
		}
		value.IgnoredPorts[i] = text
	}
	for i, text := range value.IgnoredProcesses {
		text = strings.TrimSpace(text)
		if text == "" || len(text) > 256 || !utf8.ValidString(text) || strings.ContainsFunc(text, unicode.IsControl) {
			return value, errors.New("Use process names of up to 256 bytes without control characters.")
		}
		value.IgnoredProcesses[i] = text
	}
	if value.IgnoredPorts == nil {
		value.IgnoredPorts = []string{}
	}
	if value.IgnoredProcesses == nil {
		value.IgnoredProcesses = []string{}
	}
	return value, nil
}
func (m *Module) ValidateSettings(raw json.RawMessage) error {
	_, err := configuration(raw)
	return err
}
func (m *Module) configuration() (Configuration, error) {
	raw := json.RawMessage(`{}`)
	if m.Host != nil {
		raw = m.Host.Snapshot().Settings
	}
	return configuration(raw)
}
func (m *Module) State() feature.State {
	m.configMu.RLock()
	defer m.configMu.RUnlock()
	state := m.Base.State()
	value, _ := m.configuration()
	state.Data, _ = json.Marshal(value)
	return state
}
func (c Configuration) exclude(listener Listener) bool {
	for _, text := range c.IgnoredPorts {
		r, _ := parsePortRange(text)
		if listener.Port >= r.first && listener.Port <= r.last {
			return true
		}
	}
	if listener.unknownOwner || len(listener.Processes) == 0 {
		return false
	}
	for _, process := range listener.Processes {
		ignored := false
		for _, name := range c.IgnoredProcesses {
			if process.Name != "" && strings.EqualFold(process.Name, name) {
				ignored = true
				break
			}
		}
		if !ignored {
			return false
		}
	}
	return true
}
