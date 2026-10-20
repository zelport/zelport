//go:build linux

package local_ports

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type procScanner struct {
	root       string
	maxRecords int
}

func newListenerScanner() listenerScanner {
	return procScanner{root: "/proc", maxRecords: 65536}
}

func (s procScanner) Scan(ctx context.Context) ([]rawListener, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maxRecords := s.maxRecords
	if maxRecords < 1 {
		maxRecords = 65536
	}
	var sockets []procListener
	tablesRead := 0
	for _, table := range []struct {
		name string
		ipv6 bool
	}{{"tcp", false}, {"tcp6", true}} {
		file, err := os.Open(filepath.Join(s.root, "net", table.name))
		if err != nil {
			continue
		}
		parsed, parseErr := parseProcListeners(file, table.ipv6)
		closeErr := file.Close()
		if parseErr != nil {
			return nil, discoveryFailure(parseErr)
		}
		if closeErr != nil {
			return nil, discoveryFailure(closeErr)
		}
		tablesRead++
		if len(sockets)+len(parsed) > maxRecords {
			return nil, discoveryFailure(fmt.Errorf("listener record limit exceeded"))
		}
		sockets = append(sockets, parsed...)
	}
	if tablesRead == 0 {
		return nil, discoveryFailure(fmt.Errorf("TCP listener tables are unavailable"))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	byInode := make(map[string][]ProcessIdentity)
	entries, err := os.ReadDir(s.root)
	if err == nil {
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			pid, err := strconv.Atoi(entry.Name())
			if err != nil || pid < 1 || !entry.IsDir() {
				continue
			}
			nameBytes, _ := os.ReadFile(filepath.Join(s.root, entry.Name(), "comm"))
			name := strings.TrimSpace(string(nameBytes))
			fds, err := os.ReadDir(filepath.Join(s.root, entry.Name(), "fd"))
			if err != nil {
				continue
			}
			for _, fd := range fds {
				target, err := os.Readlink(filepath.Join(s.root, entry.Name(), "fd", fd.Name()))
				if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
					continue
				}
				inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
				identity := ProcessIdentity{PID: pid, Name: name}
				duplicate := false
				for _, existing := range byInode[inode] {
					if existing == identity {
						duplicate = true
						break
					}
				}
				if !duplicate {
					byInode[inode] = append(byInode[inode], identity)
				}
			}
		}
	}

	listeners := make([]rawListener, 0, len(sockets))
	for _, socket := range sockets {
		identities := byInode[socket.Inode]
		if len(identities) == 0 {
			listeners = append(listeners, rawListener{Address: socket.Address, Port: socket.Port})
			continue
		}
		for _, identity := range identities {
			listeners = append(listeners, rawListener{Address: socket.Address, Port: socket.Port, PID: identity.PID, Process: identity.Name})
		}
	}
	return listeners, nil
}
