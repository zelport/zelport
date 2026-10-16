package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport-sdk-go/transport"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/web"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func setup(t *testing.T, path string) (*Module, *app.Manager, *web.Server) {
	t.Helper()
	mod := New().(*Module)
	manager, err := app.New(path, 8080, mod)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	server, err := web.New(manager, 8080)
	if err != nil {
		t.Fatal(err)
	}
	return mod, manager, server
}
func api(t *testing.T, s *web.Server, method, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	state := httptest.NewRecorder()
	s.ServeHTTP(state, httptest.NewRequest("GET", "http://localhost:8080/api/state", nil))
	var v struct {
		CSRF     string `json:"csrf"`
		Revision uint64 `json:"revision"`
	}
	if err := json.Unmarshal(state.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["revision"]; !ok {
		body["revision"] = v.Revision
	}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://localhost:8080/api/modules/relay/"+path, strings.NewReader(string(b)))
	r.Header.Set("Origin", "http://localhost:8080")
	r.Header.Set("X-CSRF-Token", v.CSRF)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func freePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}
func save(t *testing.T, s *web.Server, enabled bool, port int) *httptest.ResponseRecorder {
	return api(t, s, "PUT", "settings", map[string]any{"enabled": enabled, "port": port, "bind_address": "127.0.0.1"})
}
func mustOK(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
}

func TestLifecyclePersistenceAndSecretProtection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mod, m, s := setup(t, path)
	port := freePort(t)
	mustOK(t, save(t, s, true, port))
	w := api(t, s, "POST", "credentials", map[string]any{"action": "reveal"})
	mustOK(t, w)
	var secret struct {
		Token string `json:"access_token"`
	}
	json.Unmarshal(w.Body.Bytes(), &secret)
	token, e := protocol.DecodeKey(secret.Token)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(mod.State())
	if strings.Contains(string(b), secret.Token) {
		t.Fatal("state leaks secret")
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("configuration permissions", info, e)
	}
	c, e := transport.Dial(context.Background(), fmt.Sprintf("ws://127.0.0.1:%d", port), token, "control")
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	mustOK(t, save(t, s, false, port))
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, e = c.Read(make([]byte, 1)); e == nil {
		t.Fatal("active tunnel survived disable")
	} else if n, ok := e.(net.Error); ok && n.Timeout() {
		t.Fatal("disable left upgraded connection waiting")
	}
	m.Close()
	_, _, s = setup(t, path)
	w = api(t, s, "POST", "credentials", map[string]any{"action": "reveal"})
	mustOK(t, w)
	if !strings.Contains(w.Body.String(), secret.Token) {
		t.Fatal("token changed on restart")
	}
	mustOK(t, save(t, s, true, port))
}

func TestFailedBindAndWritePreserveRunningServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	mod, _, s := setup(t, path)
	port := freePort(t)
	mustOK(t, save(t, s, true, port))
	occupied, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer occupied.Close()
	w := save(t, s, true, occupied.Addr().(*net.TCPAddr).Port)
	if w.Code == 200 {
		t.Fatal("occupied port accepted")
	}
	before, _ := os.ReadFile(path)
	// Replacing the destination with a directory forces a real atomic rename failure.
	if e = os.Rename(path, path+".saved"); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	next := freePort(t)
	w = save(t, s, true, next)
	if w.Code == 200 {
		t.Fatal("failed persistence accepted")
	}
	if !strings.Contains(string(mod.State().Data), "Listening") {
		t.Fatal("old server stopped")
	}
	c, e := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	l, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", next))
	if e != nil {
		t.Fatal("reserved listener leaked", e)
	}
	l.Close()
	os.Remove(path)
	os.Rename(path+".saved", path)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("config changed")
	}
	mustOK(t, save(t, s, true, next))
	if c, e = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); e == nil {
		c.Close()
		t.Fatal("old listener remains")
	}
}

func TestStartupBindErrorIsRecoverable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	_, m, s := setup(t, path)
	port := freePort(t)
	mustOK(t, save(t, s, true, port))
	m.Close()
	occupied, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		t.Fatal(e)
	}
	mod, _, s := setup(t, path)
	if !strings.Contains(string(mod.State().Data), "Error") {
		t.Fatal("missing startup error")
	}
	occupied.Close()
	mustOK(t, save(t, s, true, port))
	if !strings.Contains(string(mod.State().Data), "Listening") {
		t.Fatal("retry failed")
	}
}

func TestConcurrentEditsAndAdminProtection(t *testing.T) {
	_, _, s := setup(t, filepath.Join(t.TempDir(), "config.json"))
	port := freePort(t)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- api(t, s, "PUT", "settings", map[string]any{"revision": uint64(0), "enabled": false, "port": port, "bind_address": "127.0.0.1"}).Code
		}()
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for code := range codes {
		got[code]++
	}
	if got[200] != 1 || got[409] != 1 {
		t.Fatal(got)
	}
	for _, path := range []string{"settings", "credentials"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "http://localhost:8080/api/modules/relay/"+path, strings.NewReader(`{}`))
		s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("unprotected mutation", w.Code)
		}
	}
}

