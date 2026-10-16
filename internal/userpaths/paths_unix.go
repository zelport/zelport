//go:build !windows

package userpaths

import (
	"os"
	"path/filepath"
	"runtime"
)

func xdgDir(variable, fallback string) (string, error) {
	base := os.Getenv(variable)
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(base, "zelport"), nil
}

func configDir() (string, error) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		return xdgDir("XDG_CONFIG_HOME", ".config")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "zelport"), nil
}

func stateDir() (string, error) { return xdgDir("XDG_STATE_HOME", ".local/state") }
