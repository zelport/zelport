package containers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	Engine     string
	LookPath   func(string) (string, error)
	Platform   string
	executable string
	Run        func(context.Context, string, ...string) ([]byte, error)
	Env        func(string) string
	Timeout    time.Duration
}

func (c Client) env(key string) string {
	if c.Env != nil {
		return c.Env(key)
	}
	return os.Getenv(key)
}
func (c Client) call(ctx context.Context, args ...string) ([]byte, error) {
	if c.Run != nil {
		return c.Run(ctx, c.Engine, args...)
	}
	executable := c.executable
	if executable == "" {
		executable = c.Engine
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var e *exec.ExitError
		if errors.As(err, &e) {
			return nil, fmt.Errorf("%s: %s", c.Engine, strings.TrimSpace(string(e.Stderr)))
		}
		return nil, err
	}
	return out, nil
}
func fingerprint(engine, label, endpoint string) string {
	sum := sha256.Sum256([]byte(engine + "\x00" + label + "\x00" + endpoint))
	return hex.EncodeToString(sum[:])
}
func (c Client) Scan(parent context.Context) (inv Inventory) {
	inv.Engine = c.Engine
	inv.Containers = []Container{}
	inv.CLIStatus = "not_found"
	inv.RuntimeStatus = "not_checked"
	lookup := c.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	path, lookupErr := lookup(c.Engine)
	if lookupErr != nil {
		inv.Stale = true
		inv.Error = lookupErr.Error()
		return
	}
	inv.CLIStatus = "found"
	inv.CLIPath = path
	c.executable = path
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	var err error
	if c.Engine == "docker" {
		err = c.docker(ctx, &inv)
	} else if c.Engine == "podman" {
		err = c.podman(ctx, &inv)
	} else if c.Engine == "nerdctl" {
		err = c.nerdctl(ctx, &inv)
	} else {
		err = errors.New("unsupported container engine")
	}
	if err != nil {
		inv.Stale = true
		inv.Containers = []Container{}
		inv.Error = err.Error()
	} else {
		inv.UpdatedAt = time.Now()
	}
	return
}
func (c Client) docker(ctx context.Context, inv *Inventory) error {
	endpoint := c.env("DOCKER_HOST")
	label := c.env("DOCKER_CONTEXT")
	if label != "" || endpoint == "" {
		if label == "" {
			b, err := c.call(ctx, "context", "show")
			if err != nil {
				return err
			}
			label = strings.TrimSpace(string(b))
		}
		if label == "" || strings.HasPrefix(label, "-") {
			return errors.New("invalid Docker context")
		}
		b, err := c.call(ctx, "context", "inspect", label)
		if err != nil {
			return err
		}
		var contexts []struct {
			Endpoints map[string]struct{ Host string }
		}
		if err = json.Unmarshal(b, &contexts); err != nil || len(contexts) != 1 {
			return errors.New("invalid Docker context JSON")
		}
		endpoint = contexts[0].Endpoints["docker"].Host
	} else {
		label = "DOCKER_HOST"
	}
	inv.Label = label
	inv.Connection = fingerprint(c.Engine, label, endpoint)
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "unix" || u.Host != "" || u.Path == "" {
		return errors.New("only local Unix-socket Docker connections are supported")
	}
	// Pin the endpoint for every call; never re-read the mutable default context.
	inv.Endpoint = canonicalSocket(u.Path)
	prefix := []string{"--host", endpoint}
	if err = c.probe(ctx, prefix, inv); err != nil {
		return err
	}
	return c.inspectContainers(ctx, prefix, inv)
}
func (c Client) inspectContainers(ctx context.Context, prefix []string, inv *Inventory) error {
	b, err := c.call(ctx, append(prefix, "ps", "--all", "--no-trunc", "--quiet")...)
	if err != nil {
		return err
	}
	ids := strings.Fields(string(b))
	for _, id := range ids {
		if !validID.MatchString(id) {
			return errors.New("invalid Docker container ID")
		}
	}
	for start := 0; start < len(ids); start += 100 {
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		args := append(append([]string{}, prefix...), "container", "inspect", "--format", "{{json .}}")
		if c.Engine == "nerdctl" {
			args = append(append([]string{}, prefix...), "inspect", "--mode", "dockercompat", "--format", "{{json .}}")
		}
		b, err = c.call(ctx, append(args, ids[start:end]...)...)
		if err != nil {
			return err
		}
		boxes, err := parseDocker(b)
		if err != nil {
			return err
		}
		if len(boxes) != end-start {
			return errors.New("incomplete Docker inspect result")
		}
		for i, box := range boxes {
			if box.ID != ids[start+i] {
				return errors.New("Docker inspect identity mismatch")
			}
		}
		inv.Containers = append(inv.Containers, boxes...)
	}
	return nil
}
func (c Client) podman(ctx context.Context, inv *Inventory) error {
	// A URI override can refer to a remote host whose published ports are not local.
	if c.env("CONTAINER_HOST") != "" {
		inv.Connection = fingerprint(c.Engine, "CONTAINER_HOST", c.env("CONTAINER_HOST"))
		return errors.New("CONTAINER_HOST overrides are not supported; use a local Podman connection")
	}
	b, err := c.call(ctx, "system", "connection", "list", "--format", "json")
	if err != nil {
		return err
	}
	var connections []struct {
		Name, URI, Identity string
		Default, IsMachine  bool
	}
	if err = json.Unmarshal(b, &connections); err != nil {
		return fmt.Errorf("invalid Podman connection JSON: %w", err)
	}
	var name, endpoint, identity string
	var machine bool
	selected := c.env("CONTAINER_CONNECTION")
	for _, connection := range connections {
		if (selected != "" && connection.Name == selected) || (selected == "" && connection.Default) {
			name = connection.Name
			endpoint = connection.URI
			identity = connection.Identity
			machine = connection.IsMachine
			break
		}
	}
	prefix := []string{}
	if name == "" {
		if selected != "" || len(connections) > 0 {
			return errors.New("Podman connection not found")
		}
		// Native Linux Podman, explicitly disabling remote mode.
		name = "local"
		endpoint = "native"
		prefix = []string{"--remote=false"}
	} else {
		if strings.HasPrefix(name, "-") {
			return errors.New("invalid Podman connection name")
		}
		u, e := url.Parse(endpoint)
		local := e == nil && u.Scheme == "unix" && u.Host == "" && u.Path != ""
		if e == nil && machine && u.Scheme == "ssh" {
			ip := net.ParseIP(u.Hostname())
			local = ip != nil && ip.IsLoopback()
		}
		inv.Label = name
		inv.Connection = fingerprint(c.Engine, name, endpoint)
		if !local {
			return errors.New("only local Podman sockets and local Podman machines are supported")
		}
		// URI pins the destination if the named connection changes during discovery.
		prefix = []string{"--url", endpoint}
		if identity != "" {
			prefix = append(prefix, "--identity", identity)
		}
	}
	inv.Label = name
	inv.Connection = fingerprint(c.Engine, name, endpoint)
	if u, e := url.Parse(endpoint); e == nil && u.Scheme == "unix" {
		inv.Endpoint = canonicalSocket(u.Path)
	}
	if err = c.probe(ctx, prefix, inv); err != nil {
		return err
	}
	b, err = c.call(ctx, append(prefix, "ps", "--all", "--no-trunc", "--format", "json")...)
	if err != nil {
		return err
	}
	boxes, err := parsePodman(b)
	if err != nil {
		return err
	}
	// Verify the named connection did not change while listing containers.
	if name != "local" {
		after, err := c.call(ctx, "system", "connection", "list", "--format", "json")
		if err != nil {
			return err
		}
		var check []struct{ Name, URI string }
		if err = json.Unmarshal(after, &check); err != nil {
			return err
		}
		found := false
		for _, v := range check {
			if v.Name == name && v.URI == endpoint {
				found = true
			}
		}
		if !found {
			return errors.New("Podman connection changed during discovery")
		}
	}
	inv.Containers = boxes
	return nil
}