func TestInvalidSettingsAndMissingModule(t *testing.T) {
	mod, _, s := setup(t, filepath.Join(t.TempDir(), "config.json"))
	for _, raw := range []string{`{"port":0}`, `{"port":65536}`, `{"port":-1}`, `{"bind_address":"example.com"}`, `{"bind_address":"0.0.0.0"}`, `{"bind_address":"224.0.0.1"}`, `{"enabled":true}`, `{"access_token":"bad"}`, `{"unknown":true}`} {
		if mod.ValidateSettings(json.RawMessage(raw)) == nil {
			t.Fatal("invalid settings accepted", raw)
		}
	}
	if mod.ValidateSettings(json.RawMessage(`{}`)) != nil {
		t.Fatal("default settings invalid")
	}
	w := api(t, s, "GET", "credentials", map[string]any{})
	if w.Code != 404 {
		t.Fatal("GET revealed token", w.Code)
	}
	m, e := app.New("", 8080)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	core, e := web.New(m, 8080)
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/modules/relay/settings", "/assets/modules/relay/module.js"} {
		w := httptest.NewRecorder()
		core.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost:8080"+path, nil))
		if w.Code != 404 {
			t.Fatal("excluded module exposed", path, w.Code)
		}
	}
}

func TestTokenRotationRevokesConnectionsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mod, manager, s := setup(t, path)
	port := freePort(t)
	mustOK(t, save(t, s, true, port))
	reveal := func() string {
		w := api(t, s, "POST", "credentials", map[string]any{"action": "reveal"})
		mustOK(t, w)
		var v map[string]string
		json.Unmarshal(w.Body.Bytes(), &v)
		return v["access_token"]
	}
	old := reveal()
	if api(t, s, "POST", "credentials", map[string]any{"action": "replace", "access_token": old[:8] + "\n" + old[8:]}).Code != 400 {
		t.Fatal("same key with different encoding accepted")
	}

	key, _ := protocol.DecodeKey(old)
	endpoint := fmt.Sprintf("ws://127.0.0.1:%d", port)
	c, err := transport.Dial(context.Background(), endpoint, key, "control")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	revision := mod.State().Revision
	w := api(t, s, "POST", "credentials", map[string]any{"action": "generate", "revision": revision})
	mustOK(t, w)
	next := reveal()
	if next == old {
		t.Fatal("rotation retained old token")
	}
	if strings.Contains(w.Body.String(), next) {
		t.Fatal("mutation response exposed token")
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, err = c.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection survived rotation")
	} else if n, ok := err.(net.Error); ok && n.Timeout() {
		t.Fatal("connection not closed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if stale, e := transport.Dial(ctx, endpoint, key, "control"); e == nil {
		stale.Close()
		t.Fatal("old token still admitted")
	}
	key, _ = protocol.DecodeKey(next)
	fresh, err := transport.Dial(ctx, endpoint, key, "control")
	if err != nil {
		t.Fatal(err)
	}
	fresh.Close()
	if api(t, s, "POST", "credentials", map[string]any{"action": "generate", "revision": revision}).Code != 409 {
		t.Fatal("stale rotation accepted")
	}
	if api(t, s, "POST", "credentials", map[string]any{"action": "replace", "access_token": "invalid"}).Code != 400 {
		t.Fatal("invalid token accepted")
	}
	mustOK(t, api(t, s, "POST", "credentials", map[string]any{"action": "replace", "access_token": old}))
	if reveal() != old {
		t.Fatal("explicit replacement failed")
	}
	manager.Close()
	_, _, s = setup(t, path)
	if reveal() != old {
		t.Fatal("replacement not persisted")
	}
}

func TestFailedTokenPersistencePreservesAdmission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mod, _, s := setup(t, path)
	port := freePort(t)
	mustOK(t, save(t, s, true, port))
	before := mod.State().Revision
	w := api(t, s, "POST", "credentials", map[string]any{"action": "reveal"})
	mustOK(t, w)
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	if err := os.Rename(path, path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Remove(path); os.Rename(path+".saved", path) }()
	if api(t, s, "POST", "credentials", map[string]any{"action": "generate"}).Code == 200 {
		t.Fatal("failed write accepted")
	}
	if mod.State().Revision != before {
		t.Fatal("failed write changed revision")
	}
	key, _ := protocol.DecodeKey(v["access_token"])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := transport.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d", port), key, "control")
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestConcurrentTokenRotationsHaveOneWinner(t *testing.T) {
	mod, _, s := setup(t, filepath.Join(t.TempDir(), "config.json"))
	revision := mod.State().Revision
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- api(t, s, "POST", "credentials", map[string]any{"revision": revision, "action": "generate"}).Code
		}()
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for code := range codes {
		got[code]++
	}
	if got[200] != 1 || got[409] != 1 {
		t.Fatal(got)
	}
}

