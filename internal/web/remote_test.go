package web

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modules/local_ports"
	"net/http"
	"testing"
)

func TestRemoteAdministrationOriginAndTokens(t *testing.T) {
	m, _ := app.New("", 9000, local_ports.New())
	m.Start(context.Background())
	defer m.Close()
	s, _ := New(m, 9000)
	ctx := feature.RemoteAdminContext{ConsumerID: "consumer", ProviderID: "provider", Authority: "host.zelport.localhost:8080"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.ServeRemoteAdministration(w, r, ctx) })
	w := request(handler, "GET", ctx.Authority, "/api/state", "", nil)
	var st struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(w.Body.Bytes(), &st)
	if w.Code != 200 || st.CSRF == "" || st.CSRF == s.token {
		t.Fatal("unscoped state", w.Code)
	}
	body := `{"revision":0,"route":{"module":"local_ports","domain":"foo.localhost","settings":{"aux_port":3000}}}`
	for _, h := range []map[string]string{nil, {"Origin": "http://evil.test", "X-CSRF-Token": st.CSRF}, {"Origin": "http://" + ctx.Authority, "X-CSRF-Token": s.token}} {
		if w := request(handler, "POST", ctx.Authority, "/api/routes", body, h); w.Code != 403 {
			t.Fatal("invalid mutation", w.Code)
		}
	}
	h := map[string]string{"Origin": "http://" + ctx.Authority, "X-CSRF-Token": st.CSRF}
	if w := request(handler, "POST", ctx.Authority, "/api/routes", body, h); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	ctx.ConsumerID = "other"
	if w := request(handler, "POST", ctx.Authority, "/api/routes", body, h); w.Code != 403 {
		t.Fatal("cross consumer", w.Code)
	}
	ctx.ConsumerID = "consumer"
	ctx.Authority = "other.zelport.localhost:8080"
	h["Origin"] = "http://" + ctx.Authority
	if w := request(handler, "POST", ctx.Authority, "/api/routes", body, h); w.Code != 403 {
		t.Fatal("cross alias", w.Code)
	}
	if w := request(s, "GET", ctx.Authority, "/api/state", "", map[string]string{"X-Zelport-Admin": "true"}); w.Code != 404 {
		t.Fatal("header bypass", w.Code)
	}
	rev, _ := m.Snapshot()
	if _, e := m.Save(context.Background(), rev, feature.Route{Module: "local_ports", Domain: "x.zelport.localhost", Settings: json.RawMessage(`{"aux_port":3000}`)}); e == nil {
		t.Fatal("reserved route accepted")
	}
}
