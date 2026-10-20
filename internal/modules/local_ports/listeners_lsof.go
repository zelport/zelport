package local_ports

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var ErrListenerDiscovery = errors.New("listener discovery unavailable")

var errListenerOutputLimit = errors.New("listener discovery output limit exceeded")

type lsofCommand func(context.Context, string, []string, io.Writer, io.Writer) error

type lsofScanner struct {
	stat      func(string) error
	lookPath  func(string) (string, error)
	run       lsofCommand
	maxOutput int64
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	remaining int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if int64(len(p)) > b.remaining {
		return 0, errListenerOutputLimit
	}
	n, err := b.buffer.Write(p)
	b.remaining -= int64(n)
	return n, err
}

func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func discoveryFailure(error) error {
	return fmt.Errorf("%w", ErrListenerDiscovery)
}

func newLsofScanner() listenerScanner {
	return lsofScanner{
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
		maxOutput: 1 << 20,
	}
}

func (s lsofScanner) Scan(ctx context.Context) ([]rawListener, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := ""
	for _, candidate := range []string{"/usr/sbin/lsof", "/usr/bin/lsof"} {
		if s.stat(candidate) == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		var err error
		path, err = s.lookPath("lsof")
		if err != nil {
			return nil, discoveryFailure(err)
		}
	}
	limit := s.maxOutput
	if limit < 1 {
		limit = 1 << 20
	}
	stdout := &limitedBuffer{remaining: limit}
	stderr := &limitedBuffer{remaining: 32 << 10}
	if err := s.run(ctx, path, []string{"-nP", "-iTCP", "-sTCP:LISTEN", "-F0pcn"}, stdout, stderr); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, discoveryFailure(err)
	}
	listeners, err := parseLsofListeners(bytes.NewReader(stdout.Bytes()))
	if err != nil {
		return nil, discoveryFailure(err)
	}
	return listeners, nil
}

func parseLsofListeners(r io.Reader) ([]rawListener, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitNUL)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var listeners []rawListener
	var pid int
	var process string
	for scanner.Scan() {
		field := strings.TrimLeft(scanner.Text(), "\r\n")
		if len(field) < 2 {
			continue
		}
		switch field[0] {
		case 'p':
			value, err := strconv.Atoi(field[1:])
			if err != nil || value < 1 {
				pid = 0
				process = ""
				continue
			}
			pid = value
			process = ""
		case 'c':
			process = field[1:]
		case 'n':
			address, port, ok := parseLsofEndpoint(field[1:])
			if ok {
				listeners = append(listeners, rawListener{Address: address, Port: port, PID: pid, Process: process})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return listeners, nil
}

func splitNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func parseLsofEndpoint(value string) (netip.Addr, int, bool) {
	value = strings.TrimSuffix(value, " (LISTEN)")
	if strings.HasPrefix(value, "*:") {
		port, err := strconv.Atoi(strings.TrimPrefix(value, "*:"))
		return netip.IPv4Unspecified(), port, err == nil && port > 0 && port <= 65535
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return netip.Addr{}, 0, false
	}
	return address, port, true
}
