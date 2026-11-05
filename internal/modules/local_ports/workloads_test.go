package local_ports

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const sandboxListing = `[
 {"name":"demo-web","status":"running","ports":[{"host_ip":"127.0.0.1","host_port":5173,"sandbox_port":3000,"protocol":"tcp"},{"host_ip":"::1","host_port":5173,"sandbox_port":3000,"protocol":"tcp"}]},
 {"name":"demo-api","status":"running","ports":[{"host_ip":"127.0.0.1","host_port":49153,"sandbox_port":8080,"protocol":"tcp"}]},
 {"name":"stopped","status":"stopped"}
]`

const containerListing = "ingress\t0.0.0.0:4321->4321/tcp, 0.0.0.0:5174->5174/tcp, 0.0.0.0:4321->4321/udp, 80/tcp\n" +
	"dummy-1212\t127.0.0.1:1212->80/tcp\n" +
	"cache\t[::]:6379->6379/tcp, :::6380->6379/tcp\n" +
	"range\t0.0.0.0:9000-9002->9000-9002/tcp\n" +
	"quiet\t\n"

type fakeRuntime struct {
	mu     sync.Mutex
	calls  map[string]int
	output map[string]string
}

func (f *fakeRuntime) count(binary string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[binary]
}

func fakeWorkloadResolver(runtime *fakeRuntime, clock *time.Time) *workloadResolver {
	r := newWorkloadResolver()
	// Only the fallback paths exist: the PATH of a launchd service is short.
	r.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	r.stat = func(path string) error {
		switch path {
		case "/opt/homebrew/bin/sbx", "/opt/podman/bin/podman", "/usr/local/bin/docker":
			return nil
		}
		return os.ErrNotExist
	}
	r.now = func() time.Time { return *clock }
	r.run = func(_ context.Context, path string, args []string, stdout, _ io.Writer) error {
		binary := path[strings.LastIndex(path, "/")+1:]
		runtime.mu.Lock()
		if runtime.calls == nil {
			runtime.calls = map[string]int{}
		}
		runtime.calls[binary]++
		output, ok := runtime.output[binary]
		runtime.mu.Unlock()
		want := []string{"ps", "--format", "{{.Names}}\t{{.Ports}}"}
		if binary == "sbx" {
			want = []string{"ls", "--json"}
		}
		if !ok || !reflect.DeepEqual(args, want) {
			return errors.New("unexpected command")
		}
		_, err := io.WriteString(stdout, output)
		return err
	}
	return r
}

func owned(port int, pid int, name string) Listener {
	return Listener{Port: port, Processes: []ProcessIdentity{{PID: pid, Name: name}}}
}

func TestWorkloadResolverNamesSandboxesAndContainers(t *testing.T) {
	runtime, clock := &fakeRuntime{output: map[string]string{"sbx": sandboxListing, "podman": containerListing}}, time.Unix(100, 0)
	resolver := fakeWorkloadResolver(runtime, &clock)
	listeners := []Listener{
		owned(1212, 4, "gvproxy"),
		owned(4321, 4, "gvproxy"),
		owned(5173, 7, "sbx"),
		owned(5411, 7, "sbx"),
		owned(8080, 9, "node"),
		owned(9001, 4, "gvproxy"),
		owned(58581, 4, "gvproxy"),
	}
	resolver.label(context.Background(), listeners)
	var got []string
	for _, listener := range listeners {
		got = append(got, listener.Processes[0].Sandbox+"|"+listener.Processes[0].Container)
	}
	want := []string{"|dummy-1212", "|ingress", "demo-web|", "|", "|", "|range", "|"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("labels = %q, want %q", got, want)
	}
	if runtime.count("docker") != 0 {
		t.Fatalf("docker was asked without a docker listener")
	}
	resolver.label(context.Background(), listeners)
	if runtime.count("sbx") != 1 || runtime.count("podman") != 1 {
		t.Fatalf("calls = %v, want one per runtime while cached", runtime.calls)
	}
	resolver.Invalidate()
	resolver.label(context.Background(), listeners)
	clock = clock.Add(11 * time.Second)
	resolver.label(context.Background(), listeners)
	if runtime.count("sbx") != 3 || runtime.count("podman") != 3 {
		t.Fatalf("calls = %v, want 3 after invalidate and expiry", runtime.calls)
	}
}

func TestWorkloadResolverMatchesDockerDesktopProcesses(t *testing.T) {
	runtime, clock := &fakeRuntime{output: map[string]string{"docker": containerListing}}, time.Unix(100, 0)
	resolver := fakeWorkloadResolver(runtime, &clock)
	listeners := []Listener{owned(6379, 3, "com.docker.backend"), owned(6380, 3, "docker-proxy")}
	resolver.label(context.Background(), listeners)
	if listeners[0].Processes[0].Container != "cache" || listeners[1].Processes[0].Container != "cache" {
		t.Fatalf("listeners = %#v, want cache", listeners)
	}
}

func TestWorkloadResolverRunsNothingForPlainProcesses(t *testing.T) {
	runtime, clock := &fakeRuntime{output: map[string]string{}}, time.Unix(100, 0)
	resolver := fakeWorkloadResolver(runtime, &clock)
	resolver.label(context.Background(), []Listener{owned(5173, 9, "node"), {Port: 9000}})
	if len(runtime.calls) != 0 {
		t.Fatalf("calls = %v, want none", runtime.calls)
	}
}

func TestWorkloadResolverFallsBackSilently(t *testing.T) {
	runtime, clock := &fakeRuntime{output: map[string]string{"sbx": "not json"}}, time.Unix(100, 0)
	resolver := fakeWorkloadResolver(runtime, &clock)
	listeners := []Listener{owned(5173, 7, "sbx"), owned(1212, 4, "gvproxy")}
	resolver.label(context.Background(), listeners)
	resolver.stat = func(string) error { return os.ErrNotExist }
	resolver.Invalidate()
	resolver.label(context.Background(), listeners)
	if listeners[0].Processes[0] != (ProcessIdentity{PID: 7, Name: "sbx"}) || listeners[1].Processes[0] != (ProcessIdentity{PID: 4, Name: "gvproxy"}) {
		t.Fatalf("listeners = %#v, want plain processes", listeners)
	}
}

func TestParseSandboxPortsAcceptsWrappedList(t *testing.T) {
	got := parseSandboxPorts([]byte(`{"sandboxes":[{"name":"demo","ports":[{"host_port":8765}]}]}`))
	if want := map[int]string{8765: "demo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ports = %#v, want %#v", got, want)
	}
}

func TestParseContainerPorts(t *testing.T) {
	got := parseContainerPorts([]byte(containerListing))
	want := map[int]string{4321: "ingress", 5174: "ingress", 1212: "dummy-1212", 6379: "cache", 6380: "cache", 9000: "range", 9001: "range", 9002: "range"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ports = %#v, want %#v", got, want)
	}
}
