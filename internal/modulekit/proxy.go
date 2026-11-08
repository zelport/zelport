package modulekit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/zelport/zelport/internal/feature"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ResponseHeaderTimeout bounds how long a service may take to start its answer.
const ResponseHeaderTimeout = 5 * time.Minute

type ProxyOptions struct {
	PreserveHost, RewriteOrigin, SkipTLSVerify bool
	// KeepRelayTrace forwards the signed relay trace, which names the device and group. Only the
	// relay leg needs it, so every other service never receives it.
	KeepRelayTrace bool
	// External marks a target that cannot lead back to a Zelport (a remote host beyond loopback).
	// It receives no Zelport loop metadata at all.
	External          bool
	ListenerAuthority string
	RootCAs           *x509.CertPool
	ProxyURL          *url.URL
	Dial              func(context.Context, string, string) (net.Conn, error)
	Error             func(http.ResponseWriter, *http.Request, error)
}

func Proxy(w http.ResponseWriter, r *http.Request, target *url.URL, o ProxyOptions) {
	l := feature.RequestLease(r)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if o.ProxyURL != nil {
		transport.Proxy = http.ProxyURL(o.ProxyURL)
	}
	transport.DisableKeepAlives = true
	// Long polling, slow exports and builds may answer late. The browser still decides when to give up.
	transport.ResponseHeaderTimeout = ResponseHeaderTimeout
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: o.SkipTLSVerify, RootCAs: o.RootCAs}
	dial := o.Dial
	if dial == nil {
		_, listenerPort, _ := net.SplitHostPort(o.ListenerAuthority)
		if listenerPort == "" {
			listenerPort = "80"
		}
		dialer := net.Dialer{Timeout: 5 * time.Second, Control: func(network, address string, _ syscall.RawConn) error {
			h, p, e := net.SplitHostPort(address)
			if e == nil && p == listenerPort && net.ParseIP(h).IsLoopback() {
				return errors.New("target resolves to Zelport's own Localhost port")
			}
			return nil
		}}
		dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			h, p, e := net.SplitHostPort(address)
			if e == nil && h == "localhost" {
				c, e := dialer.DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", p))
				if e == nil {
					return c, nil
				}
				return dialer.DialContext(ctx, "tcp6", net.JoinHostPort("::1", p))
			}
			return dialer.DialContext(ctx, network, address)
		}
	}
	trackedDial := func(ctx context.Context, n, a string) (net.Conn, error) {
		c, e := dial(ctx, n, a)
		if e != nil {
			return nil, e
		}
		if l != nil {
			if _, e = l.Track(c); e != nil {
				return nil, e
			}
		}
		return c, nil
	}
	transport.DialContext = trackedDial
	// Go otherwise reuses TLSClientConfig for the HTTPS proxy and the origin.
	// Origin's opt-out must never weaken TLS verification of the proxy itself.
	if o.ProxyURL != nil && o.ProxyURL.Scheme == "https" {
		transport.DialTLSContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := trackedDial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			server, _, err := net.SplitHostPort(address)
			if err != nil {
				conn.Close()
				return nil, err
			}
			secure := tls.Client(conn, &tls.Config{RootCAs: o.RootCAs, ServerName: server})
			handshakeCtx, cancel := context.WithTimeout(ctx, transport.TLSHandshakeTimeout)
			defer cancel()
			if err = secure.HandshakeContext(handshakeCtx); err != nil {
				conn.Close()
				return nil, err
			}
			return secure, nil
		}
	}
	defer transport.CloseIdleConnections()
	proxy := httputil.ReverseProxy{Transport: transport, FlushInterval: -1, Rewrite: func(p *httputil.ProxyRequest) {
		p.SetURL(target)
		p.Out.Header.Del("Proxy-Authorization")
		if !o.KeepRelayTrace || o.External {
			p.Out.Header.Del(feature.RelayTraceHeader)
		}
		if o.External {
			p.Out.Header.Del(feature.LoopHeader)
		}
		if o.PreserveHost {
			p.Out.Host = p.In.Host
		}
		if o.RewriteOrigin {
			origins := p.In.Header.Values("Origin")
			if len(origins) == 1 && strings.EqualFold(origins[0], "http://"+p.In.Host) {
				p.Out.Header.Set("Origin", target.Scheme+"://"+target.Host)
			}
		}
		p.Out.URL.RawQuery = p.In.URL.RawQuery
		p.SetXForwarded()
	}, ErrorHandler: o.Error}
	if proxy.ErrorHandler == nil {
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			feature.StatusDiagnostic(w, r, 502, "Service unavailable", "The service could not be reached.", e.Error())
		}
	}
	proxy.ServeHTTP(w, r)
}

// SelfTarget catches literal listener addresses; DialContext also checks resolved IPs.
func SelfTarget(target *url.URL, listener string) bool {
	_, port, err := net.SplitHostPort(listener)
	if err != nil {
		port = "80"
	}
	targetPort := target.Port()
	if targetPort == "" {
		targetPort = "80"
		if target.Scheme == "https" {
			targetPort = "443"
		}
	}
	host := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	ip := net.ParseIP(host)
	return port == targetPort && (host == "localhost" || strings.HasSuffix(host, ".localhost") || ip != nil && ip.IsLoopback())
}
