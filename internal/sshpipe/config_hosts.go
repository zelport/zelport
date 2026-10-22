package sshpipe

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var configuredHostAlias = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,254}$`)

// HostAliases reads explicit SSH Host aliases without running ssh or probing hosts.
// It never reads identity files; an alias in this list is not an authentication check.
func HostAliases(configPath string) ([]string, error) {
	const maxFiles, maxDepth, maxBytes, maxAliases = 64, 8, 1 << 20, 256
	root := filepath.Dir(configPath)
	seenFiles := map[string]bool{}
	seenAliases := map[string]bool{}
	aliases := []string{}
	var read func(string, int) error
	read = func(path string, depth int) error {
		if depth > maxDepth {
			return fmt.Errorf("SSH configuration includes are too deep")
		}
		path, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if seenFiles[path] {
			return nil
		}
		if len(seenFiles) >= maxFiles {
			return fmt.Errorf("too many SSH configuration files")
		}
		file, err := os.Open(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		defer file.Close()
		seenFiles[path] = true
		data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
		if err != nil {
			return err
		}
		if len(data) > maxBytes {
			return fmt.Errorf("SSH configuration file is too large")
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.Replace(line, "=", " ", 1)
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			switch strings.ToLower(parts[0]) {
			case "host":
				for _, alias := range parts[1:] {
					if strings.HasPrefix(alias, "#") {
						break
					}
					if !configuredHostAlias.MatchString(alias) || seenAliases[alias] {
						continue
					}
					if len(aliases) >= maxAliases {
						return fmt.Errorf("too many SSH host aliases")
					}
					seenAliases[alias] = true
					aliases = append(aliases, alias)
				}
			case "include":
				for _, pattern := range parts[1:] {
					if strings.HasPrefix(pattern, "#") {
						break
					}
					pattern = strings.Trim(pattern, `"'`)
					if strings.HasPrefix(pattern, "~/") {
						home, err := os.UserHomeDir()
						if err != nil {
							return err
						}
						pattern = filepath.Join(home, pattern[2:])
					} else if !filepath.IsAbs(pattern) {
						pattern = filepath.Join(root, pattern)
					}
					matches, err := filepath.Glob(pattern)
					if err != nil {
						return err
					}
					for _, match := range matches {
						if err := read(match, depth+1); err != nil {
							return err
						}
					}
				}
			}
		}
		return scanner.Err()
	}
	if err := read(configPath, 0); err != nil {
		return nil, err
	}
	return aliases, nil
}
