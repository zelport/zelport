package ports_over_relay

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"unicode/utf8"

	"github.com/zelport/zelport-sdk-go/protocol"
	"golang.org/x/crypto/argon2"
)

func validateSecret(secret string) error {
	if !utf8.ValidString(secret) || utf8.RuneCountInString(secret) < 32 {
		return errors.New("Use at least 32 characters for the E2E secret.")
	}
	if utf8.RuneCountInString(secret) > 256 {
		return errors.New("Use no more than 256 characters for the E2E secret.")
	}
	return nil
}

// groupToken keeps generated keys compatible and derives ordinary user input.
// Parameters and domain salt are fixed: the same input must pair across devices.
// A one-entry cache avoids repeating the KDF during catalog metadata commits;
// the lock also bounds concurrent KDF memory use to one operation per module.
func (m *Module) groupToken(secret string) (string, error) {
	// Persisted secrets predate the current minimum. Derivation must not apply
	// new-entry policy or alter their bytes and group identity.
	if !utf8.ValidString(secret) || secret == "" || utf8.RuneCountInString(secret) > 256 {
		return "", errors.New("The stored E2E secret is invalid.")
	}
	if _, err := protocol.DecodeKey(secret); err == nil {
		return secret, nil
	}
	digest := sha256.Sum256([]byte(secret))
	m.secretMu.Lock()
	defer m.secretMu.Unlock()
	if m.secretToken != "" && m.secretDigest == digest {
		return m.secretToken, nil
	}
	key := argon2.IDKey([]byte(secret), []byte("zelport/e2e-secret/argon2id/v1"), 3, 64*1024, 4, 32)
	token := base64.RawURLEncoding.EncodeToString(key)
	m.secretDigest = digest
	m.secretToken = token
	return token, nil
}

func (s Settings) revealedSecret() string {
	if s.GroupSecret != "" {
		return s.GroupSecret
	}
	return s.GroupToken
}

// Rejection sampling avoids bias from mapping 256 byte values onto 62 symbols.
func generateSecret() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result [40]byte
	for n := 0; n < len(result); {
		var random [64]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", err
		}
		for _, b := range random {
			if b >= 248 {
				continue
			}
			result[n] = alphabet[int(b)%len(alphabet)]
			n++
			if n == len(result) {
				break
			}
		}
	}
	return string(result[:]), nil
}
