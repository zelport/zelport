package endpoint

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPathAndAtomicReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG paths and POSIX modes; Windows paths and ACLs have dedicated tests")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, value := range []string{"", "relative"} {
		t.Setenv("XDG_STATE_HOME", value)
		p, err := Path()
		if err != nil || p != filepath.Join(home, ".local/state/zelport/endpoint.json") {
			t.Fatal(p, err)
		}
	}
	t.Setenv("XDG_STATE_HOME", home)
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	for _, port := range []int{8080, 8081} {
		if err := Publish(p, port); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(p)
	if string(b) != "{\"port\":8081}\n" {
		t.Fatal(string(b))
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	if err := Publish(p, 0); err == nil {
		t.Fatal("accepted invalid port")
	}
	b, _ = os.ReadFile(p)
	if string(b) != "{\"port\":8081}\n" {
		t.Fatal("invalid publication changed existing hint")
	}
	files, _ := os.ReadDir(filepath.Dir(p))
	if len(files) != 1 {
		t.Fatal("temporary file leaked")
	}
}

func TestWriteFailureAndSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require Windows privileges")
	}
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	os.WriteFile(blocker, []byte("keep"), 0600)
	if err := Publish(filepath.Join(blocker, "endpoint.json"), 8080); err == nil {
		t.Fatal("expected error")
	}
	link := filepath.Join(dir, "endpoint.json")
	if err := os.Symlink(blocker, link); err != nil {
		t.Fatal(err)
	}
	if err := Publish(link, 8080); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(blocker)
	if string(b) != "keep" {
		t.Fatal("followed destination symlink")
	}
}

func TestPublishOnAllPlatforms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "endpoint.json")
	for _, port := range []int{8080, 8081} {
		if err := Publish(path, port); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "{\"port\":8081}\n" {
		t.Fatal(string(b), err)
	}
}
