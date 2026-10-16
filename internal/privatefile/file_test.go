package privatefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceAndPreserveOnFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.json")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := CreateTemp(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Replace(f.Name(), target); err != nil {
		t.Fatal(err)
	}
	if err := Replace(filepath.Join(dir, "missing"), target); err == nil {
		t.Fatal("missing source accepted")
	}
	b, err := os.ReadFile(target)
	if err != nil || string(b) != "new" {
		t.Fatal(string(b), err)
	}
}
