package integration

import (
	"encoding/json"
	"fmt"
	relayserver "github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport-sdk-go/client"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	relaymodule "github.com/zelport/zelport/internal/modules/ports_over_relay"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRemoteAdministrationWithPublishingDisabled(t *testing.T) {
	for _, mode := range []string{"standalone-runtime", "embedded", "embedded-open"} {
		t.Run(mode, func(t *testing.T) {
			url, access, _ := hostedRelay(t, mode)
			testRemoteAdministrationWithPublishingDisabled(t, url, access)
		})
	}
}
func testRemoteAdministrationWithPublishingDisabled(t *testing.T, url, access string) {
	group, _ := protocol.Token()
	provider, _ := newInstance(t, 18090)
	consumer, _ := newInstance(t, 18091)
	provider.m.SetAdministrationHandler(provider.web.ServeRemoteAdministration)
	consumer.m.SetAdministrationHandler(consumer.web.ServeRemoteAdministration)
	provider.settings(t, url, access, group, false)
	consumer.settings(t, url, access, group, false)
	waitPeerCatalogs(t, provider, consumer)
	var providerID string
	var st struct {
		Status   client.Status `json:"status"`
		DeviceID string        `json:"device_id"`
	}
	json.Unmarshal(provider.mod.State().Data, &st)
	providerID = st.DeviceID
	until := time.Now().Add(8 * time.Second)
	host := providerID + ".zelport.localhost:18091"
	var state struct {
		CSRF     string `json:"csrf"`
		Revision uint64 `json:"revision"`
		Port     int    `json:"port"`
	}
	for {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "http://"+host+"/api/state", nil)
		consumer.web.ServeHTTP(w, r)
		if w.Code == 200 {
			if e := json.Unmarshal(w.Body.Bytes(), &state); e != nil {
				t.Fatal(e)
			}
			break
		}
		if time.Now().After(until) {
			t.Fatal(w.Code, w.Body.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state.Port != 18090 || state.CSRF == "" {
		t.Fatal("wrong provider state")
	}
	body := fmt.Sprintf(`{"revision":%d,"route":{"module":"test_source","domain":"remote-created.localhost","settings":{}}}`, state.Revision)
	for _, origin := range []string{"", "http://evil.test"} {
		r := httptest.NewRequest("POST", "http://"+host+"/api/routes", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", state.CSRF)
		w := httptest.NewRecorder()
		consumer.web.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("bad origin", w.Code)
		}
	}
	duplicate := httptest.NewRequest("POST", "http://"+host+"/api/routes", strings.NewReader(body))
	duplicate.Header.Add("Origin", "http://"+host)
	duplicate.Header.Add("Origin", "http://"+host)
	duplicate.Header.Set("X-CSRF-Token", state.CSRF)
	rejected := httptest.NewRecorder()
	consumer.web.ServeHTTP(rejected, duplicate)
	if rejected.Code != 403 {
		t.Fatal("duplicate origin accepted", rejected.Code)
	}
	r := httptest.NewRequest("POST", "http://"+host+"/api/routes", strings.NewReader(body))
	r.Header.Set("Origin", "http://"+host)
	r.Header.Set("X-CSRF-Token", state.CSRF)
	w := httptest.NewRecorder()
	consumer.web.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	_, pr := provider.m.Snapshot()
	_, cr := consumer.m.Snapshot()
	if len(pr) != 1 || len(cr) != 0 {
		t.Fatal("mutation reached wrong instance")
	}

	provider.settings(t, url, access, group, true)
	offer := waitOffer(t, consumer, pr[0].ID)
	ref, _ := json.Marshal(relaymodule.ImportSettings{Import: protocol.Import{Group: offer.Group, DeviceID: offer.DeviceID, ProviderKey: offer.PublicKey, ServiceID: pr[0].ID}})
	save(t, consumer, feature.Route{Module: "ports_over_relay", Domain: "receiver-name.localhost", Settings: ref})
	response := httptest.NewRecorder()
	consumer.web.ServeHTTP(response, httptest.NewRequest("GET", "http://"+host+"/api/state", nil))
	var mapped struct {
		Remote feature.RemoteAdminContext `json:"remote_admin"`
	}
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if e := json.Unmarshal(response.Body.Bytes(), &mapped); e != nil {
		t.Fatal(e)
	}
	if mapped.Remote.ServiceLinks[pr[0].ID] != "http://receiver-name.localhost:18091" {
		t.Fatal("incorrect receiving alias", mapped.Remote.ServiceLinks)
	}
}

func TestRemoteAdministrationAliasesAndRevocation(t *testing.T) {
	access, _ := protocol.Token()
	token, _ := protocol.DecodeKey(access)
	group, _ := protocol.Token()
	relayServer, _ := relayserver.New(relayserver.Options{AccessToken: token})
	rs := httptest.NewServer(relayServer)
	defer rs.Close()
	defer relayServer.Close()
	provider, _ := newInstance(t, 18190)
	consumer, _ := newInstance(t, 18191)
	provider.m.SetAdministrationHandler(provider.web.ServeRemoteAdministration)
	provider.settings(t, "ws"+rs.URL[4:], access, group, false)
	consumer.settings(t, "ws"+rs.URL[4:], access, group, false)
	var identity struct {
		DeviceID string `json:"device_id"`
	}
	json.Unmarshal(provider.mod.State().Data, &identity)
	host := identity.DeviceID + ".zelport.localhost:18191"
	get := func(host, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		consumer.web.ServeHTTP(w, httptest.NewRequest("GET", "http://"+host+path, nil))
		return w
	}
	until := time.Now().Add(8 * time.Second)
	for get(host, "/api/state").Code != 200 {
		if time.Now().After(until) {
			t.Fatal("admin unavailable")
		}
		time.Sleep(10 * time.Millisecond)
	}
	hostname, _ := os.Hostname()
	alias := relaymodule.HostnameLabel(hostname) + ".zelport.localhost:18191"
	for _, path := range []string{"/", "/app.js", "/api/state", "/assets/modules/ports_over_relay/module.js"} {
		if w := get(alias, path); w.Code != 200 {
			t.Fatal(path, w.Code, w.Body)
		}
	}
	for _, bad := range []string{identity.DeviceID + ".zelport.localhost:9999", "unknown.zelport.localhost:18191"} {
		if get(bad, "/api/state").Code != 404 {
			t.Fatal("bad alias accepted")
		}
	}
	// Publishing changes must not revoke a previously issued administration token.
	var state struct {
		CSRF     string `json:"csrf"`
		Revision uint64 `json:"revision"`
	}
	json.Unmarshal(get(alias, "/api/state").Body.Bytes(), &state)
	provider.settings(t, "ws"+rs.URL[4:], access, group, true)
	// Reuse the issued token after a publication change, using the current revision.
	var current struct {
		Revision uint64 `json:"revision"`
	}
	json.Unmarshal(get(alias, "/api/state").Body.Bytes(), &current)
	body := fmt.Sprintf(`{"revision":%d,"enabled":false,"publish":false,"relay_url":%q}`, current.Revision, "ws"+rs.URL[4:])
	req := httptest.NewRequest("PUT", "http://"+alias+"/api/modules/ports_over_relay/settings", strings.NewReader(body))
	req.Header.Set("Origin", "http://"+alias)
	req.Header.Set("X-CSRF-Token", state.CSRF)
	response := httptest.NewRecorder()
	consumer.web.ServeHTTP(response, req)
	// Disconnecting the current tunnel may lose the response; the commit must survive.
	raw, err := os.ReadFile(provider.path)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Modules map[string]json.RawMessage `json:"modules"`
	}
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	var settings relaymodule.Settings
	json.Unmarshal(saved.Modules["ports_over_relay"], &settings)
	if settings.Enabled {
		t.Fatalf("remote disable was not durable: %d %s", response.Code, response.Body.String())
	}

	if get(host, "/api/state").Code != 503 {
		t.Fatal("disabled relay administration reachable")
	}
}