func TestOptionalAccessTokenModeSwitch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mod, manager, s := setup(t, path)
	port := freePort(t)
	set := func(required bool) *httptest.ResponseRecorder {
		return api(t, s, "PUT", "settings", map[string]any{"enabled": true, "port": port, "bind_address": "127.0.0.1", "require_access_token": required})
	}
	mustOK(t, set(false))
	var state struct {
		Require  bool `json:"require_access_token"`
		HasToken bool `json:"has_token"`
	}
	json.Unmarshal(mod.State().Data, &state)
	if state.Require || state.HasToken {
		t.Fatal("first open enable created a token or stayed protected")
	}
	endpoint := fmt.Sprintf("ws://127.0.0.1:%d", port)
	c, e := transport.Dial(context.Background(), endpoint, nil, "control")
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	mustOK(t, set(true))
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, e = c.Read(make([]byte, 1)); e == nil {
		t.Fatal("open connection survived protection")
	} else if n, ok := e.(net.Error); ok && n.Timeout() {
		t.Fatal("revocation timed out")
	}
	w := api(t, s, "POST", "credentials", map[string]any{"action": "reveal"})
	mustOK(t, w)
	var secret map[string]string
	json.Unmarshal(w.Body.Bytes(), &secret)
	key, _ := protocol.DecodeKey(secret["access_token"])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if c, e := transport.Dial(ctx, endpoint, nil, "control"); e == nil {
		c.Close()
		t.Fatal("protected relay accepted empty token")
	}
	mustOK(t, set(false))
	// Rotating a saved credential while open must not silently turn protection on.
	mustOK(t, api(t, s, "POST", "credentials", map[string]any{"action": "generate"}))
	c, e = transport.Dial(context.Background(), endpoint, nil, "control")
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	w = api(t, s, "POST", "credentials", map[string]any{"action": "reveal"})
	mustOK(t, w)
	json.Unmarshal(w.Body.Bytes(), &secret)
	key, _ = protocol.DecodeKey(secret["access_token"])
	manager.Close()
	mod, _, s = setup(t, path)
	c, e = transport.Dial(context.Background(), endpoint, nil, "control")
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	mustOK(t, set(true))
	c, e = transport.Dial(context.Background(), endpoint, key, "control")
	if e != nil {
		t.Fatal("stored token not reused", e)
	}
	c.Close()
}

func TestFailedOpenModeSavePreservesProtectedServer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mod, _, s := setup(t, path)
	port := freePort(t)
	mustOK(t, save(t, s, true, port))
	w := api(t, s, "POST", "credentials", map[string]any{"action": "reveal"})
	mustOK(t, w)
	var secret map[string]string
	json.Unmarshal(w.Body.Bytes(), &secret)
	key, _ := protocol.DecodeKey(secret["access_token"])
	endpoint := fmt.Sprintf("ws://127.0.0.1:%d", port)
	c, e := transport.Dial(context.Background(), endpoint, key, "control")
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if e = os.Rename(path, path+".saved"); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	defer func() { os.Remove(path); os.Rename(path+".saved", path) }()
	w = api(t, s, "PUT", "settings", map[string]any{"enabled": true, "port": port, "bind_address": "127.0.0.1", "require_access_token": false})
	if w.Code == 200 {
		t.Fatal("failed write accepted")
	}
	if !strings.Contains(string(mod.State().Data), `"require_access_token":true`) {
		t.Fatal("failed write changed mode")
	}
	// A valid request on the original control connection must still receive a response.
	c.SetDeadline(time.Now().Add(time.Second))
	if e = protocol.WriteJSON(c, protocol.Message{Type: "catalog", ID: protocol.ID()}); e != nil {
		t.Fatal(e)
	}
	var msg protocol.Message
	if e = protocol.ReadJSON(c, &msg); e != nil {
		t.Fatal("old connection revoked on write failure", e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if open, e := transport.Dial(ctx, endpoint, nil, "control"); e == nil {
		open.Close()
		t.Fatal("failed write opened admission")
	}
}

func TestDesktopConnectionSummary(t *testing.T) {
	mod, _, _ := setup(t, "")
	v := mod.ConnectionSummaries()
	if len(v) != 1 || v[0].ID != "relay" || v[0].State != "stopped" || v[0].Label != "Built-in Relay" {
		t.Fatalf("%+v", v)
	}
	mod.mu.Lock()
	mod.failure = "secret raw runtime error"
	mod.mu.Unlock()
	v = mod.ConnectionSummaries()
	if v[0].State != "stopped" {
		t.Fatal("disabled relay must match header status")
	}
}
