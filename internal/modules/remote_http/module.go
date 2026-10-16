package remote_http

import (
	"crypto/tls"
	"embed"
	"errors"
	"fmt"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var assets embed.FS

type Settings struct {
	TargetURL     string `json:"target_url"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	RewriteOrigin bool   `json:"rewrite_origin"`
}
type Module struct{ modulekit.Base }

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
				detail = "Unencrypted upstream connection. Traffic can be read or modified in transit."
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
	if m.Host != nil && modulekit.SelfTarget(u, m.Host.GatewayAuthority("localhost")) {
		return fmt.Errorf("target cannot point to the Zelport Localhost port")
	}
	return nil
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	var s Settings
	feature.Decode(v.Settings, &s)
	u, e := ParseRemoteURL(s.TargetURL)
	if e != nil {
		feature.Status(w, r, 503, "Invalid remote target", e.Error())
		return
	}
	if modulekit.SelfTarget(u, m.Host.GatewayAuthority("localhost")) {
		feature.Status(w, r, 503, "Invalid remote target", "The target cannot point to the Zelport Localhost port.")
		return
	}
	modulekit.Proxy(w, r, u, modulekit.ProxyOptions{GatewayAuthority: m.Host.GatewayAuthority("localhost"), SkipTLSVerify: s.SkipTLSVerify, RewriteOrigin: s.RewriteOrigin, Error: func(w http.ResponseWriter, r *http.Request, e error) {
		title := "Remote service unavailable"
		var cert *tls.CertificateVerificationError
		if errors.As(e, &cert) {
			title = "TLS certificate verification failed"
		}
		feature.Status(w, r, 502, title, e.Error())
	}})
}
