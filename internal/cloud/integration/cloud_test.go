package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport-sdk-go/client"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	cloud "github.com/zelport/zelport/internal/modules/ports_over_relay"
	"github.com/zelport/zelport/internal/web"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type appModule struct {
	modulekit.Base
	calls   chan string
	handler http.Handler
}

func (m *appModule) ValidateRoute(feature.Route) error { return nil }
func (m *appModule) ServeRoute(w http.ResponseWriter, r *http.Request, _ feature.Route) {
	select {
	case m.calls <- r.URL.Path:
	default:
	}
	if m.handler != nil {
		m.handler.ServeHTTP(w, r)
		return
	}
	switch r.URL.Path {
	case "/ws":
		c, b, e := w.(http.Hijacker).Hijack()
		if e != nil {
			return
		}
		defer c.Close()
		fmt.Fprint(b, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		b.Flush()
		io.Copy(c, c)
	case "/stream":
		fmt.Fprintln(w, "first")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	default:
		w.WriteHeader(403)
		fmt.Fprint(w, r.Host+" "+r.URL.RequestURI()+" "+r.Header.Get("Origin"))
	}
}

type instance struct {
	m    *app.Manager
	mod  *cloud.Module
	web  *web.Server
	port int
	path string
}

func newInstance(t *testing.T, port int) (*instance, *appModule) {
	t.Helper()
	mod := cloud.New().(*cloud.Module)
	src := &appModule{Base: modulekit.Base{Desc: feature.Descriptor{Maturity: feature.Stable, ID: "test_source", Name: "Test", TabID: "test", Exportable: true}}, calls: make(chan string, 20)}
	path := filepath.Join(t.TempDir(), "config.json")
	m, e := app.New(path, port, src, mod)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	s, e := web.New(m, port)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close() })
	return &instance{m, mod, s, port, path}, src
}
func (i *instance) api(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	state := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost/api/state", nil)
	req.Host = fmt.Sprintf("localhost:%d", i.port)
	i.web.ServeHTTP(state, req)
	var st struct {
		CSRF     string `json:"csrf"`
		Revision uint64 `json:"revision"`
	}
	json.Unmarshal(state.Body.Bytes(), &st)
	v := body.(map[string]any)
	v["revision"] = st.Revision
	b, _ := json.Marshal(v)
	req = httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(string(b)))
	req.Host = fmt.Sprintf("localhost:%d", i.port)
	req.Header.Set("Origin", fmt.Sprintf("http://localhost:%d", i.port))
	req.Header.Set("X-CSRF-Token", st.CSRF)
	w := httptest.NewRecorder()
	i.web.ServeHTTP(w, req)
	return w
}
func (i *instance) settings(t *testing.T, url, access, group string, publish bool) {
	t.Helper()
	for range 20 {
		w := i.api(t, "PUT", "/api/modules/ports_over_relay/settings", map[string]any{"enabled": true, "publish": publish, "relay_url": url, "access_token": access, "group_token": group})
		if w.Code == 409 {
			continue
		}
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return
	}
	t.Fatal("settings remained conflicted")
}
func save(t *testing.T, i *instance, r feature.Route) feature.Route {
	t.Helper()
	for range 50 {
		rev, _ := i.m.Snapshot()
		v, e := i.m.Save(context.Background(), rev, r)
		if e == feature.ErrConflict {
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	t.Fatal("save remained conflicted")
	return r
}
func waitOffer(t *testing.T, i *instance, id string) protocol.Record {
	t.Helper()
	until := time.Now().Add(8 * time.Second)
	for time.Now().Before(until) {
		s := i.mod.State()
		var data struct {
			Status client.Status `json:"status"`
		}
		json.Unmarshal(s.Data, &data)
		if data.Status.Connected {
			for _, r := range data.Status.Records {
				for _, svc := range r.Services {
					if svc.ID == id {
						return r
					}
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("authenticated offer unavailable", string(i.mod.State().Data))
	return protocol.Record{}
}
func TestCloudHTTPWebSocketStreamingAndRevocation(t *testing.T) {
	for _, mode := range []string{"standalone-runtime", "embedded", "embedded-open"} {
		t.Run(mode, func(t *testing.T) {
			url, access, _ := hostedRelay(t, mode)
			testCloudHTTPWebSocketStreamingAndRevocation(t, url, access)
		})
	}
}
func testCloudHTTPWebSocketStreamingAndRevocation(t *testing.T, url, access string) {
	group, _ := protocol.Token()
	provider, _ := newInstance(t, 18090)
	consumer, _ := newInstance(t, 18091)
	source := save(t, provider, feature.Route{Module: "test_source", Domain: "source.localhost", Settings: json.RawMessage(`{}`)})
	provider.settings(t, url, access, group, true)
	consumer.settings(t, url, access, group, false)
	waitPeerCatalogs(t, provider, consumer)
	offer := waitOffer(t, consumer, source.ID)
	ref := cloud.ImportSettings{Import: protocol.Import{Group: offer.Group, DeviceID: offer.DeviceID, ServiceID: source.ID, ProviderKey: offer.PublicKey}, RewriteOrigin: true}
	b, _ := json.Marshal(ref)
	save(t, consumer, feature.Route{Module: "ports_over_relay", Domain: "renamed.localhost", Settings: b})
	server := httptest.NewServer(consumer.web)
	defer server.Close()
	req, _ := http.NewRequest("GET", server.URL+"/app?q=1", nil)
	req.Host = "renamed.localhost:18091"
	req.Header.Set("Origin", "http://renamed.localhost:18091")
	response, e := server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 403 || string(body) != "source.localhost:18090 /app?q=1 http://source.localhost:18090" {
		t.Fatal(response.StatusCode, string(body))
	}

	for _, origin := range []string{"https://foreign.example", ""} {
		req, _ = http.NewRequest("GET", server.URL+"/origin", nil)
		req.Host = "renamed.localhost:18091"
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		response, e = server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		body, _ = io.ReadAll(response.Body)
		response.Body.Close()
		if string(body) != "source.localhost:18090 /origin "+origin {
			t.Fatal("foreign/missing Origin changed", string(body))
		}
	}
	// Source renaming preserves the UUID; authenticated catalog updates authority.
	source.Domain = "new-source.localhost"
	save(t, provider, source)
	until := time.Now().Add(5 * time.Second)
	for {
		v := waitOffer(t, consumer, source.ID)
		authority, _ := v.Authority(source.ID)
		if authority == "new-source.localhost:18090" {
			break
		}
		if time.Now().After(until) {
			t.Fatal("source rename not propagated")
		}
		time.Sleep(10 * time.Millisecond)
	}
	req, _ = http.NewRequest("GET", server.URL+"/renamed", nil)
	req.Host = "renamed.localhost:18091"
	response, e = server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 403 || string(body) != "new-source.localhost:18090 /renamed " {
		t.Fatal("rename rebound by domain", response.StatusCode, string(body))
	}
	req, _ = http.NewRequest("GET", server.URL+"/stream", nil)
	req.Host = "renamed.localhost:18091"
	response, e = server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	line, e := bufio.NewReader(response.Body).ReadString('\n')
	if e != nil || line != "first\n" {
		t.Fatal(e, line)
	}
	response.Body.Close()
	conn, e := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprint(conn, "GET /ws HTTP/1.1\r\nHost: renamed.localhost:18091\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	reader := bufio.NewReader(conn)
	response, e = http.ReadResponse(reader, nil)
	if e != nil || response.StatusCode != 101 {
		t.Fatal(e, response)
	}
	conn.Write([]byte("echo"))
	v := make([]byte, 4)
	if _, e = io.ReadFull(reader, v); e != nil || string(v) != "echo" {
		t.Fatal(e, string(v))
	}
	provider.settings(t, url, access, group, false)
	if _, e = reader.ReadByte(); e == nil {
		t.Fatal("WebSocket survived publication disable")
	} else if n, ok := e.(net.Error); ok && n.Timeout() {
		t.Fatal("revocation did not close connection")
	}
}

func TestCloudIndirectLoopAndLostResponse(t *testing.T) {
	access, _ := protocol.Token()
	token, _ := protocol.DecodeKey(access)
	group, _ := protocol.Token()
	relayServer, _ := relay.New(relay.Options{AccessToken: token})
	rs := httptest.NewServer(relayServer)
	defer rs.Close()
	defer relayServer.Close()
	provider, src := newInstance(t, 18190)
	consumer, _ := newInstance(t, 18191)
	server := httptest.NewServer(consumer.web)
	defer server.Close()
	var calls atomic.Int32
	src.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/lost" {
			c, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				c.Close()
			}
			return
		}
		req, _ := http.NewRequestWithContext(r.Context(), r.Method, server.URL+r.URL.RequestURI(), r.Body)
		req.Host = "loop.localhost:18191"
		req.Header = r.Header.Clone()
		if r.URL.Path == "/stripped" {
			req.Header.Del(protocol.TraceHeader)
		}
		response, e := server.Client().Do(req)
		if e != nil {
			http.Error(w, "intermediary disconnected", 502)
			return
		}
		defer response.Body.Close()
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	})
	source := save(t, provider, feature.Route{Module: "test_source", Domain: "loop-source.localhost", Settings: json.RawMessage(`{}`)})
	relayURL := "ws" + rs.URL[4:]
	provider.settings(t, relayURL, access, group, true)
	consumer.settings(t, relayURL, access, group, false)
	offer := waitOffer(t, consumer, source.ID)
	ref, _ := json.Marshal(cloud.ImportSettings{Import: protocol.Import{Group: offer.Group, DeviceID: offer.DeviceID, ServiceID: source.ID, ProviderKey: offer.PublicKey}})
	imported := save(t, consumer, feature.Route{Module: "ports_over_relay", Domain: "loop.localhost", Settings: ref})
	for _, tc := range []struct {
		path string
		code int
		max  int32
	}{{"/loop", 508, 1}, {"/stripped", 503, 32}, {"/lost", 503, 1}} {
		calls.Store(0)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+tc.path, strings.NewReader("side effect"))
		req.Host = "loop.localhost:18191"
		response, e := server.Client().Do(req)
		if e != nil {
			cancel()
			t.Fatal(tc.path, e)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		cancel()
		if response.StatusCode != tc.code || calls.Load() > tc.max || calls.Load() == 0 {
			t.Fatal(tc.path, response.StatusCode, calls.Load())
		}
	}
	// An unavailable import remains editable, but its binding cannot be silently changed.
	consumer.settings(t, relayURL, access, group, false)
	consumer.mod.Close()
	imported.Domain = "offline.localhost"
	save(t, consumer, imported)
}

func TestCloudAPISecurity(t *testing.T) {
	i, _ := newInstance(t, 18290)
	for _, tc := range []struct {
		host, origin, csrf string
		want               int
	}{
		{"localhost:18290", "http://localhost:18290", "", 403},
		{"localhost:18290", "https://attacker.example", "invalid", 403},
		{"project.localhost:18290", "http://project.localhost:18290", "", 404},
	} {
		req := httptest.NewRequest("POST", "http://localhost/api/modules/ports_over_relay/credentials", strings.NewReader(`{"revision":0,"action":"generate"}`))
		req.Host = tc.host
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("X-CSRF-Token", tc.csrf)
		w := httptest.NewRecorder()
		i.web.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatal(tc, w.Code)
		}
	}
	if strings.Contains(string(i.mod.State().Data), `"has_identity":true`) {
		t.Fatal("rejected action created identity")
	}
}
