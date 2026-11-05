package local_ports

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

const sandboxListing = `[
 {"name":"demo-web","status":"running","ports":[{"host_ip":"127.0.0.1","host_port":5173,"sandbox_port":3000,"protocol":"tcp"},{"host_ip":"::1","host_port":5173,"sandbox_port":3000,"protocol":"tcp"}]},
 {"name":"demo-api","status":"running","ports":[{"host_ip":"127.0.0.1","host_port":49153,"sandbox_port":8080,"protocol":"tcp"}]},
 {"name":"stopped","status":"stopped"}
]`

func fakeSandboxResolver(output string, calls *int, clock *time.Time) *sandboxResolver {
	r := newSandboxResolver()
	r.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	r.stat = func(path string) error {
		if path == "/opt/homebrew/bin/sbx" {
			return nil
		}
		return os.ErrNotExist
	}
	r.now = func() time.Time { return *clock }
	r.run = func(_ context.Context, path string, args []string, stdout, _ io.Writer) error {
		*calls++
		if path != "/opt/homebrew/bin/sbx" || !reflect.DeepEqual(args, []string{"ls", "--json"}) {
			return errors.New("unexpected command")
		}
		_, err := io.WriteString(stdout, output)
		return err
	}
	return r
}

func TestLabelSandboxesNamesSbxPorts(t *testing.T) {
	calls, clock := 0, time.Unix(100, 0)
	resolver := fakeSandboxResolver(sandboxListing, &calls, &clock)
	listeners := []Listener{
		{Port: 5173, Processes: []ProcessIdentity{{PID: 7, Name: "sbx"}}},
		{Port: 5411, Processes: []ProcessIdentity{{PID: 7, Name: "sbx"}}},
		{Port: 8080, Processes: []ProcessIdentity{{PID: 9, Name: "node"}}},
		{Port: 49153, Processes: []ProcessIdentity{{PID: 7, Name: "sbx"}}},
	}
	labelSandboxes(context.Background(), resolver, listeners)
	got := []string{listeners[0].Processes[0].Sandbox, listeners[1].Processes[0].Sandbox, listeners[2].Processes[0].Sandbox, listeners[3].Processes[0].Sandbox}
	if want := []string{"demo-web", "", "", "demo-api"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sandboxes = %q, want %q", got, want)
	}
	labelSandboxes(context.Background(), resolver, listeners)
	if calls != 1 {
		t.Fatalf("sbx calls = %d, want 1 while cached", calls)
	}
	resolver.Invalidate()
	labelSandboxes(context.Background(), resolver, listeners)
	clock = clock.Add(11 * time.Second)
	labelSandboxes(context.Background(), resolver, listeners)
	if calls != 3 {
		t.Fatalf("sbx calls = %d, want 3 after invalidate and expiry", calls)
	}
}

func TestLabelSandboxesSkipsSbxWithoutSbxListeners(t *testing.T) {
	calls, clock := 0, time.Unix(100, 0)
	resolver := fakeSandboxResolver(sandboxListing, &calls, &clock)
	listeners := []Listener{{Port: 5173, Processes: []ProcessIdentity{{PID: 9, Name: "node"}}}}
	labelSandboxes(context.Background(), resolver, listeners)
	if calls != 0 || listeners[0].Processes[0].Sandbox != "" {
		t.Fatalf("calls = %d, sandbox = %q, want no sbx call", calls, listeners[0].Processes[0].Sandbox)
	}
}

func TestLabelSandboxesFallsBackSilently(t *testing.T) {
	calls, clock := 0, time.Unix(100, 0)
	resolver := fakeSandboxResolver("not json", &calls, &clock)
	listeners := []Listener{{Port: 5173, Processes: []ProcessIdentity{{PID: 7, Name: "sbx"}}}}
	labelSandboxes(context.Background(), resolver, listeners)
	resolver.stat = func(string) error { return os.ErrNotExist }
	resolver.Invalidate()
	labelSandboxes(context.Background(), resolver, listeners)
	if listeners[0].Processes[0].Sandbox != "" || listeners[0].Processes[0].Name != "sbx" {
		t.Fatalf("listener = %#v, want plain sbx", listeners[0])
	}
}

func TestParseSandboxPortsAcceptsWrappedList(t *testing.T) {
	got := parseSandboxPorts([]byte(`{"sandboxes":[{"name":"demo","ports":[{"host_port":8765}]}]}`))
	if want := map[int]string{8765: "demo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ports = %#v, want %#v", got, want)
	}
}
