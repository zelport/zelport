package sshpipe

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	mode := os.Getenv("ZELPORT_SSH_HELPER")
	if mode == "" {
		return
	}
	switch mode {
	case "echo":
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "fail":
		_, _ = io.WriteString(os.Stderr, "Permission denied (publickey).\n")
		os.Exit(255)
	case "refused":
		_, _ = io.WriteString(os.Stderr, "channel 0: open failed: connect failed: Connection refused\n")
		os.Exit(255)
	case "auth-orphan":
		child := exec.Command("/bin/sleep", "60")
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Permission denied (publickey).")
		os.Exit(255)
	case "jump-fail":
		fmt.Fprintln(os.Stderr, "channel 0: open failed: connect failed: Connection refused\nstdio forwarding failed\nkex_exchange_identification: Connection closed by remote host")
		os.Exit(255)
	case "orphan":
		child := exec.Command("/bin/sleep", "60")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, child.Process.Pid)
	case "hang":
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}
func helper(mode string) func(...string) *exec.Cmd {
	return func(args ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
		cmd.Env = append(os.Environ(), "ZELPORT_SSH_HELPER="+mode)
		return cmd
	}
}
func TestStdioConnectionAndClose(t *testing.T) {
	p := New("office", 3000)
	t.Cleanup(p.Close)
	p.command = helper("echo")
	defer p.Close()
	c, err := p.DialContext(context.Background(), "tcp", "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 5)
	if _, err = io.ReadFull(c, b); err != nil || string(b) != "hello" {
		t.Fatal(string(b), err)
	}
	if p.Status().State != "connected" {
		t.Fatal(p.Status())
	}
	if err = c.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Read(b); !os.IsTimeout(err) {
		t.Fatal("deadline", err)
	}
	c.SetReadDeadline(time.Time{})
	p.Close()
	if _, err = c.Write(b); err == nil {
		t.Fatal("write after close")
	}
	if _, err = p.DialContext(context.Background(), "tcp", ""); err == nil {
		t.Fatal("dial after close")
	}
}
func TestFailureDiagnosticsAndBackoff(t *testing.T) {
	for _, mode := range []string{"fail", "refused"} {
		p := New("office", 3000)
		t.Cleanup(p.Close)
		p.command = helper(mode)
		c, err := p.DialContext(context.Background(), "tcp", "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Read(make([]byte, 1))
		var want string
		if mode == "fail" {
			want = "Permission denied"
		} else {
			want = "Connection refused"
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatal(err)
		}
		status := p.Status()
		if status.State != "error" || status.Error == "" {
			t.Fatal(status)
		}
		if _, err = p.DialContext(context.Background(), "tcp", ""); err == nil {
			t.Fatal("no backoff")
		}
		c.Close()
		p.Close()
	}
}
func TestCanceledDial(t *testing.T) {
	p := New("office", 3000)
	t.Cleanup(p.Close)
	p.command = helper("hang")
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.DialContext(ctx, "tcp", ""); err == nil {
		t.Fatal("canceled dial succeeded")
	}
}

func TestPoolClosesFilesAfterProcessExit(t *testing.T) {
	p := New("office", 3000)
	t.Cleanup(p.Close)
	p.command = helper("fail")
	c, err := p.DialContext(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	<-c.(*conn).done
	p.Close()
	if _, err := c.(*conn).read.Stat(); !os.IsNotExist(err) && err == nil {
		t.Fatal("stdout left open")
	}
	if _, err := c.(*conn).write.Stat(); err == nil {
		t.Fatal("stdin left open")
	}
}

func TestInheritedConfigAppliesToJumpHosts(t *testing.T) {
	binary, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("SSH client unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err = os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	config := `Host office
 HostName 192.0.2.1
 User fixture
 Port 2222
 ProxyJump jump
Host jump
 HostName 192.0.2.2
 BatchMode no
 StrictHostKeyChecking no
 ControlMaster auto
 ControlPersist yes
`
	if err = os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := writeConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	for _, host := range []string{"office", "jump"} {
		out, err := exec.Command(binary, "-G", "-F", path, host).Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{"batchmode yes", "stricthostkeychecking true", "controlmaster false", "controlpersist no", "forkafterauthentication no"} {
			if !strings.Contains(string(out), expected+"\n") {
				t.Fatalf("%s missing %q", host, expected)
			}
		}
		if host == "office" && !strings.Contains(string(out), "user fixture\n") {
			t.Fatal("user config not inherited")
		}
	}
}

func TestExitedLeaderClosesDescendants(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("Unix process groups")
	}
	p := New("office", 3000)
	t.Cleanup(p.Close)
	p.command = helper("orphan")
	defer p.Close()
	c, err := p.DialContext(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if child, e := os.FindProcess(pid); e == nil {
			child.Kill()
		}
	}()
	select {
	case <-c.(*conn).done:
	case <-time.After(3 * time.Second):
		t.Fatal("leader was not reaped")
	}
	deadline := time.Now().Add(time.Second)
	for {
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
		if err != nil || strings.Contains(string(out), "Z") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant %d survived: %s", pid, out)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDelayedDiagnosticsAndJumpFailure(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("Unix process groups")
	}
	for _, mode := range []string{"auth-orphan", "jump-fail"} {
		p := New("office", 3000)
		t.Cleanup(p.Close)
		p.command = helper(mode)
		c, err := p.DialContext(context.Background(), "tcp", "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Read(make([]byte, 1))
		var failure *Error
		if !errors.As(err, &failure) || failure.Backend {
			t.Fatalf("%s: expected SSH failure, got %v", mode, err)
		}
		if p.Status().Error == "" {
			t.Fatal("missing failure/backoff")
		}
		p.Close()
	}
}
