package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRejectsEveryVersionExceptOneWithoutRewriting(t *testing.T) {
	for _, body := range []string{
		`null`,
		`{}`,
		`{"revision":1,"modules":{},"routes":[]}`,
		`{"version":0,"revision":1,"modules":{},"routes":[]}`,
		`{"version":2,"revision":1,"modules":{},"routes":[]}`,
		`{"version":3,"revision":1,"modules":{},"routes":[]}`,
		`{"version":6,"revision":1,"modules":{},"routes":[]}`,
	} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			before := []byte(body)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := New(path, 8080); err == nil {
				t.Fatal("accepted unsupported configuration")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("rejected configuration was rewritten")
			}
		})
	}
}

func TestConfigLoadsExplicitVersionOneWithoutRewriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	before := []byte("{\"version\":1,\"revision\":42,\"modules\":{},\"routes\":[]}\n")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path, 8080); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("existing configuration was rewritten")
	}
}

func TestNewCreatesDefaultConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if _, err := New(path, 8080); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("configuration permissions: %v", info.Mode())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg configuration
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 1 || cfg.Revision != 0 || cfg.Modules == nil || cfg.Routes == nil || len(cfg.Modules) != 0 || len(cfg.Routes) != 0 {
		t.Fatalf("invalid defaults: %s", body)
	}
}

func TestNewPreservesExistingConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := []byte("{\"version\":1,\"revision\":42,\"modules\":{},\"routes\":[]}\n")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path, 8080); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatal("existing configuration rewritten")
	}
}

func TestNewFailsWhenDefaultConfigurationCannotBeWritten(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	if _, err := New(filepath.Join(dir, "config.json"), 8080); err == nil {
		t.Fatal("startup accepted an unwritable configuration directory")
	}
}

func TestInitialWriteDoesNotReplaceConcurrentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := []byte("existing content")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeConfiguration(path, configuration{Version: 1}, true); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected exclusive creation error, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatal("concurrent file overwritten")
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatal("temporary file leaked")
	}
}
