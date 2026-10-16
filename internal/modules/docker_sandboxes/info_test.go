package docker_sandboxes

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/app"
	"github.com/zelport/zelport/internal/sbx"
	"net/http/httptest"
	"strings"
	"testing"
)

type infoBackend struct {
	fakeBackend
	probes int
}

func (b *infoBackend) Info(context.Context) sbx.CLIInfo {
	b.probes++
	return sbx.CLIInfo{Status: "found", Path: "/test/sbx", Version: "v0.48.0"}
}
func TestCLIInfoIsCachedUntilManualRefresh(t *testing.T) {
	b := &infoBackend{}
	mod := New().(*Module)
	mod.Backend = b
	manager, err := app.New("", 8080, mod)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	mod.Refresh(context.Background())
	mod.Refresh(context.Background())
	mod.op.Lock()
	first := b.probes
	mod.op.Unlock()
	if first != 1 {
		t.Fatalf("periodic refresh probed CLI %d times", first)
	}
	b.mu.Lock()
	b.fail = true
	b.mu.Unlock()
	mod.Refresh(context.Background())
	var state inventory
	json.Unmarshal(mod.State().Data, &state)
	if state.CLI.Version != "v0.48.0" || !state.Stale || state.Error == "" {
		t.Fatalf("%+v", state)
	}
	w := httptest.NewRecorder()
	mod.Admin().ServeHTTP(w, httptest.NewRequest("POST", "/refresh", strings.NewReader(`{}`)))
	mod.op.Lock()
	second := b.probes
	mod.op.Unlock()
	if w.Code != 200 || second != 2 {
		t.Fatalf("manual refresh: %d probes=%d", w.Code, second)
	}
}
