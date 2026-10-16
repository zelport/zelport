// Package userpaths resolves per-user configuration and state locations.
package userpaths

import "path/filepath"

func Config() (string, error) {
	base, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.json"), nil
}

func Endpoint() (string, error) {
	base, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "endpoint.json"), nil
}
