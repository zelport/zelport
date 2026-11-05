package local_ports

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Some listeners are one proxy process for many workloads: the sbx daemon for
// Docker Sandboxes, gvproxy for Podman, com.docker.backend for Docker Desktop,
// docker-proxy on Linux. The workload behind a port comes from one listing call
// of that runtime, made only when such a process listens and cached briefly.

type workloadKind int

const (
	sandboxWorkload workloadKind = iota
	containerWorkload
)

type workloadSource struct {
	kind      workloadKind
	processes []string // exact names, case-insensitive
	prefixes  []string
	command   string
	paths     []string // fallbacks when PATH is short (launchd, brew services)
	args      []string
	parse     func([]byte) map[int]string

	ports   map[int]string
	checked time.Time
}

func (s *workloadSource) owns(process string) bool {
	for _, name := range s.processes {
		if strings.EqualFold(process, name) {
			return true
		}
	}
	lower := strings.ToLower(process)
	for _, prefix := range s.prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

type workloadResolver struct {
	stat      func(string) error
	lookPath  func(string) (string, error)
	run       lsofCommand
	now       func() time.Time
	ttl       time.Duration
	timeout   time.Duration
	maxOutput int64

	mu      sync.Mutex
	sources []*workloadSource
}

func defaultWorkloadSources() []*workloadSource {
	home, _ := os.UserHomeDir()
	inHome := func(path string) string {
		if home == "" {
			return ""
		}
		return filepath.Join(home, path)
	}
	containerFormat := []string{"ps", "--format", "{{.Names}}\t{{.Ports}}"}
	return []*workloadSource{
		{kind: sandboxWorkload, processes: []string{"sbx"}, command: "sbx", paths: []string{"/opt/homebrew/bin/sbx", "/usr/local/bin/sbx"}, args: []string{"ls", "--json"}, parse: parseSandboxPorts},
		{kind: containerWorkload, processes: []string{"gvproxy", "rootlessport", "pasta", "slirp4netns"}, command: "podman", paths: []string{"/opt/podman/bin/podman", "/opt/homebrew/bin/podman", "/usr/local/bin/podman", "/usr/bin/podman"}, args: containerFormat, parse: parseContainerPorts},
		{kind: containerWorkload, processes: []string{"docker-proxy", "vpnkit-bridge", "OrbStack Helper", "limactl"}, prefixes: []string{"com.docker."}, command: "docker", paths: []string{"/usr/local/bin/docker", "/opt/homebrew/bin/docker", "/Applications/Docker.app/Contents/Resources/bin/docker", inHome(".orbstack/bin/docker"), inHome(".docker/bin/docker"), "/usr/bin/docker"}, args: containerFormat, parse: parseContainerPorts},
	}
}

func newWorkloadResolver() *workloadResolver {
	return &workloadResolver{
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
		timeout:   1500 * time.Millisecond,
		maxOutput: 1 << 20,
		sources:   defaultWorkloadSources(),
	}
}

// Invalidate makes the next lookup ask the runtimes again (Rescan ports).
func (r *workloadResolver) Invalidate() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, source := range r.sources {
		source.checked = time.Time{}
	}
}

func (r *workloadResolver) binary(source *workloadSource) string {
	if path, err := r.lookPath(source.command); err == nil {
		return path
	}
	for _, candidate := range source.paths {
		if candidate != "" && r.stat(candidate) == nil {
			return candidate
		}
	}
	return ""
}

func (r *workloadResolver) query(ctx context.Context, source *workloadSource) map[int]string {
	path := r.binary(source)
	if path == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	stdout := &limitedBuffer{remaining: r.maxOutput}
	stderr := &limitedBuffer{remaining: 32 << 10}
	if err := r.run(ctx, path, source.args, stdout, stderr); err != nil {
		return nil
	}
	return source.parse(stdout.Bytes())
}

// label names the workload behind proxy listeners. It runs nothing when no
// listener belongs to a known runtime, and asks the runtimes it needs in parallel.
func (r *workloadResolver) label(ctx context.Context, listeners []Listener) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	needed := map[*workloadSource]bool{}
	for _, listener := range listeners {
		for _, process := range listener.Processes {
			for _, source := range r.sources {
				if source.owns(process.Name) {
					needed[source] = true
				}
			}
		}
	}
	var wait sync.WaitGroup
	now := r.now()
	for source := range needed {
		if !source.checked.IsZero() && now.Sub(source.checked) < r.ttl {
			continue
		}
		wait.Add(1)
		go func(source *workloadSource) {
			defer wait.Done()
			source.ports = r.query(ctx, source)
		}(source)
		source.checked = now
	}
	wait.Wait()
	for i := range listeners {
		for j := range listeners[i].Processes {
			process := &listeners[i].Processes[j]
			for source := range needed {
				name := source.ports[listeners[i].Port]
				if name == "" || !source.owns(process.Name) {
					continue
				}
				if source.kind == sandboxWorkload {
					process.Sandbox = name
				} else {
					process.Container = name
				}
				break
			}
		}
	}
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

// parseContainerPorts reads `docker|podman ps --format '{{.Names}}\t{{.Ports}}'`:
//
//	web	127.0.0.1:8080->80/tcp, [::]:8080->80/tcp, 0.0.0.0:9000-9001->9000-9001/tcp, 443/tcp
func parseContainerPorts(data []byte) map[int]string {
	ports := map[int]string{}
	for _, line := range strings.Split(string(data), "\n") {
		name, list, ok := strings.Cut(strings.TrimRight(line, "\r"), "\t")
		name = strings.TrimSpace(strings.Split(name, ",")[0])
		if !ok || name == "" {
			continue
		}
		for _, item := range strings.Split(list, ",") {
			host, target, ok := strings.Cut(strings.TrimSpace(item), "->")
			if !ok || !strings.HasSuffix(target, "/tcp") {
				continue
			}
			colon := strings.LastIndex(host, ":")
			if colon < 0 {
				continue
			}
			first, last, err := portSpan(host[colon+1:])
			if err != nil {
				continue
			}
			for port := first; port <= last; port++ {
				if _, taken := ports[port]; !taken {
					ports[port] = name
				}
			}
		}
	}
	return ports
}

func portSpan(text string) (int, int, error) {
	low, high, isRange := strings.Cut(text, "-")
	first, err := strconv.Atoi(low)
	if err != nil {
		return 0, 0, err
	}
	last := first
	if isRange {
		if last, err = strconv.Atoi(high); err != nil {
			return 0, 0, err
		}
	}
	if first < 1 || last > 65535 || last < first || last-first > 4096 {
		return 0, 0, strconv.ErrRange
	}
	return first, last, nil
}
