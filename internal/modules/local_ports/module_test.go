package local_ports

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestLocalApplicationResponse(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "demo.localhost:8080" || r.URL.RequestURI() != "/x?q=1" {
			t.Errorf("request: %s %s", r.Host, r.URL)
		}
		w.WriteHeader(403)
		io.WriteString(w, "application response")
	}))
	defer backend.Close()
	_, port, _ := net.SplitHostPort(backend.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	m, e := app.New("", 8080, New())
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	b, _ := json.Marshal(map[string]int{"aux_port": n})
	_, e = m.Save(context.Background(), 0, feature.Route{Module: "local_ports", Domain: "demo.localhost", Settings: b})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "http://demo.localhost:8080/x?q=1", nil)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 403 || w.Body.String() != "application response" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
