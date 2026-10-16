package ports_over_relay

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"io"
	"net/http"
	"testing"
)

type source struct{ modulekit.Base }

func (s *source) ValidateRoute(feature.Route) error { return nil }
func (s *source) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	w.WriteHeader(403)
	io.WriteString(w, r.Host+r.URL.RequestURI())
}
func TestCloudDispatchAndRevocation(t *testing.T) {
	mod := New().(*Module)
	src := &source{modulekit.Base{Desc: feature.Descriptor{Maturity: feature.Stable, ID: "source", TabID: "source", Name: "Source", Exportable: true}}}
	m, e := app.New("", 8080, src, mod)
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	s, _ := newIdentity()
	s.Publish = true
	s.Enabled = true
	s.AccessToken, _ = protocol.Token()
	s.RelayURL = "ws://127.0.0.1:1"
	if e = mod.store.update(context.Background(), feature.CommitOptions{}, func(v *Settings) error { *v = s; return nil }); e != nil {
		t.Fatal(e)
	}
	rev, _ := m.Snapshot()
	route, e := m.Save(context.Background(), rev, feature.Route{Module: "source", Domain: "demo.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	record, e := mod.Catalog(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(record.Services) != 2 || record.Services[1].ID != route.ID {
		t.Fatal(record)
	}
	conn, e := mod.Open(context.Background(), protocol.PeerContext{}, protocol.OpenRequest{Kind: protocol.Application, ServiceID: route.ID, Authority: "demo.localhost:8080"})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	if e = mod.store.update(context.Background(), feature.CommitOptions{ResetScopes: []string{"sharing"}}, func(v *Settings) error { v.Publish = false; return nil }); e != nil {
		t.Fatal(e)
	}
	if _, e = conn.Write([]byte("GET / HTTP/1.1\r\n\r\n")); e == nil {
		t.Fatal("revoked stream survived")
	}
	if c, e := mod.Open(context.Background(), protocol.PeerContext{}, protocol.OpenRequest{Kind: protocol.Application, ServiceID: route.ID, Authority: "demo.localhost:8080"}); e == nil {
		c.Close()
		t.Fatal("unpublished service opened")
	}
}
