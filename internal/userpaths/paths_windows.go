package userpaths

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

func configDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
		if err != nil {
			return "", fmt.Errorf("local application data: %w", err)
		}
	}
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("LOCALAPPDATA must be an absolute path")
	}
	return filepath.Join(base, "Zelport"), nil
}

func stateDir() (string, error) {
	base, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "State"), nil
}
