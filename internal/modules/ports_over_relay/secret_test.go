package ports_over_relay

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	"net/http/httptest"
	"strings"
	"testing"
)

func saveSecret(t *testing.T, m *Module, secret string) *httptest.ResponseRecorder {
	t.Helper()
	_, revision := m.store.snapshot()
	body, _ := json.Marshal(map[string]any{"revision": revision, "relay_url": "ws://relay.example.test", "group_token": secret})
	w := httptest.NewRecorder()
	m.Admin().ServeHTTP(w, httptest.NewRequest("PUT", "/settings", strings.NewReader(string(body))))
	return w
}
func TestUserEnteredSecretDerivesStableKeyAndRevealsOriginal(t *testing.T) {
	secret := "synthetic-40-character-secret-for-test-1"
	a, _ := testModule(t)
	b, _ := testModule(t)
	for _, m := range []*Module{a, b} {
		if w := saveSecret(t, m, secret); w.Code != 200 {
			t.Fatalf("40-character secret rejected: %s", w.Body.String())
		}
	}
	sa, rev := a.store.snapshot()
	sb, _ := b.store.snapshot()
	if sa.GroupToken != sb.GroupToken {
		t.Fatal("same secret derived different keys")
	}
	if _, err := protocol.DecodeKey(sa.GroupToken); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(a.State())
	if strings.Contains(string(state), secret) || strings.Contains(string(state), sa.GroupToken) {
		t.Fatal("credentials leaked through state")
	}
	body, _ := json.Marshal(map[string]any{"revision": rev, "action": "reveal"})
	w := httptest.NewRecorder()
	a.Admin().ServeHTTP(w, httptest.NewRequest("POST", "/credentials", strings.NewReader(string(body))))
	var got map[string]string
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["group_token"] != secret {
		t.Fatal("reveal must return original input")
	}
	if w := saveSecret(t, a, "another-40-character-secret-for-test-00001"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	next, _ := a.store.snapshot()
	if next.GroupToken == sa.GroupToken {
		t.Fatal("replacement did not change group key")
	}
}
func TestSecretLengthUnicodeAndStoredPairValidation(t *testing.T) {
	m, _ := testModule(t)
	for _, secret := range []string{strings.Repeat("x", 31), strings.Repeat("😀", 31), strings.Repeat("x", 257)} {
		if w := saveSecret(t, m, secret); w.Code != 400 {
			t.Fatalf("invalid length accepted: %d", len(secret))
		} else if !strings.Contains(w.Body.String(), "characters") {
			t.Fatal("error must describe character requirements", w.Body.String())
		}
	}
	secret := strings.Repeat("ě", 32)
	if w := saveSecret(t, m, secret); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ := m.store.snapshot()
	raw, _ := json.Marshal(s)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	fields["group_secret"] = "different-valid-secret"
	raw, _ = json.Marshal(fields)
	if m.ValidateSettings(raw) == nil {
		t.Fatal("inconsistent stored secret/key pair accepted")
	}
	before, rev := m.store.snapshot()
	if w := saveSecret(t, m, "too short"); w.Code != 400 {
		t.Fatal(w.Body.String())
	}
	after, next := m.store.snapshot()
	if rev != next || before.GroupToken != after.GroupToken {
		t.Fatal("invalid save changed stored key")
	}
}
func TestGeneratedSecretRemainsCompatibleAndClearsOriginal(t *testing.T) {
	m, _ := testModule(t)
	if w := saveSecret(t, m, "a-user-entered-secret-123456789012345"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	token, _ := protocol.Token()
	if w := saveSecret(t, m, token); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ := m.store.snapshot()
	if s.GroupToken != token {
		t.Fatal("generated key changed")
	}
	raw, _ := json.Marshal(s)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["group_secret"] != nil && fields["group_secret"] != "" {
		t.Fatal("old original secret retained")
	}
}

// This vector is part of the persisted group-identity contract.
func TestOrdinarySecretDerivationVersionOneVector(t *testing.T) {
	m := New().(*Module)
	got, err := m.groupToken("synthetic-40-character-secret-for-test-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ovuopfWAaVBmMDx7PRRaINg-AyGkh85FNn2UIeaUNxA" {
		t.Fatal("v1 derivation changed; existing devices would lose their group identity")
	}
}

func TestSecretPreservesAnyCharactersAtMinimum(t *testing.T) {
	for _, secret := range []string{strings.Repeat(" ", 32), " " + strings.Repeat("😀", 30) + " ", strings.Repeat("!", 32), strings.Repeat("x", 32) + "\r\nend\r"} {
		m, _ := testModule(t)
		if w := saveSecret(t, m, secret); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		s, _ := m.store.snapshot()
		if s.revealedSecret() != secret {
			t.Fatal("input changed")
		}
		raw, _ := json.Marshal(s)
		if err := New().(*Module).ValidateSettings(raw); err != nil {
			t.Fatalf("saved secret cannot reload: %v", err)
		}
	}
}
func TestStoredShortSecretSurvivesLoadAndUnrelatedSave(t *testing.T) {
	m, _ := testModule(t)
	s, _ := newIdentity()
	s.GroupSecret = "legacy-secret-16!"
	token, err := m.groupToken(s.GroupSecret)
	if err != nil {
		t.Fatal(err)
	}
	s.GroupToken = token
	raw, _ := json.Marshal(s)
	if err := New().(*Module).ValidateSettings(raw); err != nil {
		t.Fatal(err)
	}
	if err := m.store.update(context.Background(), feature.CommitOptions{}, func(v *Settings) error { *v = s; return nil }); err != nil {
		t.Fatal(err)
	}
	if w := saveSecret(t, m, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	after, _ := m.store.snapshot()
	if after.GroupSecret != s.GroupSecret || after.GroupToken != token {
		t.Fatal("unrelated save changed legacy identity")
	}
	if w := saveSecret(t, m, s.GroupSecret); w.Code != 400 {
		t.Fatal("new entry bypassed minimum")
	}
}
