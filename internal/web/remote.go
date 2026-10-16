package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/zelport/zelport/internal/feature"
	"net/http"
)

// ServeRemoteAdministration is an internal entry point; ServeHTTP never infers this context.
func (s *Server) ServeRemoteAdministration(w http.ResponseWriter, r *http.Request, ctx feature.RemoteAdminContext) {
	if r.Host != ctx.Authority || ctx.ConsumerID == "" || ctx.ProviderID == "" {
		reply(w, 403, map[string]string{"error": "invalid remote administration context"})
		return
	}
	origin := "http://" + ctx.Authority
	raw, _ := json.Marshal([]string{"remote-admin/v1", ctx.ConsumerID, ctx.ProviderID, origin})
	mac := hmac.New(sha256.New, []byte(s.token))
	mac.Write(raw)
	s.adminWithContext(w, r, origin, hex.EncodeToString(mac.Sum(nil)), &ctx)
}
