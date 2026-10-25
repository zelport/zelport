package sshpipe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveConfigPath selects the user's SSH configuration without creating it.
// Missing files are valid so a configuration can be selected before editing it.
func ResolveConfigPath(path string) (string, error) {
	if strings.ContainsAny(path, "\x00\r\n") {
		return "", fmt.Errorf("SSH configuration path contains an invalid character")
	}
	if path == "" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "" {
			path = filepath.Join(home, ".ssh", "config")
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return "", fmt.Errorf("SSH configuration path must be an absolute path or start with ~/")
	}
	// OpenSSH before 9.9 treats '%' literally in Include, while newer clients
	// expand percent tokens. No one escaped pathname is literal in both.
	if strings.Contains(path, "%") {
		return "", fmt.Errorf("SSH configuration path cannot contain %% because SSH versions interpret it differently")
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot inspect SSH configuration: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("SSH configuration path must refer to a regular file")
	}
	return path, nil
}
