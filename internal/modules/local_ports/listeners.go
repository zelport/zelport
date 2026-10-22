package local_ports

import (
	"context"
	"net/netip"
	"sort"
)

type ProcessIdentity struct {
	PID  int    `json:"pid,omitempty"`
	Name string `json:"name,omitempty"`
}

type Listener struct {
	Port                  int               `json:"port"`
	Processes             []ProcessIdentity `json:"processes"`
	ListensBeyondLoopback bool              `json:"listens_beyond_loopback"`
}

type rawListener struct {
	Address netip.Addr
	Port    int
	PID     int
	Process string
}

type listenerScanner interface {
	Scan(context.Context) ([]rawListener, error)
}

func discoverListeners(ctx context.Context, scanner listenerScanner, excludedPort int) ([]Listener, error) {
	records, err := scanner.Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	type group struct {
		candidate bool
		beyond    bool
		processes map[ProcessIdentity]struct{}
	}
	groups := make(map[int]*group)
	for _, record := range records {
		if record.Port < 1 || record.Port > 65535 || record.Port == excludedPort || !record.Address.IsValid() {
			continue
		}
		current := groups[record.Port]
		if current == nil {
			current = &group{
				processes: make(map[ProcessIdentity]struct{}),
			}
			groups[record.Port] = current
		}
		if record.Address.IsLoopback() || record.Address.IsUnspecified() {
			current.candidate = true
		}
		if !record.Address.IsLoopback() {
			current.beyond = true
		}
		if record.PID > 0 || record.Process != "" {
			current.processes[ProcessIdentity{PID: record.PID, Name: record.Process}] = struct{}{}
		}
	}

	listeners := make([]Listener, 0, len(groups))
	for port, current := range groups {
		if !current.candidate {
			continue
		}
		processes := make([]ProcessIdentity, 0, len(current.processes))
		for process := range current.processes {
			processes = append(processes, process)
		}
		sort.Slice(processes, func(i, j int) bool {
			if processes[i].Name != processes[j].Name {
				return processes[i].Name < processes[j].Name
			}
			return processes[i].PID < processes[j].PID
		})
		listeners = append(listeners, Listener{Port: port, Processes: processes, ListensBeyondLoopback: current.beyond})
	}
	sort.Slice(listeners, func(i, j int) bool { return listeners[i].Port < listeners[j].Port })
	return listeners, nil
}