var validID = regexp.MustCompile(`^[a-f0-9]{64}$`)

func loopback(ip string) string {
	switch ip {
	case "", "0.0.0.0":
		return "127.0.0.1"
	case "::":
		return "::1"
	}
	parsed := net.ParseIP(ip)
	if parsed != nil && parsed.IsLoopback() {
		return parsed.String()
	}
	return ""
}
func cleanPorts(ports []Port) []Port {
	seen := map[Port]bool{}
	out := []Port{}
	for _, p := range ports {
		p.HostIP = loopback(p.HostIP)
		if p.Usable() && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ContainerPort != b.ContainerPort {
			return a.ContainerPort < b.ContainerPort
		}
		if a.HostPort != b.HostPort {
			return a.HostPort < b.HostPort
		}
		return a.HostIP < b.HostIP
	})
	return out
}
func parseDocker(data []byte) ([]Container, error) {
	boxes := []Container{}
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var row struct {
			ID, Name        string
			State           struct{ Status string }
			NetworkSettings struct {
				Ports map[string][]struct{ HostIP, HostPort string }
			}
			HostConfig struct {
				PortBindings map[string][]struct{ HostIP, HostPort string }
			}
		}
		err := dec.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid Docker inspect JSON: %w", err)
		}
		if !validID.MatchString(row.ID) || row.Name == "" || row.State.Status == "" {
			return nil, errors.New("incomplete Docker container")
		}
		box := Container{ID: row.ID, Name: strings.TrimPrefix(row.Name, "/"), Status: row.State.Status}
		ports := row.NetworkSettings.Ports
		if row.State.Status != "running" {
			ports = row.HostConfig.PortBindings
		}
		for key, bindings := range ports {
			port, proto, ok := strings.Cut(key, "/")
			if !ok {
				continue
			}
			inside, _ := strconv.Atoi(port)
			for _, binding := range bindings {
				outside, _ := strconv.Atoi(binding.HostPort)
				box.Ports = append(box.Ports, Port{binding.HostIP, outside, inside, proto})
			}
		}
		box.Ports = cleanPorts(box.Ports)
		boxes = append(boxes, box)
	}
	return boxes, nil
}
func parsePodman(data []byte) ([]Container, error) {
	var rows []struct {
		ID    string
		Names []string
		State string
		Ports []struct {
			HostIP        string `json:"host_ip"`
			HostPort      int    `json:"host_port"`
			ContainerPort int    `json:"container_port"`
			Range         int    `json:"range"`
			Protocol      string `json:"protocol"`
		}
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("invalid Podman ps JSON: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, errors.New("invalid null Podman inventory")
	}
	boxes := []Container{}
	seen := map[string]bool{}
	for _, row := range rows {
		if !validID.MatchString(row.ID) || len(row.Names) == 0 || row.Names[0] == "" || row.State == "" || seen[row.ID] {
			return nil, errors.New("incomplete or duplicate Podman container")
		}
		seen[row.ID] = true
		box := Container{ID: row.ID, Name: row.Names[0], Status: row.State}
		for _, p := range row.Ports {
			count := p.Range
			if count == 0 {
				count = 1
			}
			if count < 1 || count > 65535 || p.HostPort < 1 || p.HostPort+count-1 > 65535 || p.ContainerPort < 1 || p.ContainerPort+count-1 > 65535 {
				continue
			}
			for n := 0; n < count; n++ {
				box.Ports = append(box.Ports, Port{p.HostIP, p.HostPort + n, p.ContainerPort + n, p.Protocol})
			}
		}
		box.Ports = cleanPorts(box.Ports)
		boxes = append(boxes, box)
	}
	return boxes, nil
}

// A successful info response separates runtime reachability from inventory parsing.
func (c Client) probe(ctx context.Context, prefix []string, inv *Inventory) error {
	inv.RuntimeStatus = "unavailable"
	data, err := c.call(ctx, append(append([]string{}, prefix...), "info", "--format", "{{json .}}")...)
	if err != nil {
		return err
	}
	var info struct {
		ServerVersion string
		Host          *struct{ RemoteSocket *struct{ Path string } }
	}
	if err = json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("invalid %s runtime info JSON: %w", c.Engine, err)
	}
	if (c.Engine == "docker" && info.ServerVersion == "") || (c.Engine == "podman" && info.Host == nil) {
		return fmt.Errorf("incomplete %s runtime info", c.Engine)
	}
	inv.RuntimeStatus = "available"
	if c.Engine == "podman" && inv.Label == "local" && info.Host.RemoteSocket != nil {
		inv.Endpoint = canonicalSocket(info.Host.RemoteSocket.Path)
	}
	return nil
}
func canonicalSocket(path string) string {
	if path == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return "unix://" + filepath.Clean(path)
}
func (c Client) platform() string {
	if c.Platform != "" {
		return c.Platform
	}
	return runtime.GOOS
}
