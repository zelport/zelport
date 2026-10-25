package remote_http

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/userpaths"
)

type Configuration struct {
	CACertPath    string `json:"ca_cert_path"`
	ProxyURL      string `json:"proxy_url"`
	ProxyUsername string `json:"proxy_username"`
	ProxyPassword string `json:"proxy_password"`
}

const maxCABundleSize = 1 << 20

func resolveCAPath(path string) (string, error) {
	if path == "" {
		config, err := userpaths.Config()
		if err != nil {
			return "", errors.New("Default CA certificate path could not be resolved")
		}
		return filepath.Join(filepath.Dir(config), "additional-ca.pem"), nil
	}
	if strings.ContainsFunc(path, unicode.IsControl) {
		return "", errors.New("CA certificate path must not contain control characters")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("Home directory could not be resolved")
		}
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("CA certificate path must be absolute or start with ~/")
	}
	return filepath.Clean(path), nil
}

func parseProxyURL(value string) (*url.URL, error) {
	if value == "" {
		return nil, nil
	}
	invalid := errors.New("Proxy URL must be an HTTP, HTTPS, SOCKS5 or SOCKS5H endpoint without credentials, path, query or fragment")
	if strings.ContainsAny(value, "?#") || strings.ContainsFunc(value, unicode.IsSpace) || strings.ContainsFunc(value, unicode.IsControl) {
		return nil, invalid
	}
	u, err := url.Parse(value)
	if err != nil || u.Opaque != "" || u.User != nil || u.Hostname() == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, invalid
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, invalid
	}
	if strings.Contains(u.Host, "@") || strings.HasSuffix(u.Host, ":") {
		return nil, invalid
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return nil, invalid
		}
	}
	if strings.Contains(u.Hostname(), ":") && net.ParseIP(u.Hostname()) == nil {
		return nil, invalid
	}
	if _, err := ParseRemoteURL("http://" + u.Host); err != nil {
		return nil, invalid
	}
	return u, nil
}

func configuration(raw json.RawMessage) (Configuration, error) {
	var c Configuration
	if err := feature.Decode(raw, &c); err != nil {
		return c, errors.New("Invalid Remote HTTP configuration")
	}
	if c.CACertPath != "" {
		if _, err := resolveCAPath(c.CACertPath); err != nil {
			return c, err
		}
	}
	if _, err := parseProxyURL(c.ProxyURL); err != nil {
		return c, err
	}
	// Transport constructs Basic headers and SOCKS5 messages from these fields.
	if strings.Contains(c.ProxyUsername, ":") || strings.ContainsFunc(c.ProxyUsername, unicode.IsControl) || strings.ContainsFunc(c.ProxyPassword, unicode.IsControl) {
		return c, errors.New("Proxy credentials contain unsupported characters")
	}
	if c.ProxyURL == "" && (c.ProxyUsername != "" || c.ProxyPassword != "") {
		return c, errors.New("Proxy credentials require a proxy URL")
	}
	if strings.HasPrefix(c.ProxyURL, "socks5") && (len(c.ProxyUsername) > 255 || len(c.ProxyPassword) > 255) {
		return c, errors.New("SOCKS5 credentials must be at most 255 bytes each")
	}
	return c, nil
}

func (m *Module) ValidateSettings(raw json.RawMessage) error {
	_, err := configuration(raw)
	return err
}

