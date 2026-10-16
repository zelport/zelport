package userpaths

import (
	"path/filepath"
	"testing"
)

func TestWindowsLocalPaths(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	t.Setenv("APPDATA", filepath.Join(base, "roaming"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "xdg-state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg-config"))
	config, err := Config()
	if err != nil || config != filepath.Join(base, "Zelport", "config.json") {
		t.Fatal(config, err)
	}
	endpoint, err := Endpoint()
	if err != nil || endpoint != filepath.Join(base, "Zelport", "State", "endpoint.json") {
		t.Fatal(endpoint, err)
	}
	t.Setenv("LOCALAPPDATA", "relative")
	if _, err := Config(); err == nil {
		t.Fatal("accepted relative LOCALAPPDATA")
	}
	t.Setenv("LOCALAPPDATA", "")
	config, err = Config()
	if err != nil || !filepath.IsAbs(config) {
		t.Fatal("known folder fallback", config, err)
	}
}
