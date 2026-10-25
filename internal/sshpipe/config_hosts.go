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
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	// OpenSSH resolves relative user Include paths from ~/.ssh, including
	// when the selected file (or an included file) is in another directory.
	root := filepath.Join(home, ".ssh")
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
			// Only the directive separator may be '='; filenames may contain it.
			if end := strings.IndexAny(line, " \t="); end >= 0 && line[end] == '=' {
				line = line[:end] + " " + line[end+1:]
			} else if end >= 0 {
				value := strings.TrimLeft(line[end:], " \t")
				if strings.HasPrefix(value, "=") {
					line = line[:end] + " " + value[1:]
				}
			}
			parts := configFields(line)
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
					if strings.HasPrefix(pattern, "~/") {
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

// configFields preserves quoted pathnames rather than splitting them at spaces.
func configFields(line string) []string {
	var fields []string
	var field strings.Builder
	var quote rune
	escaped := false
	for _, char := range line {
		if escaped {
			// Preserve glob escapes for filepath.Glob after removing config
			// quoting; otherwise a literal wildcard can include other files.
			if strings.ContainsRune(`*?[]`, char) {
				field.WriteRune('\\')
			}
			field.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				field.WriteRune(char)
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '#':
			if field.Len() == 0 {
				return fields
			}
			field.WriteRune(char)
		case ' ', '\t':
			if field.Len() > 0 {
				fields = append(fields, field.String())
				field.Reset()
			}
		default:
			field.WriteRune(char)
		}
	}
	if field.Len() > 0 {
		fields = append(fields, field.String())
	}
	return fields
}
