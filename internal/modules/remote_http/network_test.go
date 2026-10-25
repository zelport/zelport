package remote_http

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
)

func networkManager(t *testing.T) (*Module, *app.Manager, string) {
	t.Helper()
	m := New().(*Module)
	path := filepath.Join(t.TempDir(), "config.json")
	manager, err := app.New(path, 8080, m)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	return m, manager, path
}
func networkSettings(m *Module, values map[string]any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(values)
	w := httptest.NewRecorder()
	m.Admin().ServeHTTP(w, httptest.NewRequest("PUT", "http://localhost:8080/settings", strings.NewReader(string(raw))))
	return w
}
func saveNetwork(t *testing.T, m *Module, manager *app.Manager, values map[string]any) {
	t.Helper()
	rev, _ := manager.Snapshot()
	values["revision"] = rev
	w := networkSettings(m, values)
	if w.Code != 200 {
		t.Fatalf("save settings: %d %s", w.Code, w.Body.String())
	}
}
func networkRoute(t *testing.T, manager *app.Manager, target string, insecure bool) feature.Route {
	t.Helper()
	rev, _ := manager.Snapshot()
	raw, _ := json.Marshal(Settings{TargetURL: target, SkipTLSVerify: insecure})
	r, err := manager.Save(context.Background(), rev, feature.Route{Module: "remote_http", Domain: "fixture.localhost", Settings: raw})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func networkRequest(manager *app.Manager) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://fixture.localhost/service?q=1", nil)
	r.Header.Set("Proxy-Authorization", "Basic inbound-secret")
	manager.ServeHTTP(w, r)
	return w
}
func TestNetworkSettingsSecretsConflictAndIdentity(t *testing.T) {
	m, manager, path := networkManager(t)
	route := networkRoute(t, manager, "http://example.test", false)
	saveNetwork(t, m, manager, map[string]any{"proxy_url": "http://127.0.0.1:1234", "proxy_username": "alice", "proxy_password": "test-proxy-secret"})
	raw, _ := json.Marshal(m.State())
	if strings.Contains(string(raw), "test-proxy-secret") || strings.Contains(string(raw), `"proxy_password"`) {
		t.Fatalf("state leaked credential: %s", raw)
	}
	if !strings.Contains(string(raw), `"has_proxy_password":true`) {
		t.Fatalf("missing password presence: %s", raw)
	}
	saveNetwork(t, m, manager, map[string]any{"proxy_url": "http://127.0.0.1:1234", "proxy_username": "alice"})
	contents, _ := os.ReadFile(path)
	if !strings.Contains(string(contents), "test-proxy-secret") {
		t.Fatal("blank unchanged password was not preserved")
	}
	rev, _ := manager.Snapshot()
	w := networkSettings(m, map[string]any{"revision": rev, "proxy_url": "http://127.0.0.1:5678", "proxy_username": "alice"})
	if w.Code != 400 {
		t.Fatalf("endpoint change reused stored password: %d %s", w.Code, w.Body.String())
	}
	w = networkSettings(m, map[string]any{"revision": rev - 1, "proxy_url": ""})
	if w.Code != 409 {
		t.Fatalf("stale write: %d %s", w.Code, w.Body.String())
	}
	saveNetwork(t, m, manager, map[string]any{"proxy_url": ""})
	contents, _ = os.ReadFile(path)
	if strings.Contains(string(contents), "test-proxy-secret") || strings.Contains(string(contents), "alice") {
		t.Fatal("disabled proxy retained credentials")
	}
	_, routes := manager.Snapshot()
	if len(routes) != 1 || routes[0].ID != route.ID {
		t.Fatal("settings changed route identity")
	}
}
func TestNetworkSettingsRejectMalformedProxyAndCA(t *testing.T) {
	m, manager, _ := networkManager(t)
	rev, _ := manager.Snapshot()
	for _, endpoint := range []string{"ftp://example.test", "http://alice:never-echo-secret@127.0.0.1:1", "http://127.0.0.1:1?q=x", "socks5://127.0.0.1:1/#bad", "http://127.0.0.1:99999", "http://127.0.0.1:0", "http://127.0.0.1:abc", "http://127.0.0.1:1/path"} {
		w := networkSettings(m, map[string]any{"revision": rev, "proxy_url": endpoint})
		if w.Code != 400 || strings.Contains(w.Body.String(), "never-echo-secret") {
			t.Fatalf("proxy %q: %d %s", endpoint, w.Code, w.Body.String())
		}
	}
	for _, contents := range []string{"not PEM", "-----BEGIN PRIVATE KEY-----\nYQ==\n-----END PRIVATE KEY-----\n"} {
		path := filepath.Join(t.TempDir(), "ca.pem")
		os.WriteFile(path, []byte(contents), 0600)
		w := networkSettings(m, map[string]any{"revision": rev, "ca_cert_path": path})
		if w.Code != 400 {
			t.Fatalf("bad CA accepted: %d %s", w.Code, w.Body.String())
		}
	}
	if err := m.ValidateSettings(json.RawMessage(`{"ca_cert_path":"/nonexistent/ca.pem"}`)); err != nil {
		t.Fatalf("configuration validation depends on filesystem: %v", err)
	}
}
func TestNetworkCustomCAAndRuntimeReplacementFailClosed(t *testing.T) {
	m, manager, _ := networkManager(t)
	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Error("origin received inbound proxy authorization")
		}
		fmt.Fprint(w, "trusted")
	}))
	defer backend.Close()
	networkRoute(t, manager, backend.URL, false)
	if w := networkRequest(manager); w.Code != 502 {
		t.Fatalf("system trust accepted fixture CA: %d", w.Code)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	bundle := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: backend.Certificate().Raw})
	os.WriteFile(path, bundle, 0600)
	saveNetwork(t, m, manager, map[string]any{"ca_cert_path": path})
	if w := networkRequest(manager); w.Code != 200 || w.Body.String() != "trusted" {
		t.Fatalf("custom trust: %d %s", w.Code, w.Body.String())
	}
	os.WriteFile(path, []byte("bad replacement"), 0600)
	if w := networkRequest(manager); w.Code != 503 {
		t.Fatalf("reused stale trust after corrupt replacement: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(string(m.State().Data), `"configuration_error":"`) || strings.Contains(string(m.State().Data), `"configuration_error":""`) {
		t.Fatalf("missing recoverable error: %s", m.State().Data)
	}
	os.WriteFile(path, bundle, 0600)
	if w := networkRequest(manager); w.Code != 200 {
		t.Fatalf("did not recover: %d %s", w.Code, w.Body.String())
	}
}
func TestNetworkHTTPProxyAuthenticationAndNoEnvironmentFallback(t *testing.T) {
	m, manager, _ := networkManager(t)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Error("proxy authorization reached origin")
		}
		io.WriteString(w, "direct")
	}))
	defer origin.Close()
	networkRoute(t, manager, origin.URL, false)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.URL.IsAbs() || r.URL.RawQuery != "q=1" {
			t.Errorf("incorrect proxy target: %s", r.URL)
		}
		if r.Header.Get("Proxy-Authorization") != "Basic YWxpY2U6c2VjcmV0" {
			t.Errorf("missing proxy credentials: %q", r.Header.Get("Proxy-Authorization"))
		}
		io.WriteString(w, "proxied")
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	if w := networkRequest(manager); w.Code != 200 || w.Body.String() != "direct" {
		t.Fatalf("environment proxy used: %d %s", w.Code, w.Body.String())
	}
	saveNetwork(t, m, manager, map[string]any{"proxy_url": proxy.URL, "proxy_username": "alice", "proxy_password": "secret"})
	if w := networkRequest(manager); w.Code != 200 || w.Body.String() != "proxied" {
		t.Fatalf("proxy unused: %d %s", w.Code, w.Body.String())
	}
}

// A settings save must revoke the bound lease before acknowledging success.
func TestNetworkSettingsSynchronouslyRevokeStreamingRequests(t *testing.T) {
	m, manager, _ := networkManager(t)
	started, closed := make(chan struct{}), make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		http.NewResponseController(w).Flush()
		close(started)
		<-r.Context().Done()
		close(closed)
	}))
	defer backend.Close()
	route := networkRoute(t, manager, backend.URL, false)
	proxy := httptest.NewServer(manager)
	defer proxy.Close()
	req, _ := http.NewRequest("GET", proxy.URL, nil)
	req.Host = "fixture.localhost"
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	<-started
	oldScope := m.Host.Scope("outbound")
	snapshot := m.Host.Snapshot()
	saveNetwork(t, m, manager, map[string]any{})
	if _, err = m.Host.Acquire(context.Background(), route.ID, snapshot.Revision, oldScope); err == nil {
		t.Fatal("old settings/scope acquired after save")
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("settings change did not revoke streaming backend")
	}
}

func TestNetworkCustomCAStillChecksHostname(t *testing.T) {
	m, manager, _ := networkManager(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"other.test"}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "origin") }))
	origin.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	origin.StartTLS()
	defer origin.Close()
	networkRoute(t, manager, origin.URL, false)
	path := filepath.Join(t.TempDir(), "ca.pem")
	os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
	saveNetwork(t, m, manager, map[string]any{"ca_cert_path": path})
	if w := networkRequest(manager); w.Code != 502 {
		t.Fatalf("custom CA bypassed hostname validation: %d %s", w.Code, w.Body.String())
	}
}
func TestNetworkMissingCARecoversAfterRestart(t *testing.T) {
	m, manager, path := networkManager(t)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "recovered") }))
	defer origin.Close()
	networkRoute(t, manager, origin.URL, false)
	certPath := filepath.Join(t.TempDir(), "ca.pem")
	bundle := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: origin.Certificate().Raw})
	os.WriteFile(certPath, bundle, 0600)
	saveNetwork(t, m, manager, map[string]any{"ca_cert_path": certPath})
	manager.Close()
	os.Remove(certPath)
	restarted := New().(*Module)
	loaded, err := app.New(path, 8080, restarted)
	if err != nil {
		t.Fatalf("missing runtime CA blocked startup: %v", err)
	}
	defer loaded.Close()
	if err = loaded.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if w := networkRequest(loaded); w.Code != 503 {
		t.Fatalf("missing CA failed open: %d %s", w.Code, w.Body.String())
	}
	os.WriteFile(certPath, bundle, 0600)
	if w := networkRequest(loaded); w.Code != 200 {
		t.Fatalf("recreated CA did not recover: %d %s", w.Code, w.Body.String())
	}
}
func TestNetworkCARejectsPrivateKeyAlongsideCertificatesAndOversizedFiles(t *testing.T) {
	m, manager, _ := networkManager(t)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer origin.Close()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: origin.Certificate().Raw})
	rev, _ := manager.Snapshot()
	for _, content := range [][]byte{append([]byte("-----BEGIN CERTIFICATE-----\ninvalid\n"), certificate...), append([]byte("-----BEGIN CERTIFICATE-----\ninvalid-base64\n-----END CERTIFICATE-----\n"), certificate...), append(append([]byte{}, certificate...), []byte("-----BEGIN PRIVATE KEY-----\nYQ==\n-----END PRIVATE KEY-----\n")...), append(append([]byte{}, certificate...), []byte("garbage")...), bytes.Repeat([]byte("x"), 1024*1024+1)} {
		path := filepath.Join(t.TempDir(), "ca.pem")
		os.WriteFile(path, content, 0600)
		w := networkSettings(m, map[string]any{"revision": rev, "ca_cert_path": path})
		if w.Code != 400 {
			t.Fatalf("invalid bundle accepted: %d %s", w.Code, w.Body.String())
		}
	}
}
