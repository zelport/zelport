package ports_over_ssh

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSSHWebEditorCreatesSavedFileAndRejectsChangedContents(t *testing.T) {
	m, manager, _ := sshTestManager(t)
	path := filepath.Join(t.TempDir(), "new", "ssh-config")
	rev, _ := manager.Snapshot()
	raw, _ := json.Marshal(map[string]any{"revision": rev, "ssh_config_path": path})
	if w := sshRequest(m, "PUT", "/settings", string(raw)); w.Code != 200 {
		t.Fatal(w.Body)
	}
	w := sshRequest(m, "GET", "/config-file", "")
	if w.Code != 200 {
		t.Fatalf("read: %d %s", w.Code, w.Body)
	}
	var doc struct {
		Path, Content, Version string
		Revision               uint64
	}
	json.Unmarshal(w.Body.Bytes(), &doc)
	if doc.Path != path || doc.Version == "" {
		t.Fatalf("doc %+v", doc)
	}
	body := func(text string) string {
		raw, _ := json.Marshal(map[string]any{"revision": doc.Revision, "version": doc.Version, "content": text})
		return string(raw)
	}
	const text = "Host browser-example\n  HostName example.test\n"
	if w = sshRequest(m, "PUT", "/config-file", body(text)); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != text {
		t.Fatalf("file %q", actual)
	}
	if w = sshRequest(m, "GET", "/hosts", ""); !strings.Contains(w.Body.String(), "browser-example") {
		t.Fatalf("discovery %s", w.Body)
	}
	w = sshRequest(m, "GET", "/config-file", "")
	json.Unmarshal(w.Body.Bytes(), &doc)
	os.WriteFile(path, []byte("Host external\n"), 0600)
	if w = sshRequest(m, "PUT", "/config-file", body(text)); w.Code != 409 {
		t.Fatalf("external conflict: %d", w.Code)
	}
	actual, _ = os.ReadFile(path)
	if string(actual) != "Host external\n" {
		t.Fatalf("external overwritten %q", actual)
	}
}

func TestCanceledSSHFileSaveDoesNotWriteAfterWaitingForLock(t *testing.T) {
	m, manager, _ := sshTestManager(t)
	path := filepath.Join(t.TempDir(), "config")
	os.WriteFile(path, []byte("Host original\n"), 0600)
	rev, _ := manager.Snapshot()
	raw, _ := json.Marshal(map[string]any{"revision": rev, "ssh_config_path": path})
	sshRequest(m, "PUT", "/settings", string(raw))
	read := sshRequest(m, "GET", "/config-file", "")
	var doc struct {
		Version  string
		Revision uint64
	}
	json.Unmarshal(read.Body.Bytes(), &doc)
	raw, _ = json.Marshal(map[string]any{"revision": doc.Revision, "version": doc.Version, "content": "Host canceled\n"})
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("PUT", "http://localhost:8080/config-file", strings.NewReader(string(raw))).WithContext(ctx)
	response := httptest.NewRecorder()
	m.configMu.Lock()
	done := make(chan struct{})
	go func() { m.Admin().ServeHTTP(response, request); close(done) }()
	cancel()
	m.configMu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request blocked")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "Host original\n" {
		t.Fatalf("canceled save wrote %q", data)
	}
	if response.Code == 200 {
		t.Fatal("canceled save succeeded")
	}
}
