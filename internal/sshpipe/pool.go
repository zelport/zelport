// Package sshpipe adapts OpenSSH stdio forwarding to net.Conn without a listener.
package sshpipe

import (
	"context"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Error struct {
	Message string
	Backend bool
}

func (e *Error) Error() string { return e.Message }

type Status struct {
	State       string `json:"state"`
	Error       string `json:"error,omitempty"`
	Connections int    `json:"connections"`
}
type Pool struct {
	mu         sync.Mutex
	host       string
	port       int
	command    func(...string) *exec.Cmd
	conns      map[*conn]bool
	closed     bool
	configPath string
	failure    *Error
	failures   int
	retry      time.Time
}

func New(host string, port int) *Pool {
	return &Pool{host: host, port: port, conns: make(map[*conn]bool), command: func(args ...string) *exec.Cmd { return exec.Command("ssh", args...) }}
}
func (p *Pool) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := Status{State: "idle"}
	for c, ready := range p.conns {
		select {
		case <-c.done:
			continue
		default:
		}
		s.Connections++
		if ready {
			s.State = "connected"
		} else if s.State == "idle" {
			s.State = "connecting"
		}
	}
	if p.failure != nil {
		s.Error = p.failure.Error()
		if s.State != "connected" {
			s.State = "error"
		}
	}
	return s
}
func (p *Pool) recordFailure(err *Error) {
	p.failure = err
	p.failures++
	delay := time.Second << min(p.failures-1, 5)
	p.retry = time.Now().Add(min(delay, 30*time.Second))
}
func (p *Pool) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, net.ErrClosed
	}
	if time.Now().Before(p.retry) {
		return nil, p.failure
	}
	inRead, inWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		inRead.Close()
		inWrite.Close()
		return nil, err
	}
	if p.configPath == "" {
		p.configPath, err = writeConfig()
		if err != nil {
			inRead.Close()
			inWrite.Close()
			outRead.Close()
			outWrite.Close()
			return nil, &Error{Message: "Cannot prepare SSH configuration: " + err.Error()}
		}
	}
	args := append([]string{"-F", p.configPath}, Arguments(p.host, p.port)...)
	cmd := p.command(args...)
	// Preserve SSH_AUTH_SOCK and normal host configuration; never invoke askpass.
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "SSH_ASKPASS_REQUIRE=never", "LC_ALL=C")
	cmd.Stdin = inRead
	cmd.Stdout = outWrite
	c := &conn{pool: p, cmd: cmd, read: outRead, write: inWrite, done: make(chan struct{})}
	cmd.Stderr = &c.diagnostic
	cmd.WaitDelay = time.Second
	prepareProcess(cmd)
	err = cmd.Start()
	inRead.Close()
	outWrite.Close()
	if err != nil {
		outRead.Close()
		inWrite.Close()
		failure := &Error{Message: "Cannot start SSH client: " + err.Error()}
		p.recordFailure(failure)
		return nil, failure
	}
	p.conns[c] = false
	go c.wait()
	if err = ctx.Err(); err != nil {
		// Close outside the pool lock.
		go c.Close()
		return nil, err
	}
	return c, nil
}
func (p *Pool) Close() {
	p.mu.Lock()
	p.closed = true
	conns := make([]*conn, 0, len(p.conns))
	for c := range p.conns {
		conns = append(conns, c)
	}
	p.mu.Unlock()
	for _, c := range conns {
		c.Close()
	}
	if p.configPath != "" {
		_ = os.Remove(p.configPath)
	}
}
