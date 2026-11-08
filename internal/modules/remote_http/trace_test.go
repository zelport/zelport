package remote_http

import (
	"net/url"
	"testing"
)

func TestLoopbackTargetKeepsRelayTraceOnlyOnThisComputer(t *testing.T) {
	for raw, want := range map[string]bool{
		"http://localhost:3000":         true,
		"http://app.localhost:3000":     true,
		"http://127.0.0.1:9911":         true,
		"http://[::1]:8080":             true,
		"https://telekomunikace.cz":     false,
		"http://dock01.slovan.lan:8080": false,
		"http://192.168.1.20:8080":      false,
	} {
		u, _ := url.Parse(raw)
		if got := loopbackTarget(u); got != want {
			t.Fatalf("loopbackTarget(%s) = %v, want %v", raw, got, want)
		}
	}
}
