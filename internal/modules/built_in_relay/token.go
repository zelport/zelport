package relay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	server "github.com/zelport/zelport-relay/relay"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
)

// rotateToken runs under m.mu. Persistence must succeed before revoking any connection.
func (m *Module) rotateToken(ctx context.Context, revision uint64, next Settings, action, token string) error {
	if action == "generate" {
		if token != "" {
			return errors.New("generate does not accept an access token")
		}
		var err error
		token, err = protocol.Token()
		if err != nil {
			return err
		}
	}
	key, err := protocol.DecodeKey(token)
	if err != nil {
		return errors.New("access token must be a base64url-encoded 32-byte key")
	}
	oldKey, _ := protocol.DecodeKey(next.AccessToken)
	if bytes.Equal(key, oldKey) {
		return errors.New("choose a different access token")
	}
	next.AccessToken = base64.RawURLEncoding.EncodeToString(key)
	options, err := serverOptions(next)
	if err != nil {
		return err
	}
	candidate, err := server.New(options)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(next)
	if err != nil {
		candidate.Close()
		return err
	}
	_, err = m.Host.Commit(ctx, revision, feature.CommitOptions{}, func(d *feature.Draft) error { d.Settings = raw; return nil })
	if err != nil {
		candidate.Close()
		return err
	}
	if m.runtime != nil {
		m.runtime.ReplaceServer(candidate)
	} else {
		candidate.Close()
	}
	return nil
}
