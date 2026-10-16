package sbx

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func TestCLIInfo(t *testing.T) {
	c := Client{LookPath: func(name string) (string, error) { return "/test/sbx", nil }, Run: func(_ context.Context, args ...string) ([]byte, error) {
		if !reflect.DeepEqual(args, []string{"version", "--json"}) {
			t.Fatalf("unexpected command: %v", args)
		}
		return []byte(`{"client":{"version":"v0.48.0","revision":"abc"},"server":{"state":"running"}}`), nil
	}}
	info := c.Info(context.Background())
	if info.Status != "found" || info.Path != "/test/sbx" || info.Version != "v0.48.0" || info.Error != "" {
		t.Fatalf("%+v", info)
	}
	c.LookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	c.Run = func(context.Context, ...string) ([]byte, error) { t.Fatal("missing CLI was executed"); return nil, nil }
	if got := c.Info(context.Background()); got.Status != "not_found" || got.Error == "" {
		t.Fatalf("%+v", got)
	}
	c.LookPath = func(string) (string, error) { return "/test/sbx", nil }
	for _, raw := range []string{`{}`, `not json`, `{"client":{"version":""}}`} {
		c.Run = func(context.Context, ...string) ([]byte, error) { return []byte(raw), nil }
		if got := c.Info(context.Background()); got.Status != "found" || got.Version != "" || got.Error == "" {
			t.Fatalf("%+v", got)
		}
	}
	c.Timeout = time.Millisecond
	c.Run = func(ctx context.Context, _ ...string) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }
	if got := c.Info(context.Background()); got.Error != context.DeadlineExceeded.Error() {
		t.Fatalf("%+v", got)
	}
	c.Run = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("version failed") }
	if got := c.Info(context.Background()); got.Status != "found" || got.Error != "version failed" {
		t.Fatalf("%+v", got)
	}
}
