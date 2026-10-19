package web

import (
	"path/filepath"
	"testing"
)

func TestDisplayConfigPath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "user")
	for _, tc := range []struct{ name, path, home, want string }{
		{"inside", filepath.Join(home, ".config", "zelport", "config.json"), home, filepath.Join("~", ".config", "zelport", "config.json")},
		{"home", home, home, "~"},
		{"similar prefix", home + "-other/config.json", home, home + "-other/config.json"},
		{"parent escape", home + "/../config.json", home, home + "/../config.json"},
		{"unknown home", home, "", home},
		{"relative", "config.json", home, "config.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := displayConfigPath(tc.path, tc.home); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRuntimeHostname(t *testing.T) {
	if got := newRuntimeInfo([]RuntimeInfo{{Hostname: "test-host"}}).Hostname; got != "test-host" {
		t.Fatalf("hostname = %q, want test-host", got)
	}
	if got := newRuntimeInfo(nil).Hostname; got == "" {
		t.Fatal("default hostname is empty")
	}
}
