package containers

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestPortParsers(t *testing.T) {
	d, e := parseDocker(fixture(t, "docker-inspect.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	if len(d) != 2 || len(d[0].Ports) != 2 || d[0].Ports[0].HostIP != "127.0.0.1" || len(d[1].Ports) != 1 {
		t.Fatalf("%+v", d)
	}
	p, e := parsePodman(fixture(t, "podman.json"))
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 7 || len(p[0].Ports) != 3 || p[0].Ports[0].HostIP != "127.0.0.1" || p[1].Status != "exited" {
		t.Fatalf("%+v", p)
	}
	for _, b := range [][]byte{[]byte(`null`), []byte(`{}`), []byte(`[{"Id":"short"}]`)} {
		if _, e = parsePodman(b); e == nil {
			t.Fatalf("accepted %s", b)
		}
	}
	if _, e = parseDocker([]byte(`{"Id":"short"}`)); e == nil {
		t.Fatal("accepted short ID")
	}
}
func TestDockerEndpointPinning(t *testing.T) {
	calls := []string{}
	c := Client{Engine: "docker", LookPath: func(name string) (string, error) { return name, nil }, Env: func(string) string { return "" }, Run: func(ctx context.Context, engine string, args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "context show":
			return []byte("desktop-linux\n"), nil
		case "context inspect desktop-linux":
			return []byte(`[{"Endpoints":{"docker":{"Host":"unix:///tmp/docker.sock"}}}]`), nil
		case "--host unix:///tmp/docker.sock info --format {{json .}}":
			return []byte(`{"ServerVersion":"27"}`), nil
		case "--host unix:///tmp/docker.sock ps --all --no-trunc --quiet":
			return []byte(strings.Repeat("a", 64) + "\n" + strings.Repeat("b", 64)), nil
		}
		if strings.HasPrefix(call, "--host unix:///tmp/docker.sock container inspect --format {{json .}}") {
			return fixture(t, "docker-inspect.jsonl"), nil
		}
		t.Fatalf("unexpected call %s", call)
		return nil, nil
	}}
	inv := c.Scan(context.Background())
	if inv.Stale || len(inv.Containers) != 2 || inv.Connection == "" {
		t.Fatalf("%+v calls %v", inv, calls)
	}
	c.Env = func(k string) string {
		if k == "DOCKER_HOST" {
			return "ssh://remote"
		}
		return ""
	}
	inv = c.Scan(context.Background())
	if !inv.Stale || !strings.Contains(inv.Error, "local") {
		t.Fatalf("remote accepted %+v", inv)
	}
}
func TestPodmanConnectionAndTimeout(t *testing.T) {
	conn := []byte(`[{"Name":"machine","URI":"ssh://core@127.0.0.1:1234/run/podman.sock","Identity":"/tmp/key file","IsMachine":true,"Default":true}]`)
	c := Client{Engine: "podman", LookPath: func(name string) (string, error) { return name, nil }, Env: func(string) string { return "" }, Run: func(ctx context.Context, engine string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "system connection list --format json" {
			return conn, nil
		}
		if len(args) < 4 || args[0] != "--url" || args[1] != "ssh://core@127.0.0.1:1234/run/podman.sock" || args[2] != "--identity" || args[3] != "/tmp/key file" {
			t.Fatalf("un-pinned args %v", args)
		}
		if strings.Contains(strings.Join(args, " "), "info --format") {
			return []byte(`{"host":{}}`), nil
		}
		return fixture(t, "podman.json"), nil
	}}
	inv := c.Scan(context.Background())
	if inv.Stale || len(inv.Containers) != 7 {
		t.Fatalf("%+v", inv)
	}
	conn = []byte(`[{"Name":"remote","URI":"ssh://user@example.com/run/podman.sock","Default":true}]`)
	if inv = c.Scan(context.Background()); !inv.Stale || inv.Connection == "" {
		t.Fatal(inv)
	}
	c.Timeout = time.Millisecond
	c.Run = func(ctx context.Context, _ string, _ ...string) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }
	if inv = c.Scan(context.Background()); !inv.Stale || !strings.Contains(inv.Error, "deadline") {
		t.Fatal(inv)
	}
}

type scanFunc func(context.Context) Inventory

func (f scanFunc) Scan(ctx context.Context) Inventory { return f(ctx) }
func TestCatalogIsolationAndStaleState(t *testing.T) {
	release := make(chan struct{})
	fastDone := make(chan struct{})
	slow := scanFunc(func(ctx context.Context) Inventory {
		<-release
		return Inventory{Engine: "docker", Stale: true, Error: "offline"}
	})
	fast := scanFunc(func(ctx context.Context) Inventory {
		defer close(fastDone)
		return Inventory{Engine: "podman", Connection: "one", Containers: []Container{{ID: "one"}}, UpdatedAt: time.Now()}
	})
	catalog := NewCatalog(slow, fast)
	done := make(chan struct{})
	go func() { catalog.Refresh(context.Background()); close(done) }()
	<-fastDone
	deadline := time.Now().Add(time.Second)
	for len(catalog.Snapshot()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(catalog.Snapshot()) == 0 {
		t.Fatal("healthy source blocked by slow engine")
	}
	close(release)
	<-done
	inv := Inventory{Engine: "podman", Connection: "one", Containers: []Container{{ID: "one"}}}
	c := NewCatalog(scanFunc(func(context.Context) Inventory { return inv }))
	c.Refresh(context.Background())
	inv = Inventory{Engine: "podman", Connection: "one", Stale: true, Error: errors.New("offline").Error()}
	c.Refresh(context.Background())
	if len(c.Snapshot()[0].Containers) != 1 {
		t.Fatal("lost known containers")
	}
	inv.Connection = "two"
	c.Refresh(context.Background())
	if len(c.Snapshot()[0].Containers) != 0 {
		t.Fatal("reused containers across connections")
	}
}
func TestLiveReadOnly(t *testing.T) {
	if os.Getenv("ZELPORT_CONTAINER_LIVE") != "1" {
		t.Skip("opt-in")
	}
	inv := (Client{Engine: "podman"}).Scan(context.Background())
	if inv.Stale {
		t.Fatal(inv.Error)
	}
	t.Logf("%s: %d containers", inv.Label, len(inv.Containers))
}
