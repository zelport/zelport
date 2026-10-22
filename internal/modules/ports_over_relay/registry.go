package ports_over_relay

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/zelport/zelport-sdk-go/client"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport-sdk-go/secure"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

func (m *Module) clientOptions(s Settings) (client.Options, error) {
	var access []byte
	if s.AccessToken != "" {
		var e error
		access, e = protocol.DecodeKey(s.AccessToken)
		if e != nil {
			return client.Options{}, e
		}
	}
	group, e := protocol.DecodeKey(s.GroupToken)
	if e != nil {
		return client.Options{}, e
	}
	key, e := protocol.DecodeKey(s.PrivateKey)
	if e != nil {
		return client.Options{}, e
	}
	return client.Options{RelayURL: s.RelayURL, AccessToken: access, GroupToken: group, Identity: protocol.Identity{ID: s.DeviceID, PrivateKey: key}, Registry: m, AcceptRecord: m.acceptRecord}, nil
}
func (m *Module) Catalog(ctx context.Context) (protocol.Record, error) {
	var record protocol.Record
	e := m.store.update(ctx, feature.CommitOptions{MetadataOnly: true}, func(s *Settings) error {
		if !s.Enabled {
			return errors.New("relay disabled")
		}
		if s.Sequence == math.MaxUint64 {
			return errors.New("catalog sequence exhausted")
		}
		s.Sequence++
		private, e := protocol.DecodeKey(s.PrivateKey)
		if e != nil {
			return e
		}
		pub, e := secure.Public(private)
		if e != nil {
			return e
		}
		host, _ := os.Hostname()
		if host == "" {
			host = "device"
		}
		_, port, _ := net.SplitHostPort(m.Host.ListenerAuthority("localhost"))
		listenerPort := 80
		if port != "" {
			fmt.Sscan(port, &listenerPort)
		}
		record = protocol.Record{Version: protocol.Version, DeviceID: s.DeviceID, Hostname: host, PublicKey: base64.RawURLEncoding.EncodeToString(pub), Sequence: s.Sequence, IssuedAt: time.Now().Unix(), Port: listenerPort, Publish: s.Publish, Services: []protocol.Service{}}
		record.ExpiresAt = record.IssuedAt + 60
		group, _ := protocol.DecodeKey(s.GroupToken)
		locator, _ := secure.Derive(group, "group-locator")
		record.Group = hex.EncodeToString(locator[:])
		if !protocol.ValidID(s.AdminServiceID) {
			return errors.New("administration identity is missing")
		}
		record.Services = append(record.Services, protocol.Service{Kind: protocol.Administration, ID: s.AdminServiceID, Domain: s.DeviceID + ".zelport.localhost"})
		if s.Publish {
			for _, r := range m.Host.OwnServices() {
				if r.ID == s.AdminServiceID {
					return errors.New("administration service identity collision")
				}
				record.Services = append(record.Services, protocol.Service{Kind: protocol.Application, ID: r.ID, Domain: r.Domain})
			}
		}
		if len(record.Services) > protocol.MaxServices {
			return errors.New("too many services to publish")
		}
		return nil
	})
	return record, e
}
func (m *Module) acceptRecord(r protocol.Record, raw []byte) error {
	if r.Version != protocol.Version {
		return fmt.Errorf("Unsupported relay catalog version %d. Create a fresh Ports over Relay configuration.", r.Version)
	}
	key := r.Group + "/" + r.DeviceID
	digest := protocol.Digest(raw)
	return m.store.updateClassified(context.Background(), feature.CommitOptions{}, true, func(s *Settings) error {
		if s.Watermarks == nil {
			s.Watermarks = map[string]Watermark{}
		}
		old, ok := s.Watermarks[key]
		if ok && (r.Sequence < old.Sequence || r.Sequence == old.Sequence && (digest != old.Digest || old.DigestVersion != protocol.Version)) {
			return errors.New("catalog replay or rolled-back provider configuration")
		}
		for _, route := range m.Host.Snapshot().Routes {
			var ref ImportSettings
			feature.Decode(route.Settings, &ref)
			if ref.Group == r.Group && ref.DeviceID == r.DeviceID && ref.ProviderKey != r.PublicKey {
				return errors.New("provider identity changed")
			}
		}
		if !ok && len(s.Watermarks) >= 1024 {
			return errors.New("catalog identity history limit")
		}
		if e := rememberPeer(s, r); e != nil {
			return e
		}
		s.Watermarks[key] = Watermark{Sequence: r.Sequence, Digest: digest, DigestVersion: protocol.Version}
		return nil
	})
}

type leaseConn struct {
	net.Conn
	once  sync.Once
	lease *feature.Lease
}

