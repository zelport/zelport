package local_ports

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"testing"
)

type commandExitError struct{ code int }

func (err commandExitError) Error() string { return "command failed" }
func (err commandExitError) ExitCode() int { return err.code }

func TestLsofScannerTreatsNoMatchesAsEmptyInventory(t *testing.T) {
	scanner := lsofScanner{
		stat:     func(string) error { return nil },
		lookPath: func(string) (string, error) { return "", os.ErrNotExist },
		run: func(_ context.Context, _ string, _ []string, _, _ io.Writer) error {
			return commandExitError{code: 1}
		},
		maxOutput: 1024,
	}

	got, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("listeners = %#v, want empty inventory", got)
	}
}

func TestLsofScannerUsesSystemPathAndFieldArguments(t *testing.T) {
	var path string
	var args []string
	lookedUp := false
	scanner := lsofScanner{
		stat: func(candidate string) error {
			if candidate == "/usr/sbin/lsof" {
				return nil
			}
			return os.ErrNotExist
		},
		lookPath: func(string) (string, error) {
			lookedUp = true
			return "/custom/lsof", nil
		},
		run: func(_ context.Context, candidate string, gotArgs []string, stdout, _ io.Writer) error {
			path = candidate
			args = append([]string(nil), gotArgs...)
			_, err := io.WriteString(stdout, "p101\x00cweb\x00n*:3000\x00")
			return err
		},
		maxOutput: 1024,
	}

	got, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != "/usr/sbin/lsof" || lookedUp {
		t.Fatalf("path = %q, PATH lookup = %v", path, lookedUp)
	}
	wantArgs := []string{"-nP", "-iTCP", "-sTCP:LISTEN", "-F0pcn"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
	want := []rawListener{{Address: netip.IPv4Unspecified(), Port: 3000, PID: 101, Process: "web"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeners = %#v, want %#v", got, want)
	}
}

func TestLsofScannerFallsBackToPATH(t *testing.T) {
	scanner := lsofScanner{
		stat: func(string) error { return os.ErrNotExist },
		lookPath: func(name string) (string, error) {
			if name != "lsof" {
				t.Fatalf("lookup = %q", name)
			}
			return "/custom/lsof", nil
		},
		run: func(_ context.Context, path string, _ []string, _, _ io.Writer) error {
			if path != "/custom/lsof" {
				t.Fatalf("path = %q", path)
			}
			return nil
		},
		maxOutput: 1024,
	}
	if _, err := scanner.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLsofScannerBoundsOutputAndSanitizesFailures(t *testing.T) {
	commandFailure := errors.New("secret command diagnostic")
	for _, test := range []struct {
		name string
		run  lsofCommand
	}{
		{name: "output limit", run: func(_ context.Context, _ string, _ []string, stdout, _ io.Writer) error {
			_, err := io.WriteString(stdout, strings.Repeat("x", 9))
			return err
		}},
		{name: "command failure", run: func(context.Context, string, []string, io.Writer, io.Writer) error { return commandFailure }},
	} {
		t.Run(test.name, func(t *testing.T) {
			scanner := lsofScanner{
				stat:      func(string) error { return nil },
				lookPath:  func(string) (string, error) { return "", os.ErrNotExist },
				run:       test.run,
				maxOutput: 8,
			}
			_, err := scanner.Scan(context.Background())
			if !errors.Is(err, ErrListenerDiscovery) {
				t.Fatalf("error = %v, want ErrListenerDiscovery", err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaks command output: %v", err)
			}
		})
	}
}

func TestLsofScannerPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanner := lsofScanner{
		stat:     func(string) error { return nil },
		lookPath: func(string) (string, error) { return "", os.ErrNotExist },
		run: func(ctx context.Context, _ string, _ []string, _, _ io.Writer) error {
			return ctx.Err()
		},
		maxOutput: 8,
	}
	_, err := scanner.Scan(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}
