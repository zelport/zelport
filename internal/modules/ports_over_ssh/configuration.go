package ports_over_ssh

import (
	"encoding/json"

	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/sshpipe"
)

type Configuration struct {
	SSHConfigPath string `json:"ssh_config_path"`
}

func configuration(raw json.RawMessage) (Configuration, error) {
	var value Configuration
	if err := feature.Decode(raw, &value); err != nil {
		return value, err
	}
	_, err := sshpipe.ResolveConfigPath(value.SSHConfigPath)
	return value, err
}
func (m *Module) ValidateSettings(raw json.RawMessage) error {
	_, err := configuration(raw)
	return err
}
func (m *Module) configSnapshot() (Configuration, feature.Snapshot, error) {
	snapshot := feature.Snapshot{Settings: json.RawMessage(`{}`)}
	if m.Host != nil {
		snapshot = m.Host.Snapshot()
	}
	value, err := configuration(snapshot.Settings)
	return value, snapshot, err
}
func (m *Module) selectedConfig() (string, error) {
	value, _, err := m.configSnapshot()
	if err != nil {
		return "", err
	}
	return sshpipe.ResolveConfigPath(value.SSHConfigPath)
}
