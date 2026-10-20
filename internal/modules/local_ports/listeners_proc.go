package local_ports

import (
	"bufio"
	"encoding/hex"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

type procListener struct {
	Address netip.Addr
	Port    int
	Inode   string
}

func parseProcListeners(r io.Reader, ipv6 bool) ([]procListener, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var listeners []procListener
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || fields[3] != "0A" {
			continue
		}
		addressHex, portHex, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		address, ok := parseProcAddress(addressHex, ipv6)
		if !ok {
			continue
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil || port == 0 {
			continue
		}
		listeners = append(listeners, procListener{Address: address, Port: int(port), Inode: fields[9]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return listeners, nil
}

func parseProcAddress(value string, ipv6 bool) (netip.Addr, bool) {
	want := 4
	if ipv6 {
		want = 16
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != want {
		return netip.Addr{}, false
	}
	if ipv6 {
		for start := 0; start < len(decoded); start += 4 {
			decoded[start], decoded[start+3] = decoded[start+3], decoded[start]
			decoded[start+1], decoded[start+2] = decoded[start+2], decoded[start+1]
		}
		var bytes [16]byte
		copy(bytes[:], decoded)
		return netip.AddrFrom16(bytes), true
	}
	var bytes [4]byte
	for i := range decoded {
		bytes[i] = decoded[len(decoded)-1-i]
	}
	return netip.AddrFrom4(bytes), true
}
