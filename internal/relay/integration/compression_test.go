package integration

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	relay "github.com/zelport/zelport/internal/modules/ports_over_relay"
)

// Compression belongs to the application. Neither Zelport App instance may silently
// decode/re-encode an explicitly negotiated representation, including streaming.
func TestRelayPreservesGzipRepresentation(t *testing.T) {
	url, access, _ := hostedRelay(t, "standalone-runtime")
	group, _ := protocol.Token()
	provider, src := newInstance(t, 18490)
	consumer, _ := newInstance(t, 18491)
	plain := bytes.Repeat([]byte("compressible application response\n"), 1000)
	var encoded bytes.Buffer
	zip := gzip.NewWriter(&encoded)
	if _, err := zip.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := zip.Close(); err != nil {
		t.Fatal(err)
	}
	src.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "gzip" {
			http.Error(w, "encoding negotiation changed", 400)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("ETag", `"compressed-v1"`)
		if r.URL.Path == "/fixed" {
			w.Header().Set("Content-Length", fmt.Sprint(encoded.Len()))
		}
		half := encoded.Len() / 2
		w.Write(encoded.Bytes()[:half])
		if r.URL.Path == "/stream" {
			w.(http.Flusher).Flush()
		}
		w.Write(encoded.Bytes()[half:])
	})
	source := save(t, provider, feature.Route{Module: "test_source", Domain: "gzip-source.localhost", Settings: json.RawMessage(`{}`)})
	provider.settings(t, url, access, group, true)
	consumer.settings(t, url, access, group, false)
	waitPeerCatalogs(t, provider, consumer)
	offer := waitOffer(t, consumer, source.ID)
	settings, _ := json.Marshal(relay.ImportSettings{Import: protocol.Import{Group: offer.Group, DeviceID: offer.DeviceID, ServiceID: source.ID, ProviderKey: offer.PublicKey}})
	save(t, consumer, feature.Route{Module: "ports_over_relay", Domain: "gzip.localhost", Settings: settings})
	appServer := httptest.NewServer(consumer.web)
	defer appServer.Close()
	transport := &http.Transport{DisableCompression: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, path := range []string{"/fixed", "/stream"} {
		t.Run(path, func(t *testing.T) {
			request, _ := http.NewRequest("GET", appServer.URL+path, nil)
			request.Host = "gzip.localhost:18491"
			request.Header.Set("Accept-Encoding", "gzip")
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 || response.Header.Get("Content-Encoding") != "gzip" || response.Header.Get("Vary") != "Accept-Encoding" || response.Header.Get("ETag") != `"compressed-v1"` || response.Uncompressed {
				t.Fatalf("representation headers changed: %+v", response.Header)
			}
			if !bytes.Equal(body, encoded.Bytes()) {
				t.Fatal("compressed representation changed")
			}
			if path == "/fixed" && response.ContentLength != int64(encoded.Len()) {
				t.Fatal("content length changed")
			}
			reader, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := io.ReadAll(reader)
			reader.Close()
			if err != nil || !bytes.Equal(decoded, plain) {
				t.Fatalf("decompression failed: %v", err)
			}
		})
	}
}
