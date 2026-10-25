package remote_http

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func tunnelProxy(t *testing.T, secure, auth bool) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := ""
		if auth {
			want = "Basic YWxpY2U6c2VjcmV0"
		}
		if got := r.Header.Get("Proxy-Authorization"); got != want {
			t.Errorf("proxy auth=%q, want %q", got, want)
			w.WriteHeader(407)
			return
		}
		if r.Method != http.MethodConnect {
			t.Errorf("expected CONNECT, got %s", r.Method)
			w.WriteHeader(400)
			return
		}
		upstream, err := net.DialTimeout("tcp", r.Host, time.Second)
		if err != nil {
			t.Error(err)
			w.WriteHeader(502)
			return
		}
		client, buffer, err := http.NewResponseController(w).Hijack()
		if err != nil {
			upstream.Close()
			t.Error(err)
			return
		}
		defer client.Close()
		defer upstream.Close()
		io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		done := make(chan struct{})
		go func() { io.Copy(upstream, buffer); upstream.Close(); close(done) }()
		io.Copy(client, upstream)
		client.Close()
		<-done
	})
	if secure {
		return httptest.NewTLSServer(handler)
	}
	return httptest.NewServer(handler)
}
func TestNetworkCONNECTAndHTTPSProxyTrust(t *testing.T) {
	for _, secure := range []bool{false, true} {
		for _, auth := range []bool{false, true} {
			t.Run(fmt.Sprintf("tls=%v/auth=%v", secure, auth), func(t *testing.T) {
				m, manager, _ := networkManager(t)
				origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Proxy-Authorization") != "" {
						t.Error("CONNECT credentials forwarded to origin")
					}
					io.WriteString(w, "tunneled")
				}))
				defer origin.Close()
				networkRoute(t, manager, origin.URL, false)
				proxy := tunnelProxy(t, secure, auth)
				defer proxy.Close()
				path := filepath.Join(t.TempDir(), "ca.pem")
				os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: origin.Certificate().Raw}), 0600)
				values := map[string]any{"ca_cert_path": path, "proxy_url": proxy.URL}
				if auth {
					values["proxy_username"] = "alice"
					values["proxy_password"] = "secret"
				}
				saveNetwork(t, m, manager, values)
				if w := networkRequest(manager); w.Code != 200 || w.Body.String() != "tunneled" {
					t.Fatalf("CONNECT: %d %s", w.Code, w.Body.String())
				}
			})
		}
	}
}
func TestNetworkInsecureOriginStillVerifiesHTTPSProxy(t *testing.T) {
	m, manager, _ := networkManager(t)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "origin") }))
	defer origin.Close()
	networkRoute(t, manager, origin.URL, true)
	proxy := tunnelProxy(t, true, false)
	defer proxy.Close()
	saveNetwork(t, m, manager, map[string]any{"proxy_url": proxy.URL})
	if w := networkRequest(manager); w.Code != 502 {
		t.Fatalf("untrusted HTTPS proxy bypassed TLS verification: %d %s", w.Code, w.Body.String())
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw}), 0600)
	saveNetwork(t, m, manager, map[string]any{"proxy_url": proxy.URL, "ca_cert_path": path})
	if w := networkRequest(manager); w.Code != 200 {
		t.Fatalf("trusted HTTPS proxy: %d %s", w.Code, w.Body.String())
	}
}