// Every read is bounded and hashed, so a changed or unreadable file cannot reuse
// stale trust. System roots are loaded only when a valid bundle actually changes.
func (m *Module) loadRoots(path string, optionalMissing bool) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	invalid := errors.New("Additional CA certificate file could not be read as a regular PEM certificate bundle")
	info, err := os.Stat(path)
	if optionalMissing && missingCAPath(path, err) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxCABundleSize {
		return nil, invalid
	}
	file, err := os.Open(path)
	if optionalMissing && missingCAPath(path, err) {
		return nil, nil
	}
	if err != nil {
		return nil, invalid
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxCABundleSize {
		return nil, invalid
	}
	data, err := io.ReadAll(io.LimitReader(file, maxCABundleSize+1))
	if err != nil || len(data) > maxCABundleSize {
		return nil, invalid
	}
	hash := sha256.Sum256(data)
	m.caMu.Lock()
	defer m.caMu.Unlock()
	if m.caRoots != nil && m.caHash == hash {
		return m.caRoots, nil
	}
	certs, err := parseCABundle(data)
	if err != nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.New("System certificate trust could not be loaded")
	}
	for _, cert := range certs {
		roots.AddCert(cert)
	}
	m.caHash = hash
	m.caRoots = roots
	return roots, nil
}

func parseCABundle(data []byte) ([]*x509.Certificate, error) {
	invalid := errors.New("Use a PEM file containing certificates only, at most 1 MiB.")
	if len(data) > maxCABundleSize {
		return nil, invalid
	}
	rest := data
	var certs []*x509.Certificate
	for len(bytes.TrimSpace(rest)) > 0 {
		rest = bytes.TrimSpace(rest)
		if !bytes.HasPrefix(rest, []byte("-----BEGIN CERTIFICATE-----")) {
			return nil, invalid
		}
		endMarker := []byte("-----END CERTIFICATE-----")
		end := bytes.Index(rest, endMarker)
		if end < 0 {
			return nil, invalid
		}
		end += len(endMarker)
		if bytes.Count(rest[:end], []byte("-----BEGIN ")) != 1 {
			return nil, invalid
		}
		// Decode only the first block: pem.Decode otherwise skips malformed blocks
		// and could silently accept a later valid certificate.
		block, trailing := pem.Decode(rest[:end])
		if block == nil || len(bytes.TrimSpace(trailing)) != 0 || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, invalid
		}
		parsed, err := x509.ParseCertificates(block.Bytes)
		if err != nil {
			return nil, invalid
		}
		certs = append(certs, parsed...)
		rest = rest[end:]
	}
	if len(certs) == 0 {
		return nil, invalid
	}
	return certs, nil
}

// A dangling symlink is an invalid existing configuration, not an absent default.
func missingCAPath(path string, err error) bool {
	if !os.IsNotExist(err) {
		return false
	}
	_, linkErr := os.Lstat(path)
	return os.IsNotExist(linkErr)
}

func displayCAPath(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
		return "~/" + filepath.ToSlash(rel)
	}
	return path
}

func (m *Module) State() feature.State {
	m.configMu.RLock()
	defer m.configMu.RUnlock()
	s := m.Host.Snapshot()
	c, err := configuration(s.Settings)
	path, pathErr := resolveCAPath(c.CACertPath)
	if err == nil {
		err = pathErr
	}
	if err == nil {
		_, err = m.loadRoots(path, c.CACertPath == "")
	}
	state := feature.State{Revision: s.Revision, Routes: []feature.RouteView{}}
	for _, route := range s.Routes {
		state.Routes = append(state.Routes, feature.View(route, m.Summary(route)))
	}
	problem := ""
	if err != nil {
		problem = err.Error()
	}
	defaultPath, _ := resolveCAPath("")
	state.Data, _ = json.Marshal(struct {
		CACertPath               string `json:"ca_cert_path"`
		EffectiveCACertPath      string `json:"effective_ca_cert_path"`
		ProxyURL                 string `json:"proxy_url"`
		ProxyUsername            string `json:"proxy_username"`
		HasProxyPassword         bool   `json:"has_proxy_password"`
		ConfigurationError       string `json:"configuration_error"`
		DefaultCACertPath        string `json:"default_ca_cert_path"`
		DefaultCACertDisplayPath string `json:"default_ca_cert_display_path"`
	}{c.CACertPath, path, c.ProxyURL, c.ProxyUsername, c.ProxyPassword != "", problem, defaultPath, displayCAPath(defaultPath)})
	return state
}
