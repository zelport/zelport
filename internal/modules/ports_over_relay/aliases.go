package ports_over_relay

import (
	"encoding/hex"
	"errors"
	"github.com/zelport/zelport-sdk-go/client"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport-sdk-go/secure"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxPeers = 1024
const maxAliases = 4096

var uuidLabel = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type PeerIdentity struct {
	DeviceID       string   `json:"device_id"`
	PublicKey      string   `json:"public_key"`
	AdminServiceID string   `json:"admin_service_id"`
	Hostname       string   `json:"hostname"`
	Aliases        []string `json:"aliases"`
}
type AdminTarget struct {
	protocol.Import
	Hostname  string
	Authority string
	Available bool
}
type AdminPeerView struct {
	DeviceID    string `json:"device_id"`
	Hostname    string `json:"hostname"`
	UUIDURL     string `json:"uuid_url"`
	HostnameURL string `json:"hostname_url,omitempty"`
	Available   bool   `json:"available"`
	Conflict    bool   `json:"conflict"`
}

func HostnameLabel(host string) string {
	label := strings.Trim(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, strings.ToLower(host)), "-")
	if len(label) > 63 || uuidLabel.MatchString(label) {
		return ""
	}
	return label
}
func groupID(s Settings) string {
	secret, e := protocol.DecodeKey(s.GroupToken)
	if e != nil {
		return ""
	}
	key, _ := secure.Derive(secret, "group-locator")
	return hex.EncodeToString(key[:])
}
func rememberPeer(s *Settings, r protocol.Record) error {
	if r.DeviceID == s.DeviceID {
		return nil
	}
	admin := ""
	for _, v := range r.Services {
		if v.Kind == protocol.Administration {
			admin = v.ID
		}
	}
	if admin == "" {
		return nil
	}
	key := r.Group + "/" + r.DeviceID
	if s.Peers == nil {
		s.Peers = map[string]PeerIdentity{}
	}
	p, exists := s.Peers[key]
	if exists && (p.PublicKey != r.PublicKey || p.AdminServiceID != admin) {
		return errors.New("administration provider identity changed")
	}
	if !exists && len(s.Peers) >= maxPeers {
		return errors.New("administration peer history limit")
	}
	p.DeviceID = r.DeviceID
	p.PublicKey = r.PublicKey
	p.AdminServiceID = admin
	p.Hostname = r.Hostname
	for _, label := range []string{r.DeviceID, HostnameLabel(r.Hostname)} {
		if label == "" {
			continue
		}
		found := false
		for _, v := range p.Aliases {
			if v == label {
				found = true
			}
		}
		if !found {
			p.Aliases = append(p.Aliases, label)
		}
	}
	total := len(p.Aliases)
	for k, v := range s.Peers {
		if k != key {
			total += len(v.Aliases)
		}
	}
	if total > maxAliases {
		return errors.New("administration alias history limit")
	}
	s.Peers[key] = p
	return nil
}
func aliasOwners(s Settings, group, label string) []PeerIdentity {
	result := []PeerIdentity{}
	for key, p := range s.Peers {
		if !strings.HasPrefix(key, group+"/") {
			continue
		}
		for _, a := range p.Aliases {
			if a == label {
				result = append(result, p)
				break
			}
		}
	}
	return result
}
func (m *Module) cloudStatus() client.Status {
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()
	if c == nil {
		return client.Status{}
	}
	return c.Snapshot()
}
func resolveAdmin(s Settings, status client.Status, label string) (AdminTarget, error) {
	group := groupID(s)
	owners := aliasOwners(s, group, label)
	if len(owners) == 0 {
		return AdminTarget{}, errors.New("unknown administration alias")
	}
	if len(owners) > 1 {
		return AdminTarget{Hostname: label}, errors.New("hostname alias conflicts; use a UUID address")
	}
	p := owners[0]
	t := AdminTarget{Import: protocol.Import{Group: group, DeviceID: p.DeviceID, ProviderKey: p.PublicKey, ServiceID: p.AdminServiceID}, Hostname: p.Hostname}
	if p.DeviceID == s.DeviceID {
		return AdminTarget{}, errors.New("self administration alias is unavailable")
	}
	if !s.Enabled || !status.Connected {
		return t, nil
	}
	for _, r := range status.Records {
		if r.DeviceID != p.DeviceID || r.Group != group || r.PublicKey != p.PublicKey || r.ExpiresAt <= time.Now().Unix() {
			continue
		}
		if label != p.DeviceID && label != HostnameLabel(r.Hostname) {
			return t, nil
		}
		for _, service := range r.Services {
			if service.ID == p.AdminServiceID && service.Kind == protocol.Administration {
				t.Authority, _ = r.Authority(service.ID)
				t.Available = t.Authority != ""
			}
		}
	}
	return t, nil
}
func (m *Module) resolveAdministration(host string) (AdminTarget, error) {
	s, _ := m.store.snapshot()
	return resolveAdmin(s, m.cloudStatus(), strings.TrimSuffix(host, ".zelport.localhost"))
}
func (m *Module) adminPeers(s Settings, status client.Status) []AdminPeerView {
	out := []AdminPeerView{}
	group := groupID(s)
	for key, p := range s.Peers {
		if !strings.HasPrefix(key, group+"/") || p.DeviceID == s.DeviceID {
			continue
		}
		t, _ := resolveAdmin(s, status, p.DeviceID)
		v := AdminPeerView{DeviceID: p.DeviceID, Hostname: p.Hostname, UUIDURL: "http://" + m.Host.GatewayAuthority(p.DeviceID+".zelport.localhost"), Available: t.Available}
		if label := HostnameLabel(p.Hostname); label != "" {
			if len(aliasOwners(s, group, label)) == 1 {
				v.HostnameURL = "http://" + m.Host.GatewayAuthority(label+".zelport.localhost")
			} else {
				v.Conflict = true
			}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}
