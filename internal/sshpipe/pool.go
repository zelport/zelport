// Package sshpipe adapts OpenSSH stdio forwarding to net.Conn without a listener.
package sshpipe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	diagclean "github.com/zelport/zelport/internal/diagnostic"
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
func Arguments(host string, port int) []string {
	args := []string{"-T", "-a", "-x"}
	for _, option := range []string{
		"BatchMode=yes", "StrictHostKeyChecking=yes", "UpdateHostKeys=no",
		"ConnectTimeout=10", "ConnectionAttempts=1", "ServerAliveInterval=15", "ServerAliveCountMax=2",
		"ExitOnForwardFailure=yes", "ClearAllForwardings=yes", "PermitLocalCommand=no",
		"ForkAfterAuthentication=no", "StdinNull=no", "RemoteCommand=none",
		"ControlMaster=no", "ControlPath=none", "ControlPersist=no", "LogLevel=ERROR",
	} {
		args = append(args, "-o", option)
	}
	return append(args, "-W", "127.0.0.1:"+strconv.Itoa(port), "--", host)
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

type diagnostic struct {
	mu sync.Mutex
	b  []byte
}

func (d *diagnostic) Write(b []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(b)
	d.b = append(d.b, b...)
	if len(d.b) > 4096 {
		d.b = append([]byte(nil), d.b[len(d.b)-4096:]...)
	}
	return n, nil
}
func (d *diagnostic) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return diagclean.Clean(string(d.b))
}

type conn struct {
	pool        *Pool
	cmd         *exec.Cmd
	read, write *os.File
	diagnostic  diagnostic
	done        chan struct{}
	closeOnce   sync.Once
	stopOnce    sync.Once
	mu          sync.Mutex
	closed      bool
	exitErr     *Error
}

func (c *conn) wait() {
	err := c.cmd.Wait()
	c.stopOnce.Do(func() { stopProcess(c.cmd) })
	c.mu.Lock()
	if err != nil && !c.closed {
		message := c.diagnostic.String()
		if message == "" {
			message = err.Error()
		}
		c.exitErr = &Error{Message: "SSH: " + message, Backend: backendFailure(message)}
	}
	failure := c.exitErr
	c.mu.Unlock()
	c.pool.mu.Lock()
	if failure != nil && !c.pool.closed {
		c.pool.recordFailure(failure)
	}
	c.pool.mu.Unlock()
	close(c.done)
}
func (c *conn) ioError(err error) error {
	if err == nil || os.IsTimeout(err) {
		return err
	}
	// stdout can close just before Wait finishes draining the diagnostic pipe.
	select {
	case <-c.done:
	case <-time.After(1500 * time.Millisecond):
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.exitErr != nil {
		return c.exitErr
	}
	return err
}
func (c *conn) Read(b []byte) (int, error) {
	n, err := c.read.Read(b)
	if n > 0 {
		c.pool.mu.Lock()
		if _, ok := c.pool.conns[c]; ok {
			c.pool.conns[c] = true
		}
		c.pool.failure = nil
		c.pool.failures = 0
		c.pool.retry = time.Time{}
		c.pool.mu.Unlock()
	}
	return n, c.ioError(err)
}
func (c *conn) Write(b []byte) (int, error) { n, err := c.write.Write(b); return n, c.ioError(err) }
func (c *conn) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		c.read.Close()
		c.write.Close()
		// Kill the process group owned by this connection, including ProxyJump helpers.
		select {
		case <-c.done:
		default:
			c.stopOnce.Do(func() { stopProcess(c.cmd) })
		}
		<-c.done
		c.pool.mu.Lock()
		delete(c.pool.conns, c)
		c.pool.mu.Unlock()
	})
	return nil
}

type address string

func (a address) Network() string   { return "ssh" }
func (a address) String() string    { return string(a) }
func (c *conn) LocalAddr() net.Addr { return address("stdio") }
func (c *conn) RemoteAddr() net.Addr {
	return address(fmt.Sprintf("%s/127.0.0.1:%d", c.pool.host, c.pool.port))
}
func (c *conn) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}
func (c *conn) SetReadDeadline(t time.Time) error  { return c.read.SetReadDeadline(t) }
func (c *conn) SetWriteDeadline(t time.Time) error { return c.write.SetWriteDeadline(t) }

var _ net.Conn = (*conn)(nil)
var _ io.Writer = (*diagnostic)(nil)

// The same -F file is inherited by OpenSSH's implicit ProxyJump children.
// Only references are stored; user keys and configuration are never copied.
func writeConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(home, "\r\n") {
		return "", errors.New("invalid home directory")
	}
	quote := func(s string) string { return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(s) + "\"" }
	f, err := os.CreateTemp("", "zelport-ssh-*.conf")
	if err != nil {
		return "", err
	}
	content := "Host *\n"
	args := Arguments("unused", 1)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-o" {
			content += "  " + strings.Replace(args[i+1], "=", " ", 1) + "\n"
			i++
		}
	}
	content += "  RequestTTY no\n  ForwardAgent no\n  ForwardX11 no\n"
	content += "Include " + quote(filepath.Join(home, ".ssh", "config")) + "\nHost *\nInclude /etc/ssh/ssh_config\n"
	_, err = f.WriteString(content)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// A jump host can report a forwarding failure to the destination SSH port.
// A subsequent key-exchange failure belongs to SSH connectivity, not HTTP.
func backendFailure(message string) bool {
	return strings.Contains(message, "open failed: connect failed:") &&
		!strings.Contains(message, "kex_exchange_identification") &&
		!strings.Contains(message, "ssh_exchange_identification") &&
		!strings.Contains(message, "Connection closed by UNKNOWN") &&
		!strings.Contains(message, "banner exchange")
}