func (c *leaseConn) Close() error {
	e := c.Conn.Close()
	c.once.Do(func() { c.lease.Close() })
	return e
}

type oneListener struct {
	conn net.Conn
	ctx  context.Context
	once sync.Once
}

func (l *oneListener) Accept() (net.Conn, error) {
	var c net.Conn
	l.once.Do(func() { c = l.conn })
	if c != nil {
		return c, nil
	}
	<-l.ctx.Done()
	return nil, net.ErrClosed
}
func (l *oneListener) Close() error   { return nil }
func (l *oneListener) Addr() net.Addr { return l.conn.LocalAddr() }
func (m *Module) Open(ctx context.Context, peer protocol.PeerContext, o protocol.OpenRequest) (net.Conn, error) {
	if e := o.Validate(); e != nil {
		return nil, e
	}
	if o.Kind == protocol.Administration {
		return m.openAdministration(ctx, peer, o)
	}
	s, rev := m.store.snapshot()
	if !s.Enabled || !s.Publish {
		return nil, errors.New("sharing disabled")
	}
	sharing := m.Host.Scope("sharing")
	l, e := m.Host.Acquire(ctx, o.ServiceID, rev, sharing)
	if e != nil {
		return nil, e
	}
	route := l.Route()
	if m.Host.ListenerAuthority(route.Domain) != o.Authority {
		l.Close()
		return nil, errors.New("The service authority changed. Refresh the relay catalog.")
	}
	a, b := net.Pipe()
	if _, e = l.Track(a); e != nil {
		b.Close()
		return nil, e
	}
	if _, e = l.Track(b); e != nil {
		a.Close()
		return nil, e
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 32 << 10, BaseContext: func(net.Listener) context.Context { return l.Context() }, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != o.Authority {
			feature.Status(w, r, 503, "Service changed", "Refresh the relay catalog.")
			return
		}
		traces := r.Header.Values(protocol.TraceHeader)
		if len(traces) > 1 || r.Header.Get(protocol.TraceHeader) != o.Trace {
			feature.Status(w, r, 508, "Relay trace rejected", "Conflicting traversal metadata.")
			return
		}
		m.Host.Dispatch(w, r, l)
	}), ConnState: func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			l.Close()
		}
	}}
	server.SetKeepAlivesEnabled(false)
	go server.Serve(&oneListener{conn: b, ctx: l.Context()})
	return &leaseConn{Conn: a, lease: l}, nil
}
func (m *Module) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := feature.RequestLease(r)
		if l == nil || l.Route().Module == "ports_over_relay" {
			next.ServeHTTP(w, r)
			return
		}
		s, _ := m.store.snapshot()
		if s.GroupToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		key, e := protocol.DecodeKey(s.GroupToken)
		if e != nil {
			feature.Status(w, r, 508, "Relay trace rejected", "Invalid local group configuration.")
			return
		}
		values := r.Header.Values(protocol.TraceHeader)
		if len(values) > 1 {
			feature.Status(w, r, 508, "Relay trace rejected", "Conflicting traversal metadata.")
			return
		}
		trace, e := protocol.AdvanceTrace(r.Header.Get(protocol.TraceHeader), key, s.DeviceID, l.Route().ID, time.Now())
		if e != nil {
			feature.StatusDiagnostic(w, r, 508, "Relay loop detected", "The relay request could not be completed.", e.Error())
			return
		}
		r = r.Clone(r.Context())
		r.Header.Set(protocol.TraceHeader, trace)
		next.ServeHTTP(w, r)
	})
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, route feature.Route) {
	var ref ImportSettings
	feature.Decode(route.Settings, &ref)
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()
	if c == nil || m.validateOffer(ref) != nil {
		feature.Status(w, r, 503, "Relay service unavailable", "The import is preserved. Reconnect or refresh the relay catalog.")
		return
	}
	var authority string
	for _, v := range c.Snapshot().Records {
		if v.DeviceID == ref.DeviceID {
			authority, _ = v.Authority(ref.ServiceID)
		}
	}
	modulekit.Proxy(w, r, &url.URL{Scheme: "http", Host: authority}, modulekit.ProxyOptions{RewriteOrigin: ref.RewriteOrigin, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return c.Open(ctx, ref.Import, protocol.OpenRequest{Kind: protocol.Application, ServiceID: ref.ServiceID, Authority: authority, Trace: r.Header.Get(protocol.TraceHeader)})
	}, Error: func(w http.ResponseWriter, r *http.Request, e error) {
		feature.StatusDiagnostic(w, r, 503, "Relay service unavailable", "The relay connection could not be established.", e.Error())
	}})
}
