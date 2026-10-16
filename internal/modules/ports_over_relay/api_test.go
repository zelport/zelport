package ports_over_relay

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudCredentialsExplicitAndRedacted(t *testing.T) {
	mod, _ := testModule(t)
	request := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mod.Admin().ServeHTTP(w, httptest.NewRequest("POST", "/credentials", strings.NewReader(body)))
		return w
	}
	w := request(`{"revision":0,"action":"generate"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	if len(v["group_token"]) != 43 {
		t.Fatal("invalid generated token")
	}
	w = request(`{"revision":0,"action":"reveal"}`)
	if w.Code != 409 {
		t.Fatal("stale request accepted")
	}
	w = httptest.NewRecorder()
	mod.Admin().ServeHTTP(w, httptest.NewRequest("GET", "/settings", nil))
	if strings.Contains(w.Body.String(), v["group_token"]) {
		t.Fatal("GET leaked token")
	}
	s, _ := mod.store.snapshot()
	if s.Enabled || s.Publish {
		t.Fatal("setup enabled sharing")
	}
}
func TestCloudCatalogWatermarkRejectsRollback(t *testing.T) {
	mod, _ := testModule(t)
	s, _ := newIdentity()
	mod.store.update(context.Background(), feature.CommitOptions{}, func(v *Settings) error { *v = s; return nil })
	r := protocol.Record{Group: strings.Repeat("a", 64), DeviceID: protocol.ID(), Sequence: 2}
	if e := mod.acceptRecord(r, []byte("record-two")); e != nil {
		t.Fatal(e)
	}
	r.Sequence = 1
	if e := mod.acceptRecord(r, []byte("record-one")); e == nil {
		t.Fatal("accepted rollback")
	}
	r.Sequence = 2
	if e := mod.acceptRecord(r, []byte("conflict")); e == nil {
		t.Fatal("accepted equal sequence conflict")
	}
	if e := mod.acceptRecord(r, []byte("record-two")); e != nil {
		t.Fatal(e)
	}
}

func TestCloudSettingsSaveAcrossMetadataUpdates(t *testing.T) {
	mod, _ := testModule(t)
	identity, _ := newIdentity()
	if e := mod.store.update(context.Background(), feature.CommitOptions{}, func(s *Settings) error { *s = identity; return nil }); e != nil {
		t.Fatal(e)
	}
	_, opened := mod.store.snapshot()
	if e := mod.store.update(context.Background(), feature.CommitOptions{MetadataOnly: true}, func(s *Settings) error { s.Sequence = 42; return nil }); e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	mod.Admin().ServeHTTP(w, httptest.NewRequest("PUT", "/settings", strings.NewReader(fmt.Sprintf(`{"revision":%d,"publish":true}`, opened))))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, _ := mod.store.snapshot()
	if saved.Sequence != 42 || !saved.Publish {
		t.Fatal("lost counter or user change")
	}
	w = httptest.NewRecorder()
	mod.Admin().ServeHTTP(w, httptest.NewRequest("PUT", "/settings", strings.NewReader(fmt.Sprintf(`{"revision":%d,"publish":false}`, opened))))
	if w.Code != 409 {
		t.Fatal("concurrent user change overwritten")
	}
}

func TestEmptyAccessTokenAndExplicitClearing(t *testing.T) {
	mod, _ := testModule(t)
	token, _ := protocol.Token()
	request := func(extra map[string]any) *httptest.ResponseRecorder {
		_, rev := mod.store.snapshot()
		extra["revision"] = rev
		extra["relay_url"] = "ws://127.0.0.1:8090"
		b, _ := json.Marshal(extra)
		w := httptest.NewRecorder()
		mod.Admin().ServeHTTP(w, httptest.NewRequest("PUT", "/settings", strings.NewReader(string(b))))
		return w
	}
	for _, extra := range []map[string]any{{"access_token": token}, {"access_token": ""}} {
		if w := request(extra); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	s, _ := mod.store.snapshot()
	if s.AccessToken != token {
		t.Fatal("blank field erased stored token")
	}
	if w := request(map[string]any{"clear_access_token": true, "access_token": token}); w.Code != 400 {
		t.Fatal("ambiguous request accepted")
	}
	if w := request(map[string]any{"clear_access_token": true}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = mod.store.snapshot()
	if s.AccessToken != "" {
		t.Fatal("token not cleared")
	}
	s.Enabled = true
	b, _ := json.Marshal(s)
	if e := mod.ValidateSettings(b); e != nil {
		t.Fatal(e)
	}
	opts, e := mod.clientOptions(s)
	if e != nil || len(opts.AccessToken) != 0 || len(opts.GroupToken) != 32 {
		t.Fatal("empty token client options invalid", e)
	}
}

func TestLegacyWatermarkFloor(t *testing.T) {
	mod, _ := testModule(t)
	s, _ := newIdentity()
	r := protocol.Record{Version: 3, Group: strings.Repeat("a", 64), DeviceID: protocol.ID(), Sequence: 2}
	raw := []byte("canonical")
	s.Watermarks = map[string]Watermark{r.Group + "/" + r.DeviceID: {Sequence: 2, Digest: protocol.Digest(raw)}}
	if e := mod.store.update(context.Background(), feature.CommitOptions{}, func(v *Settings) error { *v = s; return nil }); e != nil {
		t.Fatal(e)
	}
	if e := mod.acceptRecordVersion(r, raw, 3); e == nil {
		t.Fatal("legacy equal floor accepted")
	}
	r.Sequence++
	if e := mod.acceptRecordVersion(r, raw, 3); e != nil {
		t.Fatal(e)
	}
	saved, _ := mod.store.snapshot()
	if saved.Watermarks[r.Group+"/"+r.DeviceID].DigestVersion != 3 {
		t.Fatal("missing semantic version")
	}
	if e := mod.acceptRecordVersion(r, raw, 3); e != nil {
		t.Fatal("idempotence", e)
	}
	if e := mod.acceptRecordVersion(r, []byte("changed"), 3); e == nil {
		t.Fatal("content substitution")
	}
}

func TestCatalogSemanticWatermarkIdempotentRevision(t *testing.T) {
	mod, _ := testModule(t)
	r := protocol.Record{Version: 3, Group: strings.Repeat("a", 64), DeviceID: protocol.ID(), Sequence: 2}
	if e := mod.acceptRecord(r, []byte("same")); e != nil {
		t.Fatal(e)
	}
	_, before := mod.store.snapshot()
	if e := mod.acceptRecord(r, []byte("same")); e != nil {
		t.Fatal(e)
	}
	_, after := mod.store.snapshot()
	if before != after {
		t.Fatal("idempotent catalog changed revision", before, after)
	}
}
