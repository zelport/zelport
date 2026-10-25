package remote_http

import (
	"crypto/tls"
	"crypto/x509"
	"embed"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/configfile"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"sync"
)

//go:embed assets/*
var assets embed.FS

type Settings struct {
	TargetURL     string `json:"target_url"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	RewriteOrigin bool   `json:"rewrite_origin"`
}
type Module struct {
	modulekit.Base
	fileEditor configfile.Editor
	configMu   sync.RWMutex
	caMu       sync.Mutex
	caHash     [32]byte
	caRoots    *x509.CertPool
	closed     bool
}

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	m := &Module{}
	m.Desc = feature.Descriptor{Maturity: feature.Stable, ID: "remote_http", Name: "Remote HTTP", TabID: "remote", Order: 40, Exportable: true, Assets: files}
	m.Summary = func(r feature.Route) feature.Summary {
		var s Settings
		feature.Decode(r.Settings, &s)
		detail := ""
		if u, err := ParseRemoteURL(s.TargetURL); err == nil {
			if u.Scheme == "http" {
				detail = "Traffic to this target is unencrypted."
			} else if s.SkipTLSVerify {
				detail = "TLS certificate verification is disabled. The remote server's identity is not verified."
			}
		}
		return feature.Summary{State: "configured", Target: s.TargetURL, Detail: detail}
	}
	return m
}
func (m *Module) ValidateRoute(r feature.Route) error {
	var s Settings
	if e := feature.Decode(r.Settings, &s); e != nil {
		return e
	}
	u, e := ParseRemoteURL(s.TargetURL)
	if e != nil {
		return e
	}
	if s.SkipTLSVerify && u.Scheme != "https" {
		return fmt.Errorf("TLS settings require an HTTPS target")
	}
	if m.Host != nil && modulekit.SelfTarget(u, m.Host.ListenerAuthority("localhost")) {
		return fmt.Errorf("target cannot point to the Zelport Localhost port")
	}
	return nil
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	// Read the settings, scope generation and revision, then acquire a lease as
	// one operation relative to settings saves. Host's CAS also rejects unrelated
	// concurrent commits, and the parent's context preserves route revocation.
	m.configMu.RLock()
	scope := m.Host.Scope("outbound")
	snapshot := m.Host.Snapshot()
	config, err := configuration(snapshot.Settings)
	lease, leaseErr := m.Host.Acquire(r.Context(), v.ID, snapshot.Revision, scope)
	m.configMu.RUnlock()
	if leaseErr != nil {
		feature.Status(w, r, 503, "Service unavailable", "The route configuration changed. Try again.")
		return
	}
	rw := &feature.Writer{ResponseWriter: w, Lease: lease}
	defer func() {
		if !rw.Hijacked {
			lease.Close()
		}
	}()
	w = rw
	r = r.WithContext(feature.WithLease(lease.Context(), lease))
	// Use the freshly acquired route rather than an older dispatcher snapshot.
	v = lease.Route()
	if err != nil {
		feature.Status(w, r, 503, "Remote HTTP configuration unavailable", err.Error())
		return
	}
	path, err := resolveCAPath(config.CACertPath)
	var roots *x509.CertPool
	if err == nil {
		roots, err = m.loadRoots(path, config.CACertPath == "")
	}
	if err != nil {
		feature.Status(w, r, 503, "Remote HTTP configuration unavailable", err.Error())
		return
	}
	proxyURL, _ := parseProxyURL(config.ProxyURL)
	if proxyURL != nil && (config.ProxyUsername != "" || config.ProxyPassword != "") {
		proxyURL.User = url.UserPassword(config.ProxyUsername, config.ProxyPassword)
	}

	var s Settings
	feature.Decode(v.Settings, &s)
	u, e := ParseRemoteURL(s.TargetURL)
	if e != nil {
		feature.StatusDiagnostic(w, r, 503, "Invalid remote target", "The remote service URL is invalid.", e.Error())
		return
	}
	if modulekit.SelfTarget(u, m.Host.ListenerAuthority("localhost")) {
		feature.Status(w, r, 503, "Invalid remote target", "The target cannot point to the Zelport Localhost port.")
		return
	}
	// A proxy resolves target names itself, bypassing the direct dial guard.
	// Check local resolution too when the target uses our listener's port.
	if proxyURL != nil && resolvedSelfTarget(r, u, m.Host.ListenerAuthority("localhost")) {
		feature.Status(w, r, 503, "Invalid remote target", "The target cannot point to the Zelport Localhost port.")
		return
	}
	modulekit.Proxy(w, r, u, modulekit.ProxyOptions{RootCAs: roots, ProxyURL: proxyURL, ListenerAuthority: m.Host.ListenerAuthority("localhost"), SkipTLSVerify: s.SkipTLSVerify, RewriteOrigin: s.RewriteOrigin, Error: func(w http.ResponseWriter, r *http.Request, e error) {
		title := "Remote service unavailable"
		message := "The remote service could not be reached."
		var cert *tls.CertificateVerificationError
		if errors.As(e, &cert) {
			title = "TLS certificate verification failed"
			message = "The remote server's TLS certificate could not be verified."
		}
		diagnostic := e.Error()
		if proxyURL != nil {
			diagnostic = "The outbound proxy connection or upstream request failed."
		}
		feature.StatusDiagnostic(w, r, 502, title, message, diagnostic)
	}})
}

// Proxy-side DNS cannot be controlled locally, but names resolving to this
// listener must receive the same protection as direct connections.
func resolvedSelfTarget(r *http.Request, target *url.URL, listener string) bool {
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
	if targetPort != port {
		return false
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(r.Context(), target.Hostname())
	if err != nil {
		return false
	}
	for _, address := range addresses {
		if address.IP.IsLoopback() {
			return true
		}
	}
	return false
}

func (m *Module) Close() error {
	m.configMu.Lock()
	m.closed = true
	m.configMu.Unlock()
	return m.Base.Close()
}
