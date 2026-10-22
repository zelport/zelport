package sshpipe

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type conn struct {
	pool        *Pool
	cmd         *exec.Cmd
	read, write *os.File
	diagnostic  diagnosticBuffer
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
