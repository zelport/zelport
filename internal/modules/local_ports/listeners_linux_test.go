//go:build linux

package local_ports

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcScannerKeepsListenersWhenProcessMetadataIsPartial(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "net/tcp"), `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 111 1 0000000000000000 100 0 0 10 0
   1: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 222 1 0000000000000000 100 0 0 10 0
`)
	writeTestFile(t, filepath.Join(root, "net/tcp6"), `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:0FA0 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 333 1 0000000000000000 100 0 0 10 0
`)
	writeTestFile(t, filepath.Join(root, "123/comm"), "web\n")
	if err := os.MkdirAll(filepath.Join(root, "123/fd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("socket:[111]", filepath.Join(root, "123/fd/5")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "124/comm"), "hidden\n")
	if err := os.MkdirAll(filepath.Join(root, "124/fd"), 0o000); err != nil {
		t.Fatal(err)
	}

	got, err := (procScanner{root: root, maxRecords: 10}).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []rawListener{
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 8080, PID: 123, Process: "web"},
		{Address: netip.MustParseAddr("127.0.0.1"), Port: 3000},
		{Address: netip.IPv6Loopback(), Port: 4000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeners:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestProcScannerStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (procScanner{root: t.TempDir(), maxRecords: 10}).Scan(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}
