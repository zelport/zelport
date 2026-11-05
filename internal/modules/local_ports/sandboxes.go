package local_ports

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Docker Sandboxes publish every sandbox port from one `sbx daemon` process, so
// the process name alone says "sbx". The sandbox behind a port comes from one
// `sbx ls --json` call, made only when a listener belongs to sbx and cached briefly.
type sandboxResolver struct {
	stat      func(string) error
	lookPath  func(string) (string, error)
	run       lsofCommand
	now       func() time.Time
	ttl       time.Duration
	timeout   time.Duration
	maxOutput int64

	mu      sync.Mutex
	ports   map[int]string
	checked time.Time
}

func newSandboxResolver() *sandboxResolver {
	return &sandboxResolver{
		stat: func(path string) error {
			_, err := os.Stat(path)
			return err
		},
		lookPath: exec.LookPath,
		run: func(ctx context.Context, path string, args []string, stdout, stderr io.Writer) error {
			command := exec.CommandContext(ctx, path, args...)
			command.Stdout = stdout
			command.Stderr = stderr
			return command.Run()
		},
		now:       time.Now,
		ttl:       10 * time.Second,
		timeout:   time.Second,
		maxOutput: 1 << 20,
	}
}

func isSandboxDaemon(name string) bool { return strings.EqualFold(name, "sbx") }

// Invalidate makes the next lookup ask sbx again (Rescan ports).
func (r *sandboxResolver) Invalidate() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.checked = time.Time{}
	r.mu.Unlock()
}

func (r *sandboxResolver) lookup(ctx context.Context) map[int]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.checked.IsZero() && r.now().Sub(r.checked) < r.ttl {
		return r.ports
	}
	r.ports = r.query(ctx)
	r.checked = r.now()
	return r.ports
}

func (r *sandboxResolver) binary() string {
	if path, err := r.lookPath("sbx"); err == nil {
		return path
	}
	// Services started by launchd or brew get a short PATH.
	for _, candidate := range []string{"/opt/homebrew/bin/sbx", "/usr/local/bin/sbx"} {
		if r.stat(candidate) == nil {
			return candidate
		}
	}
	return ""
}

func (r *sandboxResolver) query(ctx context.Context) map[int]string {
	path := r.binary()
	if path == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	stdout := &limitedBuffer{remaining: r.maxOutput}
	stderr := &limitedBuffer{remaining: 32 << 10}
	if err := r.run(ctx, path, []string{"ls", "--json"}, stdout, stderr); err != nil {
		return nil
	}
	return parseSandboxPorts(stdout.Bytes())
}

func parseSandboxPorts(data []byte) map[int]string {
	var payload []struct {
		Name  string `json:"name"`
		Ports []struct {
			HostPort int `json:"host_port"`
		} `json:"ports"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		// Some sbx versions wrap the list in an object.
		var wrapped struct {
			Sandboxes json.RawMessage `json:"sandboxes"`
		}
		if json.Unmarshal(data, &wrapped) != nil || len(wrapped.Sandboxes) == 0 || json.Unmarshal(wrapped.Sandboxes, &payload) != nil {
			return nil
		}
	}
	ports := map[int]string{}
	for _, sandbox := range payload {
		name := strings.TrimSpace(sandbox.Name)
		if name == "" {
			continue
		}
		for _, port := range sandbox.Ports {
			if port.HostPort > 0 && port.HostPort <= 65535 {
				if _, taken := ports[port.HostPort]; !taken {
					ports[port.HostPort] = name
				}
			}
		}
	}
	return ports
}

// labelSandboxes names the sandbox behind sbx listeners. It costs nothing when
// no listener belongs to sbx.
func labelSandboxes(ctx context.Context, resolver *sandboxResolver, listeners []Listener) {
	if resolver == nil {
		return
	}
	needed := false
	for _, listener := range listeners {
		for _, process := range listener.Processes {
			needed = needed || isSandboxDaemon(process.Name)
		}
	}
	if !needed {
		return
	}
	ports := resolver.lookup(ctx)
	if len(ports) == 0 {
		return
	}
	for i := range listeners {
		name := ports[listeners[i].Port]
		if name == "" {
			continue
		}
		for j := range listeners[i].Processes {
			if isSandboxDaemon(listeners[i].Processes[j].Name) {
				listeners[i].Processes[j].Sandbox = name
			}
		}
	}
}
