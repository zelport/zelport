package relay

import (
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strconv"

	server "github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
)

type Settings struct {
	Enabled            bool   `json:"enabled"`
	RequireAccessToken bool   `json:"require_access_token"`
	Port               int    `json:"port"`
	BindAddress        string `json:"bind_address"`
	AccessToken        string `json:"access_token,omitempty"`
}

func settings(raw json.RawMessage) (Settings, error) {
	s := Settings{Port: 8090}
	if err := feature.Decode(raw, &s); err != nil {
		return s, err
	}
	if s.Port < 1 || s.Port > 65535 {
		return s, errors.New("relay port must be between 1 and 65535")
	}
	if s.BindAddress != "" {
		addr, err := netip.ParseAddr(s.BindAddress)
		if err != nil || addr.IsUnspecified() || addr.IsMulticast() {
			return s, errors.New("The bind address must be a local IP address. Leave it blank to listen on all interfaces.")
		}
	}
	if s.AccessToken != "" {
		if _, err := protocol.DecodeKey(s.AccessToken); err != nil {
			return s, errors.New("invalid relay access token")
		}
	}
	if s.Enabled && s.RequireAccessToken && s.AccessToken == "" {
		return s, errors.New("enabled relay requires an access token")
	}
	return s, nil
}

func serverOptions(s Settings) (server.Options, error) {
	if !s.RequireAccessToken {
		return server.Options{AllowUnauthenticated: true}, nil
	}
	token, err := protocol.DecodeKey(s.AccessToken)
	return server.Options{AccessToken: token}, err
}

func prepare(s Settings) (*server.Runtime, error) {
	options, err := serverOptions(s)
	if err != nil {
		return nil, err
	}
	return server.Prepare(net.JoinHostPort(s.BindAddress, strconv.Itoa(s.Port)), options)
}
