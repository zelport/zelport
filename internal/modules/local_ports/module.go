package local_ports

import (
	"embed"
	"fmt"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
)

//go:embed assets/*
var assets embed.FS

type Settings struct {
	AuxPort int `json:"aux_port"`
}
type Module struct{ modulekit.Base }

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	m := &Module{}
	m.Desc = feature.Descriptor{Maturity: feature.Stable, ID: "local_ports", Name: "Local Ports", TabID: "local", Order: 30, Exportable: true, Assets: files}
	m.Summary = func(r feature.Route) feature.Summary {
		var s Settings
		feature.Decode(r.Settings, &s)
		return feature.Summary{State: "configured", Target: fmt.Sprintf("Aux port %d", s.AuxPort)}
	}
	return m
}
func (m *Module) ValidateRoute(r feature.Route) error {
	var s Settings
	if e := feature.Decode(r.Settings, &s); e != nil {
		return e
	}
	if s.AuxPort < 1 || s.AuxPort > 65535 {
		return fmt.Errorf("Aux port must be between 1 and 65535")
	}
	if m.Host != nil && m.Host.ListenerAuthority("localhost") == "localhost:"+strconv.Itoa(s.AuxPort) || m.Host != nil && s.AuxPort == 80 && m.Host.ListenerAuthority("localhost") == "localhost" {
		return fmt.Errorf("Aux port cannot be the Zelport Localhost port")
	}
	return nil
}
func (m *Module) ServeRoute(w http.ResponseWriter, r *http.Request, v feature.Route) {
	var s Settings
	feature.Decode(v.Settings, &s)
	target := &url.URL{Scheme: "http", Host: "localhost:" + strconv.Itoa(s.AuxPort)}
	if modulekit.SelfTarget(target, m.Host.ListenerAuthority("localhost")) {
		feature.Status(w, r, 503, "Invalid Aux port", "The Aux port cannot be the Zelport Localhost port.")
		return
	}
	modulekit.Proxy(w, r, &url.URL{Scheme: "http", Host: "localhost:" + strconv.Itoa(s.AuxPort)}, modulekit.ProxyOptions{PreserveHost: true, ListenerAuthority: m.Host.ListenerAuthority("localhost")})
}