func socksProxy(t *testing.T, auth bool, targetHost string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var connections sync.WaitGroup
	t.Cleanup(func() { listener.Close(); connections.Wait() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)
				greeting := make([]byte, 2)
				if _, err = io.ReadFull(reader, greeting); err != nil {
					return
				}
				methods := make([]byte, int(greeting[1]))
				io.ReadFull(reader, methods)
				method := byte(0)
				if auth {
					method = 2
				}
				conn.Write([]byte{5, method})
				if auth {
					prefix := make([]byte, 2)
					io.ReadFull(reader, prefix)
					username := make([]byte, int(prefix[1]))
					io.ReadFull(reader, username)
					length, _ := reader.ReadByte()
					password := make([]byte, int(length))
					io.ReadFull(reader, password)
					if string(username) != "alice" || string(password) != "secret" {
						t.Errorf("SOCKS auth: %q/%q", username, password)
						return
					}
					conn.Write([]byte{1, 0})
				}
				request := make([]byte, 4)
				if _, err = io.ReadFull(reader, request); err != nil {
					return
				}
				host := ""
				switch request[3] {
				case 1:
					b := make([]byte, 4)
					io.ReadFull(reader, b)
					host = net.IP(b).String()
				case 4:
					b := make([]byte, 16)
					io.ReadFull(reader, b)
					host = net.IP(b).String()
				case 3:
					length, _ := reader.ReadByte()
					b := make([]byte, int(length))
					io.ReadFull(reader, b)
					host = string(b)
				default:
					t.Errorf("SOCKS address type %d", request[3])
					return
				}
				if host != targetHost {
					t.Errorf("SOCKS proxy did not receive target hostname: %q", host)
					return
				}
				port := make([]byte, 2)
				io.ReadFull(reader, port)
				upstream, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(binary.BigEndian.Uint16(port))))
				if err != nil {
					t.Error(err)
					return
				}
				defer upstream.Close()
				conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				done := make(chan struct{})
				go func() { io.Copy(upstream, reader); upstream.Close(); close(done) }()
				io.Copy(conn, upstream)
				conn.Close()
				<-done
			}()
		}
	}()
	return listener.Addr().String()
}
func TestNetworkSOCKSProxyAuthenticationAndRemoteDNS(t *testing.T) {
	for _, scheme := range []string{"socks5", "socks5h"} {
		for _, auth := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/auth=%v", scheme, auth), func(t *testing.T) {
				m, manager, _ := networkManager(t)
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Proxy-Authorization") != "" {
						t.Error("proxy authorization reached SOCKS origin")
					}
					io.WriteString(w, "socks")
				}))
				defer origin.Close()
				_, port, _ := net.SplitHostPort(strings.TrimPrefix(origin.URL, "http://"))
				networkRoute(t, manager, "http://origin.test:"+port, false)
				endpoint := socksProxy(t, auth, "origin.test")
				values := map[string]any{"proxy_url": scheme + "://" + endpoint}
				if auth {
					values["proxy_username"] = "alice"
					values["proxy_password"] = "secret"
				}
				saveNetwork(t, m, manager, values)
				if w := networkRequest(manager); w.Code != 200 || w.Body.String() != "socks" {
					t.Fatalf("SOCKS: %d %s", w.Code, w.Body.String())
				}
			})
		}
	}
}
func TestNetworkSettingsRevokeWebSockets(t *testing.T) {
	for _, transport := range []string{"direct", "http-connect", "socks5"} {
		t.Run(transport, func(t *testing.T) {
			m, manager, _ := networkManager(t)
			backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Proxy-Authorization") != "" {
					t.Error("proxy credentials reached WebSocket origin")
				}
				conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
				if err != nil {
					return
				}
				defer conn.CloseNow()
				for {
					kind, data, err := conn.Read(r.Context())
					if err != nil {
						return
					}
					if err = conn.Write(r.Context(), kind, data); err != nil {
						return
					}
				}
			}))
			defer backend.Close()
			networkRoute(t, manager, backend.URL, false)
			path := filepath.Join(t.TempDir(), "ca.pem")
			if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: backend.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			values := map[string]any{"ca_cert_path": path}
			switch transport {
			case "http-connect":
				outbound := tunnelProxy(t, false, true)
				defer outbound.Close()
				values["proxy_url"] = outbound.URL
			case "socks5":
				values["proxy_url"] = "socks5://" + socksProxy(t, true, "127.0.0.1")
			}
			if transport != "direct" {
				values["proxy_username"] = "alice"
				values["proxy_password"] = "secret"
			}
			saveNetwork(t, m, manager, values)
			proxy := httptest.NewServer(manager)
			defer proxy.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(proxy.URL, "http"), &websocket.DialOptions{Host: "fixture.localhost"})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow()
			if err = conn.Write(ctx, websocket.MessageText, []byte("before")); err != nil {
				t.Fatal(err)
			}
			_, data, err := conn.Read(ctx)
			if err != nil || string(data) != "before" {
				t.Fatalf("WebSocket forwarding: %s %v", data, err)
			}
			saveNetwork(t, m, manager, map[string]any{"ca_cert_path": path})
			readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
			defer readCancel()
			if _, _, err = conn.Read(readCtx); err == nil || readCtx.Err() != nil {
				t.Fatalf("WebSocket was not revoked by settings change: %v", err)
			}
		})
	}
}
func TestNetworkProxyErrorsDoNotExposeCredentials(t *testing.T) {
	m, manager, _ := networkManager(t)
	networkRoute(t, manager, "http://example.test", false)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listener.Addr().String()
	listener.Close()
	saveNetwork(t, m, manager, map[string]any{"proxy_url": "socks5://" + endpoint, "proxy_username": "proxy-private-user", "proxy_password": "proxy-private-password"})
	w := networkRequest(manager)
	if w.Code != 502 || strings.Contains(w.Body.String(), "proxy-private") || strings.Contains(w.Body.String(), "Basic ") {
		t.Fatalf("unsafe dial failure: %d %s", w.Code, w.Body.String())
	}
}
