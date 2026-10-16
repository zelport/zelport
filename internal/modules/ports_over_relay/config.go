package ports_over_relay

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
)

type Watermark struct {
	DigestVersion int    `json:"digest_version,omitempty"`
	Sequence      uint64 `json:"sequence"`
	Digest        string `json:"digest"`
}
type Settings struct {
	AdminServiceID string                  `json:"admin_service_id,omitempty"`
	Peers          map[string]PeerIdentity `json:"peers,omitempty"`
	Enabled        bool                    `json:"enabled"`
	Publish        bool                    `json:"publish"`
	RelayURL       string                  `json:"relay_url,omitempty"`
	AccessToken    string                  `json:"access_token,omitempty"`
	GroupToken     string                  `json:"group_token,omitempty"`
	DeviceID       string                  `json:"device_id,omitempty"`
	PrivateKey     string                  `json:"private_key,omitempty"`
	Sequence       uint64                  `json:"sequence,omitempty"`
	Watermarks     map[string]Watermark    `json:"watermarks,omitempty"`
}
type ImportSettings struct {
	protocol.Import
	RewriteOrigin bool `json:"rewrite_origin"`
}

func newIdentity() (Settings, error) {
	key, e := ecdh.X25519().GenerateKey(rand.Reader)
	if e != nil {
		return Settings{}, e
	}
	token, e := protocol.Token()
	if e != nil {
		return Settings{}, e
	}
	return Settings{AdminServiceID: protocol.ID(), DeviceID: protocol.ID(), PrivateKey: base64.RawURLEncoding.EncodeToString(key.Bytes()), GroupToken: token}, nil
}
func (m *Module) ValidateSettings(b json.RawMessage) error {
	var s Settings
	if e := feature.Decode(b, &s); e != nil {
		return e
	}
	established := s.DeviceID != "" || s.PrivateKey != "" || s.GroupToken != "" || s.Enabled || s.Sequence != 0
	if established {
		if !protocol.ValidID(s.DeviceID) {
			return errors.New("relay identity is missing or invalid; restore the configuration backup")
		}
		if _, e := protocol.DecodeKey(s.PrivateKey); e != nil {
			return errors.New("relay private identity is invalid; restore the configuration backup")
		}
		if _, e := protocol.DecodeKey(s.GroupToken); e != nil {
			return errors.New("E2E secret must contain 32 random bytes")
		}
	}
	if s.AccessToken != "" && s.AccessToken == s.GroupToken {
		return errors.New("relay access token and E2E secret must be different")
	}
	if s.AccessToken != "" {
		if _, e := protocol.DecodeKey(s.AccessToken); e != nil {
			return errors.New("relay access token must contain 32 random bytes")
		}
	}
	if s.RelayURL != "" {
		u, e := url.Parse(s.RelayURL)
		if e != nil || u.Host == "" || (u.Scheme != "ws" && u.Scheme != "wss") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/" && u.Path != "/v3/ws" && u.Path != "/v2/ws") {
			return errors.New("relay URL must be ws://host or wss://host without credentials or query")
		}
	}
	if s.Enabled && s.RelayURL == "" {
		return errors.New("relay URL is required")
	}
	if s.AdminServiceID != "" && (!protocol.ValidID(s.AdminServiceID) || s.AdminServiceID == s.DeviceID) {
		return errors.New("invalid administration service identity")
	}
	if len(s.Peers) > maxPeers {
		return errors.New("administration peer history limit")
	}
	total := 0
	for key, p := range s.Peers {
		parts := strings.Split(key, "/")
		if len(parts) != 2 || len(parts[0]) != 64 || strings.Trim(parts[0], "0123456789abcdef") != "" || parts[1] != p.DeviceID || !protocol.ValidID(p.DeviceID) || !protocol.ValidID(p.AdminServiceID) || len(p.Hostname) > 253 {
			return errors.New("invalid administration peer")
		}
		if _, e := protocol.DecodeKey(p.PublicKey); e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, a := range p.Aliases {
			if a == "" || len(a) > 63 || strings.Contains(a, ".") || !protocol.ValidDomain(a+".localhost") || seen[a] || uuidLabel.MatchString(a) && a != p.DeviceID {
				return errors.New("invalid administration alias")
			}
			seen[a] = true
			total++
		}
		if !seen[p.DeviceID] {
			return errors.New("missing administration UUID alias")
		}
	}
	if total > maxAliases {
		return errors.New("administration alias history limit")
	}
	for k, w := range s.Watermarks {
		if (w.DigestVersion != 0 && w.DigestVersion != 3) || len(k) > 256 || w.Sequence == 0 || len(w.Digest) != 64 {
			return errors.New("invalid catalog watermark")
		}
	}
	return nil
}
func (m *Module) ValidateRoute(r feature.Route) error {
	var s ImportSettings
	if e := feature.Decode(r.Settings, &s); e != nil {
		return e
	}
	if len(s.Group) != 64 || strings.Trim(s.Group, "0123456789abcdef") != "" || !protocol.ValidID(s.DeviceID) || !protocol.ValidID(s.ServiceID) {
		return errors.New("select a valid relay service")
	}
	if _, e := protocol.DecodeKey(s.ProviderKey); e != nil {
		return errors.New("invalid provider key")
	}
	if m.Host != nil {
		own, _ := m.store.snapshot()
		if s.DeviceID == own.DeviceID {
			return errors.New("cannot import this device's own services")
		}
		for _, old := range m.Host.Snapshot().Routes {
			if old.ID == r.ID {
				var prev ImportSettings
				feature.Decode(old.Settings, &prev)
				if prev.Import != s.Import {
					return errors.New("an existing import cannot change its provider binding")
				}
				return nil
			}
		}
		return m.validateOffer(s)
	}
	return nil
}
