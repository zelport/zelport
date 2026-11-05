package ports_over_relay

import "testing"

func TestUnreachableNamesTheLinkedZelport(t *testing.T) {
	for _, c := range []struct{ hostname, host, want string }{
		{"MacBook-Pro-M4.local", "macbook-pro-m4.zelport.localhost", "MacBook-Pro-M4.local is unreachable"},
		{"", "build-server.zelport.localhost:7070", "build-server is unreachable"},
		{"", "", "Linked Zelport is unreachable"},
	} {
		if got := unreachable(c.hostname, c.host); got != c.want {
			t.Fatalf("unreachable(%q, %q) = %q, want %q", c.hostname, c.host, got, c.want)
		}
	}
}
