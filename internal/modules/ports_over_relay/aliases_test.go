package ports_over_relay

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zelport/zelport-sdk-go/client"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/feature"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostnameAndAliasOwnership(t *testing.T) {
	for in, want := range map[string]string{"desktop.example.test": "desktop-example-test", "UPPER.local": "upper-local", "---": "", "a_b": "a-b", strings.Repeat("a", 64): "", "12345678-1234-1234-1234-123456789012": ""} {
		if got := HostnameLabel(in); got != want {
			t.Fatalf("%q -> %q", in, got)
		}
	}
	s := Settings{}
	group := strings.Repeat("a", 64)
	r := protocol.Record{Group: group, DeviceID: protocol.ID(), Hostname: "same.host", PublicKey: protocol.RandomNonce()}
	r.Services = []protocol.Service{{Kind: protocol.Administration, ID: protocol.ID(), Domain: r.DeviceID + ".zelport.localhost"}}
	if e := rememberPeer(&s, r); e != nil {
		t.Fatal(e)
	}
	first := r.DeviceID
	r.DeviceID = protocol.ID()
	r.PublicKey = protocol.RandomNonce()
	r.Services[0].Domain = r.DeviceID + ".zelport.localhost"
	if e := rememberPeer(&s, r); e != nil {
		t.Fatal(e)
	}
	if owners := aliasOwners(s, group, "same-host"); len(owners) != 2 {
		t.Fatal(owners)
	}
	r.Hostname = "renamed"
	if e := rememberPeer(&s, r); e != nil {
		t.Fatal(e)
	}
	if len(aliasOwners(s, group, "same-host")) != 2 {
		t.Fatal("old alias lost ownership")
	}
	r.PublicKey = protocol.RandomNonce()
	if e := rememberPeer(&s, r); e == nil {
		t.Fatal("key substitution accepted")
	}
	if len(aliasOwners(s, group, first)) != 1 {
		t.Fatal("UUID alias missing")
	}
	if len(aliasOwners(s, strings.Repeat("b", 64), first)) != 0 {
		t.Fatal("cross group alias")
	}
}

func TestAliasPersistenceAndUnavailable(t *testing.T) {
	s, _ := newIdentity()
	s.Enabled = true
	r := protocol.Record{Group: groupID(s), DeviceID: protocol.ID(), Hostname: "host.local", PublicKey: protocol.RandomNonce(), Port: 9000, ExpiresAt: time.Now().Unix() + 60}
	r.Services = []protocol.Service{{Kind: protocol.Administration, ID: protocol.ID(), Domain: r.DeviceID + ".zelport.localhost"}}
	if e := rememberPeer(&s, r); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(s)
	var restored Settings
	json.Unmarshal(raw, &restored)
	target, e := resolveAdmin(restored, client.Status{Connected: true, Records: []protocol.Record{r}}, "host-local")
	if e != nil || !target.Available || target.DeviceID != r.DeviceID {
		t.Fatal(target, e)
	}
	target, e = resolveAdmin(restored, client.Status{}, "host-local")
	if e != nil || target.Available {
		t.Fatal("offline alias lost")
	}
	restored.Enabled = false
	target, e = resolveAdmin(restored, client.Status{Connected: true, Records: []protocol.Record{r}}, r.DeviceID)
	if e != nil || target.Available {
		t.Fatal("disabled alias active")
	}
	s.Peers = map[string]PeerIdentity{}
	for i := 0; i < maxPeers; i++ {
		s.Peers[fmt.Sprint(i)] = PeerIdentity{}
	}
	if e := rememberPeer(&s, r); e == nil {
		t.Fatal("peer bound ignored")
	}
	s.Peers = map[string]PeerIdentity{"other": {Aliases: make([]string, maxAliases)}}
	if e := rememberPeer(&s, r); e == nil {
		t.Fatal("alias bound ignored")
	}
}

func TestPeerChangesAreEffectiveCommits(t *testing.T) {
	m, _ := testModule(t)
	identity, _ := newIdentity()
	if err := m.store.update(context.Background(), feature.CommitOptions{}, func(settings *Settings) error { *settings = identity; return nil }); err != nil {
		t.Fatal(err)
	}
	r := protocol.Record{Version: 1, Group: strings.Repeat("a", 64), DeviceID: protocol.ID(), PublicKey: protocol.RandomNonce(), Hostname: "first", Sequence: 1}
	r.Services = []protocol.Service{{Kind: protocol.Administration, ID: protocol.ID(), Domain: r.DeviceID + ".zelport.localhost"}}
	before := m.Host.Snapshot().EditRevision
	if e := m.acceptRecord(r, []byte("first")); e != nil {
		t.Fatal(e)
	}
	after := m.Host.Snapshot().EditRevision
	if after <= before {
		t.Fatal("peer pin did not advance effective revision")
	}
	r.Sequence++
	if e := m.acceptRecord(r, []byte("repeat")); e != nil {
		t.Fatal(e)
	}
	if m.Host.Snapshot().EditRevision != after {
		t.Fatal("watermark advanced effective revision")
	}
	r.Sequence++
	r.Hostname = "renamed"
	if e := m.acceptRecord(r, []byte("rename")); e != nil {
		t.Fatal(e)
	}
	if m.Host.Snapshot().EditRevision <= after {
		t.Fatal("alias change did not advance effective revision")
	}
}

func TestPeerPersistenceFailureAndRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	mod := New().(*Module)
	manager, e := app.New(path, 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	if e = manager.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	defer manager.Close()
	identity, _ := newIdentity()
	if e = mod.store.update(context.Background(), feature.CommitOptions{}, func(settings *Settings) error { *settings = identity; return nil }); e != nil {
		t.Fatal(e)
	}
	r := protocol.Record{Version: 1, Group: strings.Repeat("a", 64), DeviceID: protocol.ID(), PublicKey: protocol.RandomNonce(), Hostname: "original", Sequence: 1}
	r.Services = []protocol.Service{{Kind: protocol.Administration, ID: protocol.ID(), Domain: r.DeviceID + ".zelport.localhost"}}
	if e = mod.acceptRecord(r, []byte("one")); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(path, path+".saved"); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	r.Sequence++
	r.Hostname = "renamed"
	if e = mod.acceptRecord(r, []byte("two")); e == nil {
		t.Fatal("failed persistence accepted")
	}
	s, _ := mod.store.snapshot()
	if len(aliasOwners(s, r.Group, "renamed")) != 0 {
		t.Fatal("unpersisted alias activated")
	}
	os.Remove(path)
	os.Rename(path+".saved", path)
	if e = mod.acceptRecord(r, []byte("two")); e != nil {
		t.Fatal(e)
	}
	manager.Close()
	restored := New().(*Module)
	reopened, e := app.New(path, 8080, restored)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if e = reopened.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	s, _ = restored.store.snapshot()
	if len(aliasOwners(s, r.Group, "original")) != 1 || len(aliasOwners(s, r.Group, "renamed")) != 1 {
		t.Fatal("restart lost alias claims")
	}
	r.DeviceID = protocol.ID()
	r.PublicKey = protocol.RandomNonce()
	r.Services[0].ID = protocol.ID()
	r.Services[0].Domain = r.DeviceID + ".zelport.localhost"
	if e = restored.acceptRecord(r, []byte("three")); e != nil {
		t.Fatal(e)
	}
	s, _ = restored.store.snapshot()
	if len(aliasOwners(s, r.Group, "renamed")) != 2 {
		t.Fatal("collision claim lost")
	}
}
