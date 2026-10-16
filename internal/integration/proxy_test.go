package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modules/local_ports"
	"github.com/zelport/zelport/internal/modules/remote_http"
	"github.com/zelport/zelport/internal/web"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func harness(t *testing.T, target, kind string, skip, rewrite bool) (*app.Manager, *web.Server, feature.Route) {
	t.Helper()
	m, e := app.New(filepath.Join(t.TempDir(), "config.json"), 8080, local_ports.New(), remote_http.New())
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close() })
	var settings any
	if kind == "local_ports" {
		u, _ := url.Parse(target)
		_, p, _ := net.SplitHostPort(u.Host)
		var port int
		fmt.Sscan(p, &port)
		settings = map[string]any{"aux_port": port}
	} else {
		settings = map[string]any{"target_url": target, "skip_tls_verify": skip, "rewrite_origin": rewrite}
	}
	b, _ := json.Marshal(settings)
	route, e := m.Save(context.Background(), 0, feature.Route{Module: kind, Domain: "project.localhost", Settings: b})
	if e != nil {
		t.Fatal(e)
	}
	s, e := web.New(m, 8080)
	if e != nil {
		t.Fatal(e)
	}
	return m, s, route
}
func TestRemoteAuthorityOriginAndErrors(t *testing.T) {
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://original.test/login")
		w.Header().Set("Set-Cookie", "x=y; Path=/service")
		w.WriteHeader(403)
		fmt.Fprintf(w, "%s %s %s", r.Host, r.URL.RequestURI(), r.Header.Get("Origin"))
	}))
	defer b.Close()
	for _, rewrite := range []bool{false, true} {
		_, s, _ := harness(t, b.URL+"/service", "remote_http", false, rewrite)
		for _, origin := range []string{"", "http://other.test", "http://project.localhost:8080"} {
			r := httptest.NewRequest("GET", "http://project.localhost:8080/a%2Fb?x=1&x=2", nil)
			r.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			wantOrigin := origin
			if rewrite && origin == "http://project.localhost:8080" {
				wantOrigin = b.URL
			}
			want := strings.TrimPrefix(b.URL, "http://") + " /service/a%2Fb?x=1&x=2 " + wantOrigin
			if w.Code != 403 || w.Body.String() != want || w.Header().Get("Location") != "https://original.test/login" || w.Header().Get("Set-Cookie") != "x=y; Path=/service" {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		}
	}
}
func TestTLSVerificationIsolation(t *testing.T) {
	b := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer b.Close()
	for _, skip := range []bool{false, true, false} {
		_, s, _ := harness(t, b.URL, "remote_http", skip, false)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", "http://project.localhost:8080/", nil))
		want := 502
		if skip {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("skip=%v: %d", skip, w.Code)
		}
	}
}
func TestStreamingAndWebSocketRevocation(t *testing.T) {
	for _, kind := range []string{"local_ports", "remote_http"} {
		t.Run(kind, func(t *testing.T) {
			b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/stream" {
					fmt.Fprint(w, "ready\n")
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				c, rw, e := w.(http.Hijacker).Hijack()
				if e != nil {
					return
				}
				defer c.Close()
				fmt.Fprint(rw, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
				rw.Flush()
				io.Copy(c, rw)
			}))
			defer b.Close()
			m, s, route := harness(t, b.URL, kind, false, false)
			p := httptest.NewServer(s)
			defer p.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "GET", p.URL+"/stream", nil)
			req.Host = "project.localhost:8080"
			resp, e := http.DefaultClient.Do(req)
			if e != nil {
				t.Fatal(e)
			}
			line, e := bufio.NewReader(resp.Body).ReadString('\n')
			if e != nil || line != "ready\n" {
				t.Fatal(line, e)
			}
			resp.Body.Close()
			u, _ := url.Parse(p.URL)
			c, e := net.Dial("tcp", u.Host)
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(3 * time.Second))
			fmt.Fprint(c, "GET /ws HTTP/1.1\r\nHost: project.localhost:8080\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
			reader := bufio.NewReader(c)
			upgrade, e := http.ReadResponse(reader, nil)
			if e != nil || upgrade.StatusCode != 101 {
				t.Fatal(upgrade, e)
			}
			payload := []byte{0x81, 2, 'o', 'k'}
			c.Write(payload)
			got := make([]byte, 4)
			if _, e = io.ReadFull(reader, got); e != nil || !bytes.Equal(got, payload) {
				t.Fatal(got, e)
			}
			if e = m.Delete(context.Background(), 1, route.ID); e != nil {
				t.Fatal(e)
			}
			if _, e = reader.ReadByte(); e == nil {
				t.Fatal("deleted route kept WebSocket alive")
			} else if n, ok := e.(net.Error); ok && n.Timeout() {
				t.Fatal("socket only closed by deadline")
			}
		})
	}
}
func TestRejectOwnGatewayTargets(t *testing.T) {
	for _, spec := range []struct {
		mod      feature.Module
		id       string
		settings string
	}{{local_ports.New(), "local_ports", `{"aux_port":8080}`}, {remote_http.New(), "remote_http", `{"target_url":"http://some.localhost:8080"}`}} {
		m, e := app.New("", 8080, spec.mod)
		if e != nil {
			t.Fatal(e)
		}
		m.Start(context.Background())
		_, e = m.Save(context.Background(), 0, feature.Route{Module: spec.id, Domain: "loop.localhost", Settings: json.RawMessage(spec.settings)})
		m.Close()
		if e == nil {
			t.Errorf("%s accepted gateway target", spec.id)
		}
	}
}
