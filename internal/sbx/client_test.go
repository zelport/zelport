package sbx

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestPortBindingPreservesPublishedScope(t *testing.T) {
	c := Client{Run: func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "ls" {
			return []byte(`[{"id":"box-id","name":"box","status":"running"}]`), nil
		}
		return []byte(`[{"host_ip":"0.0.0.0","host_port":49152,"sandbox_port":8080,"protocol":"tcp4"}]`), nil
	}}
	ports, err := c.Ports(context.Background(), "box-id")
	if err != nil || len(ports) != 1 {
		t.Fatalf("ports = %+v, error = %v", ports, err)
	}
	binding, ok := ports[0].Binding()
	if !ok || binding.String() != "0.0.0.0" || ports[0].HostIP != "127.0.0.1" || ports[0].Address() != "127.0.0.1:49152" {
		t.Fatalf("port = %+v, binding = %v, %v", ports[0], binding, ok)
	}
}

func TestPortBindingCompatibilityWithoutPublishedIP(t *testing.T) {
	port := Port{HostIP: "::1", HostPort: 49152, SandboxPort: 8080, Protocol: "tcp6"}
	binding, ok := port.Binding()
	if !ok || binding.String() != "::1" || port.Address() != "[::1]:49152" {
		t.Fatalf("legacy port = %+v, binding = %v, %v", port, binding, ok)
	}
}

func TestRealWireFormat(t *testing.T) {
	c := Client{Run: func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "ls" {
			return os.ReadFile("testdata/list.json")
		}
		return os.ReadFile("testdata/ports.json")
	}}
	list, err := c.List(context.Background())
	if err != nil || len(list) != 2 || list[0].ID != "8358da49-f3c6-4d41-93c6-769252f86125" || list[1].Status != "stopped" {
		t.Fatalf("%+v %v", list, err)
	}
	ports, err := c.Ports(context.Background(), list[0].ID)
	if err != nil || len(ports) != 2 || ports[0].HostPort != 5173 {
		t.Fatalf("%+v %v", ports, err)
	}
}
func TestCommandsAndFailures(t *testing.T) {
	var calls [][]string
	c := Client{Run: func(_ context.Context, a ...string) ([]byte, error) {
		if a[0] == "ls" {
			return os.ReadFile("testdata/list.json")
		}
		calls = append(calls, a)
		return nil, nil
	}}
	if err := c.Publish(context.Background(), "8358da49-f3c6-4d41-93c6-769252f86125", 3000); err != nil {
		t.Fatal(err)
	}
	if err := c.Unpublish(context.Background(), "8358da49-f3c6-4d41-93c6-769252f86125", Port{HostIP: "::1", HostPort: 45001, SandboxPort: 3000, Protocol: "tcp"}); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"ports", "project", "--publish", "3000"}, {"ports", "project", "--unpublish", "[::1]:45001:3000/tcp"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("%v", calls)
	}
	for _, raw := range []string{`{}`, `null`, `{"sandboxes":[{"name":"x"}]}`} {
		c.Run = func(context.Context, ...string) ([]byte, error) { return []byte(raw), nil }
		if _, err := c.List(context.Background()); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	c.Run = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("login unavailable") }
	if _, err := c.List(context.Background()); err == nil {
		t.Fatal("lost CLI error")
	}
}
func TestCommandTimeout(t *testing.T) {
	c := Client{Timeout: time.Millisecond, Run: func(ctx context.Context, _ ...string) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }}
	if _, err := c.List(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
}

func TestMissingIdentityNeverUsesRecycledName(t *testing.T) {
	c := Client{Run: func(_ context.Context, a ...string) ([]byte, error) {
		if a[0] != "ls" {
			t.Fatalf("unexpected mutation: %v", a)
		}
		return os.ReadFile("testdata/list.json")
	}}
	if err := c.Publish(context.Background(), "removed-uuid", 3000); err == nil {
		t.Fatal("published missing sandbox")
	}
}

func TestAutomaticPublishDoesNotStartStoppedSandbox(t *testing.T) {
	c := Client{Run: func(_ context.Context, a ...string) ([]byte, error) {
		if a[0] != "ls" {
			t.Fatalf("stopped sandbox was published: %v", a)
		}
		return os.ReadFile("testdata/list.json")
	}}
	if err := c.Publish(context.Background(), "8207ee90-a8e8-4d4d-a858-29dd5a110fd6", 3000); err == nil {
		t.Fatal("expected stopped-state rejection")
	}
}

func TestLiveReadOnly(t *testing.T) {
	if os.Getenv("ZELPORT_LIVE_TEST") != "1" {
		t.Skip("set ZELPORT_LIVE_TEST=1 to read real Docker Sandboxes")
	}
	c := Client{}
	boxes, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range boxes {
		ports, err := c.Ports(context.Background(), box.ID)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s, %d mappings", box.ID, box.Status, len(ports))
	}
}

func TestListFormats(t *testing.T) {
	for _, file := range []string{"list.json", "list-array.json"} {
		t.Run(file, func(t *testing.T) {
			c := Client{Run: func(_ context.Context, args ...string) ([]byte, error) {
				if !reflect.DeepEqual(args, []string{"ls", "--json"}) {
					t.Fatalf("unexpected arguments: %v", args)
				}
				return os.ReadFile("testdata/" + file)
			}}
			boxes, err := c.List(context.Background())
			if err != nil || len(boxes) != 2 {
				t.Fatalf("boxes=%v error=%v", boxes, err)
			}
		})
	}
	for _, raw := range []string{`[]`, `{"sandboxes":[]}`} {
		c := Client{Run: func(context.Context, ...string) ([]byte, error) { return []byte(raw), nil }}
		boxes, err := c.List(context.Background())
		if err != nil || len(boxes) != 0 {
			t.Fatalf("%s: %v %v", raw, boxes, err)
		}
	}
	for _, raw := range []string{`null`, `{}`, `{"sandboxes":null}`, `{"sandboxes":{}}`, `[null]`, `[{"id":"a"}]`, `[{"id":"a","name":"x","status":"running"},{"id":"a","name":"y","status":"running"}]`, `[] trailing`, `"text"`} {
		c := Client{Run: func(context.Context, ...string) ([]byte, error) { return []byte(raw), nil }}
		if _, err := c.List(context.Background()); err == nil {
			t.Fatalf("accepted invalid list: %s", raw)
		}
	}
}
