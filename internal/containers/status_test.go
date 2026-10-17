package containers

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestMissingCLIStopsBeforeRuntime(t *testing.T) {
	for _, engine := range []string{"docker", "podman", "nerdctl"} {
		called := false
		inv := (Client{Engine: engine, LookPath: func(string) (string, error) { return "", exec.ErrNotFound }, Run: func(context.Context, string, ...string) ([]byte, error) { called = true; return nil, nil }}).Scan(context.Background())
		if called || inv.CLIStatus != "not_found" || inv.RuntimeStatus != "not_checked" || !inv.Stale {
			t.Fatalf("%s: %+v called=%v", engine, inv, called)
		}
	}
}
func TestRuntimeAndDiscoveryFailuresAreDistinct(t *testing.T) {
	for _, runtimeFails := range []bool{true, false} {
		c := Client{Engine: "docker", LookPath: func(string) (string, error) { return "/test/docker", nil }, Env: func(k string) string {
			if k == "DOCKER_HOST" {
				return "unix:///test/docker.sock"
			}
			return ""
		}, Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "info --format") && !runtimeFails {
				return []byte(`{"ServerVersion":"27"}`), nil
			}
			return nil, errors.New("test failure")
		}}
		inv := c.Scan(context.Background())
		want := "available"
		if runtimeFails {
			want = "unavailable"
		}
		if inv.CLIStatus != "found" || inv.RuntimeStatus != want || !inv.Stale || inv.Error == "" {
			t.Fatalf("%+v", inv)
		}
	}
}
func TestNerdctlIdentityIncludesNamespaceAndDaemon(t *testing.T) {
	ns, daemon := "default", "daemon-one"
	c := Client{Engine: "nerdctl", Platform: "linux", LookPath: func(string) (string, error) { return "/test/nerdctl", nil }, Env: func(string) string { return "" }, Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		if strings.Contains(call, "info --mode native") {
			return []byte(`{"Namespace":"` + ns + `","Daemon":{"Server":{"UUID":"` + daemon + `"}}}`), nil
		}
		if call == "--namespace "+ns+" ps --all --no-trunc --quiet" {
			return []byte(strings.Repeat("a", 64)), nil
		}
		if strings.HasPrefix(call, "--namespace "+ns+" inspect --mode dockercompat --format {{json .}}") {
			return []byte(`{"Id":"` + strings.Repeat("a", 64) + `","Name":"web","State":{"Status":"running"},"NetworkSettings":{"Ports":{"80/tcp":[{"HostIp":"127.0.0.1","HostPort":"8088"}]}}}`), nil
		}
		t.Fatalf("unexpected call %s", call)
		return nil, nil
	}}
	a := c.Scan(context.Background())
	if a.Stale || a.RuntimeStatus != "available" || len(a.Containers) != 1 || len(a.Containers[0].Ports) != 1 {
		t.Fatalf("%+v", a)
	}
	ns = "other"
	b := c.Scan(context.Background())
	if b.Stale || a.Connection == b.Connection {
		t.Fatal("namespace not isolated", b)
	}
	daemon = "daemon-two"
	d := c.Scan(context.Background())
	if d.Stale || b.Connection == d.Connection {
		t.Fatal("daemon not isolated", d)
	}
}

func TestNerdctlRejectsChangingDaemonAndUnverifiedVM(t *testing.T) {
	calls := 0
	c := Client{Engine: "nerdctl", Platform: "darwin", LookPath: func(string) (string, error) { return "/test/nerdctl", nil }, Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "info --mode native") {
			calls++
			id := "one"
			if calls > 1 {
				id = "two"
			}
			return []byte(`{"Namespace":"default","Daemon":{"Server":{"UUID":"` + id + `"}}}`), nil
		}
		return []byte(""), nil
	}}
	inv := c.Scan(context.Background())
	if calls != 0 || !inv.Stale || inv.RuntimeStatus != "not_checked" {
		t.Fatalf("unverified VM accepted: %+v", inv)
	}
	c.Platform = "linux"
	inv = c.Scan(context.Background())
	if !inv.Stale || !strings.Contains(inv.Error, "changed") || len(inv.Containers) != 0 {
		t.Fatalf("changed daemon accepted: %+v", inv)
	}
}
func TestMissingCLIRetainsLastKnownInventoryAndRecovers(t *testing.T) {
	inv := Inventory{Engine: "nerdctl", Connection: "known", CLIStatus: "found", RuntimeStatus: "available", Containers: []Container{{ID: "known-container"}}}
	catalog := NewCatalog(scanFunc(func(context.Context) Inventory { return inv }))
	catalog.Refresh(context.Background())
	inv = Inventory{Engine: "nerdctl", CLIStatus: "not_found", RuntimeStatus: "not_checked", Stale: true, Error: "CLI removed"}
	catalog.Refresh(context.Background())
	snapshot := catalog.Snapshot()[0]
	if snapshot.Connection != "known" || len(snapshot.Containers) != 1 || !snapshot.Stale || snapshot.CLIStatus != "not_found" {
		t.Fatalf("%+v", snapshot)
	}
	inv = Inventory{Engine: "nerdctl", Connection: "new", CLIStatus: "found", RuntimeStatus: "available", Containers: []Container{}}
	catalog.Refresh(context.Background())
	if len(catalog.Snapshot()[0].Containers) != 0 || catalog.Snapshot()[0].Stale {
		t.Fatal("reused old inventory after identity change")
	}
}

func TestRuntimePermissionDenied(t *testing.T) {
	for _, tc := range []struct {
		name, message, want string
	}{
		{"docker socket", "docker: permission denied while trying to connect to the Docker API at unix:///var/run/docker.sock", "permission_denied"},
		{"socket dial", "dial unix /run/user/1000/podman/podman.sock: connect: permission denied", "permission_denied"},
		{"access denied", "Access is denied", "permission_denied"},
		{"offline", "Cannot connect to the Docker daemon at unix:///var/run/docker.sock", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Client{Engine: "docker", LookPath: func(string) (string, error) { return "/test/docker", nil },
				Env: func(key string) string {
					if key == "DOCKER_HOST" {
						return "unix:///test/docker.sock"
					}
					return ""
				},
				Run: func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New(tc.message) },
			}
			inv := c.Scan(context.Background())
			if inv.RuntimeStatus != tc.want || inv.CLIStatus != "found" || !inv.Stale || inv.Error != tc.message {
				t.Fatalf("unexpected permission status: %+v", inv)
			}
		})
	}
}
