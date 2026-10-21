// Package sbx is the boundary to the installed Docker Sandboxes CLI.
package sbx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Port struct {
	HostIP      string `json:"host_ip"`
	PublishedIP string `json:"published_ip,omitempty"`
	HostPort    int    `json:"host_port"`
	SandboxPort int    `json:"sandbox_port"`
	Protocol    string `json:"protocol"`
}

func (p Port) Binding() (netip.Addr, bool) {
	value := p.PublishedIP
	if value == "" {
		value = p.HostIP
	}
	ip, err := netip.ParseAddr(value)
	return ip, err == nil
}

func (p Port) Usable() bool {
	ip := net.ParseIP(p.HostIP)
	return ip != nil && ip.IsLoopback() && p.HostPort > 0 && p.HostPort <= 65535 && p.SandboxPort > 0 && p.SandboxPort <= 65535 && (p.Protocol == "tcp" || p.Protocol == "tcp4" || p.Protocol == "tcp6")
}
func (p Port) Address() string { return net.JoinHostPort(p.HostIP, strconv.Itoa(p.HostPort)) }
func (p Port) Spec() string {
	host := p.HostIP
	if binding, ok := p.Binding(); ok {
		host = binding.String()
	}
	return net.JoinHostPort(host, strconv.Itoa(p.HostPort)) + ":" + strconv.Itoa(p.SandboxPort) + "/" + p.Protocol
}

type Sandbox struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Ports  []Port `json:"ports"`
}
type Backend interface {
	List(context.Context) ([]Sandbox, error)
	Ports(context.Context, string) ([]Port, error)
	Publish(context.Context, string, int) error
	Start(context.Context, string, int) error
	Unpublish(context.Context, string, Port) error
}
type Client struct {
	LookPath func(string) (string, error)
	Run      func(context.Context, ...string) ([]byte, error)
	Timeout  time.Duration
}

func (c Client) call(ctx context.Context, args ...string) ([]byte, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if c.Run != nil {
		return c.Run(ctx, args...)
	}
	cmd := exec.CommandContext(ctx, "sbx", args...)
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var e *exec.ExitError
		if errors.As(err, &e) {
			return nil, fmt.Errorf("sbx %s: %s", args[0], strings.TrimSpace(string(e.Stderr)))
		}
		return nil, fmt.Errorf("sbx %s: %w", args[0], err)
	}
	return out, nil
}
func (c Client) List(ctx context.Context) ([]Sandbox, error) {
	b, err := c.call(ctx, "ls", "--json")
	if err != nil {
		return nil, err
	}
	// sbx has emitted both an envelope and a top-level array. Decode only
	// these known shapes; malformed output must not imply confirmed removal.
	var boxes []Sandbox
	payload := bytes.TrimSpace(b)
	if len(payload) > 0 && payload[0] == '[' {
		err = json.Unmarshal(payload, &boxes)
	} else {
		var envelope struct {
			Sandboxes *[]Sandbox `json:"sandboxes"`
		}
		err = json.Unmarshal(payload, &envelope)
		if err == nil {
			if envelope.Sandboxes == nil {
				return nil, errors.New("sbx list JSON is missing sandboxes")
			}
			boxes = *envelope.Sandboxes
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid sbx list JSON: %w", err)
	}
	seen := map[string]bool{}
	for _, s := range boxes {
		if s.ID == "" || s.Name == "" || s.Status == "" || seen[s.ID] {
			return nil, errors.New("sbx list contains invalid or duplicate sandbox identity")
		}
		seen[s.ID] = true
	}
	return boxes, nil
}
func (c Client) Ports(ctx context.Context, id string) ([]Port, error) {
	name, err := c.resolve(ctx, id)
	if err != nil {
		return nil, err
	}
	b, err := c.call(ctx, "ports", name, "--json")
	if err != nil {
		return nil, err
	}
	var ports []Port
	if err = json.Unmarshal(b, &ports); err != nil {
		return nil, fmt.Errorf("invalid sbx ports JSON: %w", err)
	}
	for i := range ports {
		if ports[i].PublishedIP == "" {
			ports[i].PublishedIP = ports[i].HostIP
			if ports[i].PublishedIP == "" {
				ports[i].PublishedIP = "0.0.0.0"
			}
		}
		switch ports[i].HostIP {
		case "", "0.0.0.0":
			ports[i].HostIP = "127.0.0.1"
		case "::":
			ports[i].HostIP = "::1"
		}
	}
	return ports, nil
}
func (c Client) Publish(ctx context.Context, id string, port int) error {
	return c.publish(ctx, id, port, false)
}
func (c Client) Start(ctx context.Context, id string, port int) error {
	return c.publish(ctx, id, port, true)
}
func (c Client) publish(ctx context.Context, id string, port int, allowStart bool) error {
	boxes, err := c.List(ctx)
	if err != nil {
		return err
	}
	for _, box := range boxes {
		if box.ID == id {
			if !allowStart && box.Status != "running" {
				return errors.New("The sandbox stopped before the port was published. Start it explicitly.")
			}
			if strings.HasPrefix(box.Name, "-") {
				return errors.New("invalid sandbox name")
			}
			_, err = c.call(ctx, "ports", box.Name, "--publish", strconv.Itoa(port))
			return err
		}
	}
	return errors.New("sandbox identity no longer exists")
}
func (c Client) Unpublish(ctx context.Context, id string, port Port) error {
	name, err := c.resolve(ctx, id)
	if err != nil {
		return err
	}
	_, err = c.call(ctx, "ports", name, "--unpublish", port.Spec())
	return err
}

// Local sbx ports accepts names, not UUIDs. Resolve immediately before each
// operation; never fall back to a saved name when its identity has disappeared.
func (c Client) resolve(ctx context.Context, id string) (string, error) {
	boxes, err := c.List(ctx)
	if err != nil {
		return "", err
	}
	for _, s := range boxes {
		if s.ID == id {
			if strings.HasPrefix(s.Name, "-") {
				return "", errors.New("invalid sandbox name")
			}
			return s.Name, nil
		}
	}
	return "", errors.New("sandbox identity no longer exists")
}
