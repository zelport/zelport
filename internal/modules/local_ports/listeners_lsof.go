package local_ports

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

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
