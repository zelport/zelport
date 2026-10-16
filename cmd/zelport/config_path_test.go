package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveConfigPath(t *testing.T) {
	for _, explicit := range []string{"custom.json", filepath.Join(t.TempDir(), "config.json")} {
		got, err := resolveConfigPath(explicit)
		if err != nil || got != explicit {
			t.Fatalf("explicit path = %q, %v; want %q", got, err, explicit)
		}
	}
	if runtime.GOOS == "windows" {
		base := t.TempDir()
		t.Setenv("LOCALAPPDATA", base)
		got, err := resolveConfigPath("")
		if err != nil || got != filepath.Join(base, "Zelport", "config.json") {
			t.Fatal(got, err)
		}
		return
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		dir, err := os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		got, err := resolveConfigPath("")
		if err != nil || got != filepath.Join(dir, "zelport", "config.json") {
			t.Fatalf("native path = %q, %v", got, err)
		}
		return
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, xdg := range []string{"", "relative/config", filepath.Join(t.TempDir(), "configuration")} {
		t.Run(xdg, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", xdg)
			dir := filepath.Join(home, ".config")
			if filepath.IsAbs(xdg) {
				dir = xdg
			}
			got, err := resolveConfigPath("")
			if err != nil || got != filepath.Join(dir, "zelport", "config.json") {
				t.Fatalf("XDG path = %q, %v; base %q", got, err, dir)
			}
		})
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if _, err := resolveConfigPath(""); err == nil {
		t.Fatal("missing home must fail instead of selecting a relative path")
	}
}
