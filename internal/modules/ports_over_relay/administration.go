package ports_over_relay

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport-sdk-go/secure"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func (m *Module) ServeAdministration(w http.ResponseWriter, r *http.Request) bool {
	host, _, e := protocol.AdminAuthority(strings.ToLower(r.Host))
	if e != nil || !strings.EqualFold(r.Host, m.Host.GatewayAuthority(host)) {
		feature.Status(w, r, 404, "Unknown address", "Invalid administration authority.")
		return true
	}
	r = r.Clone(r.Context())
	r.Host = strings.ToLower(r.Host)
	target, e := m.resolveAdministration(host)
	if e != nil {
		code := 503
		if target.Hostname == "" {
			code = 404
		}
		feature.Status(w, r, code, "Administration unavailable", e.Error())
		return true
	}
	if !target.Available {
		feature.Status(w, r, 503, "Administration unavailable", "The remote Zelport is disconnected or its identity cannot be verified.")
		return true
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		origins := r.Header.Values("Origin")
		if len(origins) != 1 || origins[0] != "http://"+r.Host {
			feature.Status(w, r, 403, "Forbidden", "Request origin is invalid.")
			return true
		}
	}
	if len(r.Header.Values(protocol.TraceHeader)) > 0 {
		feature.Status(w, r, 508, "Administration loop rejected", "Administration cannot be chained through a relay service.")
		return true
	}
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()
	if c == nil {
		feature.Status(w, r, 503, "Administration unavailable", "Relay is disconnected.")
		return true
	}
	admin := &protocol.AdminOpen{Authority: r.Host, ServiceLinks: []protocol.ServiceLink{}}
	if r.Method == "GET" && r.URL.Path == "/api/state" {
		admin.ServiceLinks = m.serviceLinks(target)
	}
	modulekit.Proxy(w, r, &url.URL{Scheme: "http", Host: target.Authority}, modulekit.ProxyOptions{PreserveHost: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return c.Open(ctx, target.Import, protocol.OpenRequest{Kind: protocol.Administration, ServiceID: target.ServiceID, Authority: target.Authority, Admin: admin})
	}, Error: func(w http.ResponseWriter, r *http.Request, e error) {
		feature.Status(w, r, 503, "Administration unavailable", "The remote administration tunnel is unavailable.")
	}})
	return true
}
func (m *Module) serviceLinks(target AdminTarget) []protocol.ServiceLink {
	links := map[string]string{}
	for _, route := range m.Host.Snapshot().Routes {
		var ref ImportSettings
		if feature.Decode(route.Settings, &ref) != nil {
			continue
		}
		if ref.Group != target.Group || ref.DeviceID != target.DeviceID || ref.ProviderKey != target.ProviderKey || m.validateOffer(ref) != nil {
			continue
		}
		address := "http://" + m.Host.GatewayAuthority(route.Domain)
		if old := links[ref.ServiceID]; old == "" || address < old {
			links[ref.ServiceID] = address
		}
	}
	ids := []string{}
	for id := range links {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) > protocol.MaxServices {
		ids = ids[:protocol.MaxServices]
	}
	result := []protocol.ServiceLink{}
	for _, id := range ids {
		result = append(result, protocol.ServiceLink{ServiceID: id, URL: links[id]})
	}
	return result
}

type adminPipe struct {
	net.Conn
	other   net.Conn
	cancel  context.CancelFunc
	once    sync.Once
	mu      sync.Mutex
	closed  bool
	release func()
}

func (p *adminPipe) Close() error {
	p.once.Do(func() {
		p.mu.Lock()
		p.closed = true
		release := p.release
		p.mu.Unlock()
		p.cancel()
		p.Conn.Close()
		p.other.Close()
		if release != nil {
			release()
		}
	})
	return nil
}
func (p *adminPipe) setRelease(release func()) {
	p.mu.Lock()
	closed := p.closed
	if !closed {
		p.release = release
	}
	p.mu.Unlock()
	if closed {
		release()
	}
}

func (m *Module) openAdministration(ctx context.Context, peer protocol.PeerContext, o protocol.OpenRequest) (net.Conn, error) {
	// Retry only authorization/registration, before any HTTP request is accepted.
	for attempts := 0; attempts < 4; attempts++ {
		c, err := m.openAdministrationAttempt(ctx, peer, o)
		if !errors.Is(err, feature.ErrClosed) {
			return c, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, errors.New("administration state changed; try again")
}
func (m *Module) openAdministrationAttempt(ctx context.Context, peer protocol.PeerContext, o protocol.OpenRequest) (net.Conn, error) {

	s, revision := m.store.snapshot()
	if !s.Enabled || o.ServiceID != s.AdminServiceID || peer.Provider != s.DeviceID || peer.Consumer == s.DeviceID || !protocol.ValidID(peer.Consumer) || peer.Group != groupID(s) {
		return nil, errors.New("administration is not authorized")
	}
	private, e := protocol.DecodeKey(s.PrivateKey)
	if e != nil {
		return nil, e
	}
	public, e := secure.Public(private)
	if e != nil {
		return nil, e
	}
	if peer.ProviderKey != base64.RawURLEncoding.EncodeToString(public) {
		return nil, errors.New("provider identity changed")
	}
	if pin, ok := s.Peers[peer.Group+"/"+peer.Consumer]; ok && pin.PublicKey != peer.ConsumerKey {
		return nil, errors.New("consumer identity changed")
	}
	canonical := m.Host.GatewayAuthority(s.DeviceID + ".zelport.localhost")
	if o.Authority != canonical {
		return nil, errors.New("administration service authority mismatch")
	}
	host, _, e := protocol.AdminAuthority(o.Admin.Authority)
	if e != nil {
		return nil, e
	}
	hostname, _ := os.Hostname()
	label := HostnameLabel(hostname)
	if host != s.DeviceID+".zelport.localhost" && (label == "" || host != label+".zelport.localhost") {
		return nil, errors.New("administration alias is not owned by this instance")
	}
	dispatcher, ok := m.Host.(feature.AdministrationHost)
	if !ok {
		return nil, errors.New("administration bridge unavailable")
	}
	contextData := feature.RemoteAdminContext{ConsumerID: peer.Consumer, ProviderID: s.DeviceID, Hostname: hostname, Authority: o.Admin.Authority, ServiceLinks: map[string]string{}}
	for _, l := range o.Admin.ServiceLinks {
		contextData.ServiceLinks[l.ServiceID] = l.URL
	}
	streamCtx, cancel := context.WithCancel(ctx)
	a, b := net.Pipe()
	pair := &adminPipe{Conn: a, other: b, cancel: cancel}
	// Register before accepting HTTP; failed registration closes both ends.
	release, e := m.Host.Track(revision, m.Host.Scope("client"), pair)
	if e != nil {
		return nil, e
	}
	pair.setRelease(release)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 32 << 10, BaseContext: func(net.Listener) context.Context { return streamCtx }, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != contextData.Authority || len(r.Header.Values(protocol.TraceHeader)) > 0 {
			feature.Status(w, r, 403, "Forbidden", "Invalid administration request.")
			return
		}
		if e := dispatcher.DispatchAdministration(w, r, contextData); e != nil {
			feature.Status(w, r, 503, "Administration unavailable", e.Error())
		}
	}), ConnState: func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			pair.Close()
		}
	}}
	server.SetKeepAlivesEnabled(false)
	go func() { server.Serve(&oneListener{conn: b, ctx: streamCtx}); pair.Close() }()
	return pair, nil
}
