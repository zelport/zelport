package configfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMissingFileIsCreatedOnlyBySave(t *testing.T) {
	var editor Editor
	path := filepath.Join(t.TempDir(), "new", "config")
	doc, err := editor.Read(path, "settings")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "" || doc.Path != path || doc.Version == "" {
		t.Fatalf("document: %+v", doc)
	}
	if _, err = os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("read created parent: %v", err)
	}
	saved, err := editor.Write(context.Background(), path, "settings", doc.Version, "Host example\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Content != "Host example\n" || saved.Version == doc.Version {
		t.Fatalf("save: %+v", saved)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "Host example\n" {
		t.Fatalf("bytes %q", data)
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	if _, err = editor.Write(context.Background(), path, "settings", doc.Version, "overwrite", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("old version: %v", err)
	}
}

func TestConflictsAndValidationPreserveOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	os.WriteFile(path, []byte("original"), 0640)
	var editor Editor
	doc, err := editor.Read(path, "saved")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, binding, version, content string
		validate                              func([]byte) error
	}{
		{"path", path + "other", "saved", doc.Version, "new", nil},
		{"settings", path, "changed", doc.Version, "new", nil},
		{"token", path, "saved", "unknown", "new", nil},
		{"limit", path, "saved", doc.Version, strings.Repeat("x", MaxSize+1), nil},
		{"binary", path, "saved", doc.Version, "bad\x00", nil},
		{"utf8", path, "saved", doc.Version, string([]byte{255}), nil},
		{"validation", path, "saved", doc.Version, "new", func([]byte) error { return errors.New("invalid") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := editor.Write(context.Background(), tc.path, tc.binding, tc.version, tc.content, tc.validate); err == nil {
				t.Fatal("accepted")
			}
			data, _ := os.ReadFile(path)
			if string(data) != "original" {
				t.Fatalf("changed %q", data)
			}
		})
	}
	os.WriteFile(path, []byte("external"), 0640)
	if _, err = editor.Write(context.Background(), path, "saved", doc.Version, "new", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("external: %v", err)
	}
	doc, err = editor.Read(path, "saved")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = editor.Write(context.Background(), path, "saved", doc.Version, "saved", nil); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatal(info.Mode())
	}
}

func TestSymlinkReplacementAndFailedWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privilege")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	os.WriteFile(target, []byte("original"), 0600)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	var editor Editor
	doc, err := editor.Read(link, "saved")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = editor.Write(context.Background(), link, "saved", doc.Version, "new", nil); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(link); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link replaced")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "new" {
		t.Fatalf("target: %q", data)
	}
	doc, _ = editor.Read(link, "saved")
	os.Remove(link)
	os.Symlink(filepath.Join(dir, "missing"), link)
	if _, err = editor.Write(context.Background(), link, "saved", doc.Version, "bad", nil); err == nil {
		t.Fatal("replaced link accepted")
	}
	if _, err = editor.Read(link, "saved"); err == nil {
		t.Fatal("dangling link accepted")
	}
	if _, err = editor.Read(dir, "saved"); err == nil {
		t.Fatal("directory accepted")
	}
	doc, _ = editor.Read(target, "saved")
	if _, err = editor.Write(context.Background(), target, "saved", doc.Version, "bad", func([]byte) error { os.Chmod(dir, 0500); return nil }); err == nil {
		t.Fatal("unwritable directory accepted")
	}
	os.Chmod(dir, 0700)
	data, _ = os.ReadFile(target)
	if string(data) != "new" {
		t.Fatalf("failed write changed %q", data)
	}
}

func TestReplacedSameBytesFileConflicts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte("same"), 0600)
	var editor Editor
	doc, err := editor.Read(path, "saved")
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	other := filepath.Join(dir, "other")
	os.WriteFile(other, []byte("same"), 0600)
	os.Chtimes(other, info.ModTime(), info.ModTime())
	os.Rename(other, path)
	if _, err = editor.Write(context.Background(), path, "saved", doc.Version, "new", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("replaced inode: %v", err)
	}
}

func TestCancellationBeforePublishPreservesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	os.WriteFile(path, []byte("original"), 0600)
	var editor Editor
	doc, err := editor.Read(path, "settings")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err = editor.Write(ctx, path, "settings", doc.Version, "new", func([]byte) error { cancel(); return nil }); err == nil {
		t.Fatal("canceled validation saved")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatalf("changed %q", data)
	}
}
