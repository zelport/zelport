package sshpipe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	config := filepath.Join(home, "custom config")
	if err := os.WriteFile(config, []byte("Host fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, path, want string }{
		{"default", "", filepath.Join(home, ".ssh", "config")},
		{"tilde", "~/custom config", config},
		{"absolute", config, config},
		{"missing", filepath.Join(home, "missing", "config"), filepath.Join(home, "missing", "config")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveConfigPath(tc.path)
			if err != nil || got != tc.want {
				t.Fatalf("ResolveConfigPath(%q) = %q, %v; want %q", tc.path, got, err, tc.want)
			}
		})
	}
	for _, path := range []string{"relative/config", "~another/config", home, config + "\x00", config + "\nHost injected", config + "\r"} {
		if got, err := ResolveConfigPath(path); err == nil {
			t.Errorf("ResolveConfigPath(%q) = %q; want error", path, got)
		}
	}
	if _, err := ResolveConfigPath(filepath.Join(home, "config %h")); err == nil {
		t.Fatal("percent token accepted despite incompatible OpenSSH Include versions")
	}
}

func TestResolveConfigPathRejectsNonregularFile(t *testing.T) {
	// A symlink may target a regular file, but a filesystem socket cannot be
	// an SSH configuration file. This fixture does not require Unix syscalls.
	if _, err := ResolveConfigPath(os.DevNull); err == nil {
		t.Fatal("device accepted as an SSH configuration file")
	}
}
