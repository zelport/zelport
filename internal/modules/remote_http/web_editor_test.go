package remote_http

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func caFileRequest(m *Module, method, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	m.Admin().ServeHTTP(w, httptest.NewRequest(method, "http://peer.zelport.localhost:8080/ca-file", strings.NewReader(body)))
	return w
}
func TestCAWebEditorValidatesAndUsesSavedPath(t *testing.T) {
	path := isolateCAHome(t)
	m, manager, _ := networkManager(t)
	w := caFileRequest(m, "GET", "")
	if w.Code != 200 {
		t.Fatalf("read: %d %s", w.Code, w.Body)
	}
	var doc struct {
		Path, Content, Version string
		Revision               uint64
	}
	json.Unmarshal(w.Body.Bytes(), &doc)
	if doc.Path != path || doc.Content != "" || doc.Version == "" {
		t.Fatalf("doc: %+v", doc)
	}
	origin := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer origin.Close()
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: origin.Certificate().Raw}))
	body := func(content string) string {
		raw, _ := json.Marshal(map[string]any{"revision": doc.Revision, "version": doc.Version, "content": content})
		return string(raw)
	}
	if w = caFileRequest(m, "PUT", body("invalid")); w.Code != 400 {
		t.Fatalf("invalid: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid created file: %v", err)
	}
	if w = caFileRequest(m, "PUT", body(cert)); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != cert {
		t.Fatal("not saved")
	}
	if w = caFileRequest(m, "PUT", body(cert)); w.Code != 409 {
		t.Fatalf("stale: %d", w.Code)
	}
	w = caFileRequest(m, "GET", "")
	json.Unmarshal(w.Body.Bytes(), &doc)
	saveNetwork(t, m, manager, map[string]any{"proxy_url": "http://proxy.example.test:8080"})
	if w = caFileRequest(m, "PUT", body(cert)); w.Code != 409 {
		t.Fatalf("settings changed: %d", w.Code)
	}
	raw, _ := json.Marshal(map[string]any{"revision": doc.Revision, "version": doc.Version, "content": cert, "path": "/other"})
	if w = caFileRequest(m, "PUT", string(raw)); w.Code != 400 {
		t.Fatalf("path injection: %d", w.Code)
	}
}

func TestCanceledCAFileSaveDoesNotCreateDefault(t *testing.T) {
	path := isolateCAHome(t)
	m, _, _ := networkManager(t)
	read := caFileRequest(m, "GET", "")
	var doc struct {
		Version  string
		Revision uint64
	}
	json.Unmarshal(read.Body.Bytes(), &doc)
	raw, _ := json.Marshal(map[string]any{"revision": doc.Revision, "version": doc.Version, "content": "invalid"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest("PUT", "http://peer.zelport.localhost:8080/ca-file", strings.NewReader(string(raw))).WithContext(ctx)
	response := httptest.NewRecorder()
	m.Admin().ServeHTTP(response, request)
	if response.Code != 503 {
		t.Fatalf("canceled save: %d", response.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("canceled save created file: %v", err)
	}
}
