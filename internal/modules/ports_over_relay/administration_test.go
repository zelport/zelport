package ports_over_relay

import (
	"bufio"
	"context"
	"encoding/base64"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport-sdk-go/secure"
	"github.com/zelport/zelport/internal/feature"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAdminStreamScopeAndAuthorization(t *testing.T) {
	mod, manager := testModule(t)
	s, _ := newIdentity()
	s.Enabled = true
	s.Publish = true
	s.AccessToken, _ = protocol.Token()
	s.RelayURL = "ws://127.0.0.1:1"
	if e := mod.store.update(context.Background(), feature.CommitOptions{}, func(v *Settings) error { *v = s; return nil }); e != nil {
		t.Fatal(e)
	}
	private, _ := protocol.DecodeKey(s.PrivateKey)
	pub, _ := secure.Public(private)
	peer := protocol.PeerContext{Version: protocol.Version, Group: groupID(s), Consumer: protocol.ID(), ConsumerKey: protocol.RandomNonce(), Provider: s.DeviceID, ProviderKey: base64.RawURLEncoding.EncodeToString(pub)}
	authority := s.DeviceID + ".zelport.localhost:8080"
	open := protocol.OpenRequest{Kind: protocol.Administration, ServiceID: s.AdminServiceID, Authority: authority, Admin: &protocol.AdminOpen{Authority: authority, ServiceLinks: []protocol.ServiceLink{}}}
	for _, change := range []func(*protocol.PeerContext, *protocol.OpenRequest){func(p *protocol.PeerContext, o *protocol.OpenRequest) { p.Group = strings.Repeat("f", 64) }, func(p *protocol.PeerContext, o *protocol.OpenRequest) { o.ServiceID = protocol.ID() }, func(p *protocol.PeerContext, o *protocol.OpenRequest) { o.Kind = protocol.Application }, func(p *protocol.PeerContext, o *protocol.OpenRequest) { p.ProviderKey = protocol.RandomNonce() }, func(p *protocol.PeerContext, o *protocol.OpenRequest) { p.Consumer = s.DeviceID }} {
		p, o := peer, open
		change(&p, &o)
		if conn, e := mod.Open(context.Background(), p, o); e == nil {
			conn.Close()
			t.Fatal("unauthorized admin opened")
		}
	}
	started := make(chan struct{})
	pulse := make(chan struct{})
	manager.SetAdministrationHandler(func(w http.ResponseWriter, r *http.Request, ctx feature.RemoteAdminContext) {
		io.WriteString(w, "ready\n")
		w.(http.Flusher).Flush()
		close(started)
		select {
		case <-pulse:
			io.WriteString(w, "alive\n")
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
		<-r.Context().Done()
	})
	conn, e := mod.Open(context.Background(), peer, open)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	go io.WriteString(conn, "GET / HTTP/1.1\r\nHost: "+authority+"\r\n\r\n")
	response, e := http.ReadResponse(bufio.NewReader(conn), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	line, e := bufio.NewReader(response.Body).ReadString('\n')
	if e != nil || line != "ready\n" {
		t.Fatal(line, e)
	}
	<-started
	if e = mod.store.update(context.Background(), feature.CommitOptions{ResetScopes: []string{"sharing"}}, func(v *Settings) error { v.Publish = false; return nil }); e != nil {
		t.Fatal(e)
	}
	close(pulse)
	if line, e := bufio.NewReader(response.Body).ReadString('\n'); e != nil || line != "alive\n" {
		t.Fatal("sharing change closed active administration", line, e)
	}
	// An independent open still succeeds with ordinary sharing off.
	c, e := mod.Open(context.Background(), peer, open)
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	if e = mod.store.update(context.Background(), feature.CommitOptions{ResetScopes: []string{"client"}}, func(v *Settings) error { v.Enabled = false; return nil }); e != nil {
		t.Fatal(e)
	}
	if _, e = response.Body.Read(make([]byte, 1)); e == nil {
		t.Fatal("revoked admin stream survived")
	}
}
