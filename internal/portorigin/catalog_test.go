package portorigin

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/sbx"
)

type containerScanFunc func(context.Context) containers.Inventory

func (f containerScanFunc) Scan(ctx context.Context) containers.Inventory { return f(ctx) }

type sandboxStub struct {
	info  func(context.Context) sbx.CLIInfo
	list  func(context.Context) ([]sbx.Sandbox, error)
	ports func(context.Context, string) ([]sbx.Port, error)
}

func (s sandboxStub) Info(ctx context.Context) sbx.CLIInfo {
	if s.info == nil {
		return sbx.CLIInfo{Status: "not_found", Error: exec.ErrNotFound.Error()}
	}
	return s.info(ctx)
}
func (s sandboxStub) List(ctx context.Context) ([]sbx.Sandbox, error) {
	if s.list == nil {
		return []sbx.Sandbox{}, nil
	}
	return s.list(ctx)
}
func (s sandboxStub) Ports(ctx context.Context, id string) ([]sbx.Port, error) {
	if s.ports == nil {
		return []sbx.Port{}, nil
	}
	return s.ports(ctx, id)
}

func goodInventory(engine, name string, port int) containers.Inventory {
	return containers.Inventory{
		Engine: engine, Connection: engine + "-local", CLIStatus: "found", RuntimeStatus: "available",
		Containers: []containers.Container{{
			ID: name + "-id", Name: name, Status: "running",
			Ports: []containers.Port{{HostIP: "127.0.0.1", HostPort: port, ContainerPort: 80, Protocol: "tcp"}},
		}}, UpdatedAt: time.Unix(10, 0),
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition was not reached")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("refresh did not finish")
	}
}

func TestCatalogRefreshesProvidersIndependently(t *testing.T) {
	sandboxStarted := make(chan struct{})
	releaseSandbox := make(chan struct{})
	var once sync.Once
	catalog := NewCatalog(
		[]ContainerProvider{containerScanFunc(func(context.Context) containers.Inventory {
			return goodInventory("podman", "dummy-1212", 1212)
		})},
		sandboxStub{
			info: func(context.Context) sbx.CLIInfo { return sbx.CLIInfo{Status: "found", Version: "test"} },
			list: func(ctx context.Context) ([]sbx.Sandbox, error) {
				once.Do(func() { close(sandboxStarted) })
				select {
				case <-releaseSandbox:
					return []sbx.Sandbox{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		},
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catalog.Start(ctx, time.Hour)
	defer catalog.Close()
	<-sandboxStarted

	waitFor(t, func() bool {
		snapshot := catalog.Snapshot()
		return len(snapshot.Containers) == 1 && len(snapshot.Containers[0].Containers) == 1
	})
	snapshot := catalog.Snapshot()
	if snapshot.Containers[0].Containers[0].Name != "dummy-1212" || snapshot.Generation != 0 || snapshot.Health != Refreshing {
		t.Fatalf("partial snapshot = %+v", snapshot)
	}
	// Snapshot data is immutable from the caller's point of view.
	snapshot.Containers[0].Containers[0].Name = "changed"
	if got := catalog.Snapshot().Containers[0].Containers[0].Name; got != "dummy-1212" {
		t.Fatalf("snapshot alias leaked mutation: %q", got)
	}

	close(releaseSandbox)
	waitFor(t, func() bool { return catalog.Snapshot().Generation == 1 })
	if got := catalog.Snapshot().Health; got != Ready {
		t.Fatalf("health = %q, want %q", got, Ready)
	}
}

func TestCatalogPreservesLastKnownGoodAsStale(t *testing.T) {
	var mu sync.Mutex
	next := goodInventory("podman", "dummy-1313", 1313)
	provider := containerScanFunc(func(context.Context) containers.Inventory {
		mu.Lock()
		defer mu.Unlock()
		return next
	})
	catalog := NewCatalog([]ContainerProvider{provider}, sandboxStub{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catalog.Start(ctx, time.Hour)
	defer catalog.Close()
	waitFor(t, func() bool { return catalog.Snapshot().Generation == 1 })

	mu.Lock()
	next = containers.Inventory{Engine: "podman", Connection: "podman-local", CLIStatus: "found", RuntimeStatus: "unavailable", Stale: true, Error: "offline"}
	mu.Unlock()
	waitDone(t, catalog.RequestRefresh())

	snapshot := catalog.Snapshot()
	if snapshot.Health != Stale || len(snapshot.Containers) != 1 || len(snapshot.Containers[0].Containers) != 1 || snapshot.Containers[0].Containers[0].Name != "dummy-1313" {
		t.Fatalf("stale snapshot = %+v", snapshot)
	}
}

func TestCatalogCoalescesConcurrentRefreshRequests(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	provider := containerScanFunc(func(ctx context.Context) containers.Inventory {
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
			return goodInventory("podman", "one", 1200)
		case <-ctx.Done():
			return containers.Inventory{Engine: "podman", Stale: true, Error: ctx.Err().Error()}
		}
	})
	catalog := NewCatalog([]ContainerProvider{provider}, sandboxStub{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catalog.Start(ctx, time.Hour)
	defer catalog.Close()
	<-entered

	one := catalog.RequestRefresh()
	two := catalog.RequestRefresh()
	if one != two {
		t.Fatal("concurrent refreshes did not share one completion channel")
	}
	close(release)
	waitDone(t, one)
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func TestCatalogRecoversProviderInstalledAfterStartup(t *testing.T) {
	var installed atomic.Bool
	provider := containerScanFunc(func(context.Context) containers.Inventory {
		if !installed.Load() {
			return containers.Inventory{Engine: "podman", CLIStatus: "not_found", RuntimeStatus: "not_checked", Stale: true, Error: exec.ErrNotFound.Error()}
		}
		return goodInventory("podman", "recovered", 1400)
	})
	catalog := NewCatalog([]ContainerProvider{provider}, sandboxStub{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catalog.Start(ctx, time.Hour)
	defer catalog.Close()
	waitFor(t, func() bool { return catalog.Snapshot().Generation == 1 })
	if got := catalog.Snapshot().Health; got != Ready {
		t.Fatalf("missing CLI health = %q, want ready", got)
	}

	installed.Store(true)
	waitDone(t, catalog.RequestRefresh())
	snapshot := catalog.Snapshot()
	if snapshot.Health != Ready || len(snapshot.Containers) != 1 || len(snapshot.Containers[0].Containers) != 1 || snapshot.Containers[0].Containers[0].Name != "recovered" {
		t.Fatalf("recovered snapshot = %+v", snapshot)
	}
}

func TestCatalogCloseCancelsBlockedProviders(t *testing.T) {
	entered := make(chan struct{})
	returned := make(chan struct{})
	provider := containerScanFunc(func(ctx context.Context) containers.Inventory {
		close(entered)
		<-ctx.Done()
		close(returned)
		return containers.Inventory{Engine: "podman", Stale: true, Error: ctx.Err().Error()}
	})
	catalog := NewCatalog([]ContainerProvider{provider}, sandboxStub{
		info: func(context.Context) sbx.CLIInfo { return sbx.CLIInfo{Status: "found"} },
		list: func(context.Context) ([]sbx.Sandbox, error) { return nil, errors.New("unavailable") },
	})
	catalog.Start(context.Background(), time.Hour)
	<-entered

	closed := make(chan struct{})
	go func() {
		_ = catalog.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not join blocked provider")
	}
	select {
	case <-returned:
	default:
		t.Fatal("provider did not observe cancellation")
	}
}
