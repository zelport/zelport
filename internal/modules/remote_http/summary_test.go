package remote_http

import (
	"encoding/json"
	"github.com/zelport/zelport/internal/feature"
	"testing"
)

func TestUpstreamSecuritySummary(t *testing.T) {
	for _, tc := range []struct {
		url  string
		skip bool
		want string
	}{
		{"http://example.test/service", false, "Unencrypted upstream connection. Traffic can be read or modified in transit."},
		{"https://example.test/service", false, ""},
		{"https://example.test/service", true, "TLS certificate verification is disabled. The remote server's identity is not verified."},
	} {
		raw, _ := json.Marshal(Settings{TargetURL: tc.url, SkipTLSVerify: tc.skip})
		got := New().(*Module).Summary(feature.Route{Settings: raw})
		if got.Detail != tc.want || got.Target != tc.url || got.State != "configured" {
			t.Fatalf("%s skip=%v: %+v", tc.url, tc.skip, got)
		}
	}
}
