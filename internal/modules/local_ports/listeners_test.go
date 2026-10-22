package local_ports

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type staticListenerScanner struct {
	listeners []rawListener
	err       error
}

func (s staticListenerScanner) Scan(context.Context) ([]rawListener, error) {
	return s.listeners, s.err
}

func TestDiscoverListenersNormalizesScopeProcessesAndOrder(t *testing.T) {
	scanner := staticListenerScanner{listeners: []rawListener{
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 8080, PID: 20, Process: "zeta"},
		{Address: netip.MustParseAddr("::1"), Port: 8080, PID: 10, Process: "alpha"},
		{Address: netip.IPv4Unspecified(), Port: 3000, PID: 30, Process: "server"},
		{Address: netip.IPv6Unspecified(), Port: 3000, PID: 30, Process: "server"},
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 4000},
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 5000, PID: 50, Process: "mixed"},
		{Address: netip.MustParseAddr("192.168.1.20"), Port: 5000, PID: 50, Process: "mixed"},
		{Address: netip.MustParseAddr("192.168.1.20"), Port: 6000, PID: 60, Process: "lan-only"},
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 9090, PID: 90, Process: "zelport"},
	}}

	got, err := discoverListeners(context.Background(), scanner, 9090)
	if err != nil {
		t.Fatal(err)
	}
	want := []Listener{
		{Port: 3000, Processes: []ProcessIdentity{{PID: 30, Name: "server"}}, ListensBeyondLoopback: true},
		{Port: 4000, Processes: []ProcessIdentity{}},
		{Port: 5000, Processes: []ProcessIdentity{{PID: 50, Name: "mixed"}}, ListensBeyondLoopback: true},
		{Port: 8080, Processes: []ProcessIdentity{{PID: 10, Name: "alpha"}, {PID: 20, Name: "zeta"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeners:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDiscoverListenersReturnsScannerFailure(t *testing.T) {
	want := errors.New("listener table unavailable")
	_, got := discoverListeners(context.Background(), staticListenerScanner{err: want}, 0)
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

func TestNativeScannerFindsLoopbackListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	listeners, err := discoverListeners(context.Background(), newListenerScanner(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, found := range listeners {
		if found.Port == port {
			if found.ListensBeyondLoopback {
				t.Fatalf("loopback listener %d marked beyond loopback", port)
			}
			return
		}
	}
	t.Fatalf("loopback listener %d not found in %#v", port, listeners)
}

func TestParseProcListeners(t *testing.T) {
	fixture := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 111 1 0000000000000000 100 0 0 10 0
   1: 00000000:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 222 1 0000000000000000 100 0 0 10 0
   2: 0100007F:0050 00000000:0000 01 00000000:00000000 00:00000000 00000000  1000        0 333 1 0000000000000000 100 0 0 10 0
   malformed
`
	got, err := parseProcListeners(strings.NewReader(fixture), false)
	if err != nil {
		t.Fatal(err)
	}
	want := []procListener{
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 8080, Inode: "111"},
		{Address: netip.IPv4Unspecified(), Port: 3000, Inode: "222"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeners:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestParseProcListenersIPv6(t *testing.T) {
	fixture := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:0FA0 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 444 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:1388 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 555 1 0000000000000000 100 0 0 10 0
`
	got, err := parseProcListeners(strings.NewReader(fixture), true)
	if err != nil {
		t.Fatal(err)
	}
	want := []procListener{
		{Address: netip.IPv6Loopback(), Port: 4000, Inode: "444"},
		{Address: netip.IPv6Unspecified(), Port: 5000, Inode: "555"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeners:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestParseLsofListeners(t *testing.T) {
	fixture := "p101\x00cweb\x00tIPv4\x00n*:3000\x00n127.0.0.1:8080\x00\n" +
		"p202\x00tIPv6\x00n*:5000\x00n[::1]:4000\x00ninvalid\x00\n"
	got, err := parseLsofListeners(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []rawListener{
		{Address: netip.IPv4Unspecified(), Port: 3000, PID: 101, Process: "web"},
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 8080, PID: 101, Process: "web"},
		{Address: netip.IPv6Unspecified(), Port: 5000, PID: 202},
		{Address: netip.IPv6Loopback(), Port: 4000, PID: 202},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeners:\n got: %#v\nwant: %#v", got, want)
	}
}
