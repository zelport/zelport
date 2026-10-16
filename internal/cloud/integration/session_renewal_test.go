package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	cloud "github.com/zelport/zelport/internal/modules/ports_over_relay"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var longRenewal = flag.Bool("long-renewal", false, "transfer over 3 GiB per relay mode to exercise production renewal thresholds")

type zeroStream struct{}

func (zeroStream) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type verifyZeros struct{ total int64 }

func (v *verifyZeros) Write(p []byte) (int, error) {
	for _, b := range p {
		if b != 0 {
			return 0, fmt.Errorf("stream corruption at offset %d", v.total)
		}
	}
	v.total += int64(len(p))
	return len(p), nil
}
func TestSessionRenewalPreservesStreams(t *testing.T) {
	if !*longRenewal {
		t.Skip("use -long-renewal for production-threshold transport test")
	}
	for _, mode := range []string{"standalone-runtime", "embedded", "embedded-open"} {
		t.Run(mode, func(t *testing.T) {
			url, access, _ := hostedRelay(t, mode)
			group, _ := protocol.Token()
			provider, src := newInstance(t, 18390)
			consumer, _ := newInstance(t, 18391)
			const total = int64(3<<30) + (1 << 20)
			src.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/ws" {
					c, b, e := w.(http.Hijacker).Hijack()
					if e != nil {
						return
					}
					defer c.Close()
					fmt.Fprint(b, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
					b.Flush()
					io.Copy(c, c)
					return
				}
				w.Header().Set("Content-Length", fmt.Sprint(total))
				io.CopyN(w, zeroStream{}, total)
			})
			source := save(t, provider, feature.Route{Module: "test_source", Domain: "bulk.localhost", Settings: json.RawMessage(`{}`)})
			provider.settings(t, url, access, group, true)
			consumer.settings(t, url, access, group, false)
			waitPeerCatalogs(t, provider, consumer)
			offer := waitOffer(t, consumer, source.ID)
			ref, _ := json.Marshal(cloud.ImportSettings{Import: protocol.Import{Group: offer.Group, DeviceID: offer.DeviceID, ServiceID: source.ID, ProviderKey: offer.PublicKey}})
			save(t, consumer, feature.Route{Module: "ports_over_relay", Domain: "import.localhost", Settings: ref})
			gateway := httptest.NewServer(consumer.web)
			defer gateway.Close()
			conn, e := net.Dial("tcp", strings.TrimPrefix(gateway.URL, "http://"))
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(4 * time.Minute))
			fmt.Fprint(conn, "GET /ws HTTP/1.1\r\nHost: import.localhost:18391\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
			reader := bufio.NewReader(conn)
			response, e := http.ReadResponse(reader, nil)
			if e != nil || response.StatusCode != 101 {
				t.Fatal("upgrade", e, response)
			}
			ping := func() error {
				if _, e := conn.Write([]byte("ping")); e != nil {
					return e
				}
				var got [4]byte
				if _, e := io.ReadFull(reader, got[:]); e != nil {
					return e
				}
				if string(got[:]) != "ping" {
					return fmt.Errorf("WebSocket corruption")
				}
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "GET", gateway.URL+"/bulk", nil)
			req.Host = "import.localhost:18391"
			started := time.Now()
			response, e = gateway.Client().Do(req)
			if e != nil {
				t.Fatal(e)
			}
			defer response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatal(response.Status)
			}
			transfer := make(chan error, 1)
			go func() {
				var v verifyZeros
				_, e := io.CopyBuffer(&v, response.Body, make([]byte, 128<<10))
				if e == nil && v.total != total {
					e = fmt.Errorf("short stream: %d", v.total)
				}
				transfer <- e
			}()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case e := <-transfer:
					if e != nil {
						t.Fatal(e)
					}
					if e = ping(); e != nil {
						t.Fatal("WebSocket did not survive renewal", e)
					}
					t.Logf("%.2f GiB HTTP stream and unchanged WebSocket in %s", float64(total)/(1<<30), time.Since(started))
					return
				case <-ticker.C:
					if e := ping(); e != nil {
						t.Fatal("WebSocket during renewal", e)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
		})
	}
}
