package local_ports

import (
	"context"
	"net/netip"
	"sort"
)

type ProcessIdentity struct {
	PID  int    `json:"pid,omitempty"`
	Name string `json:"name,omitempty"`
	// Sandbox names the Docker Sandbox behind an sbx listener.
	Sandbox string `json:"sandbox,omitempty"`
	// Container names the Docker or Podman container behind a proxy listener.
	Container string `json:"container,omitempty"`
	// Shadowed marks a process that holds the port only on a wildcard or
	// non-loopback address while another process binds the loopback address
	// itself. Connections to localhost reach the other process.
	Shadowed bool `json:"shadowed,omitempty"`
	// ListensBeyondLoopback is set for the processes that make the port reachable
	// from other machines.
	ListensBeyondLoopback bool `json:"listens_beyond_loopback,omitempty"`
}

type processKey struct {
	PID  int
	Name string
}

type processBinding struct {
	loopback bool // binds a loopback address itself
	beyond   bool
}

type Listener struct {
	unknownOwner          bool
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
		candidate    bool
		unknownOwner bool
		beyond       bool
		processes    map[processKey]*processBinding
		loopback     bool
	}
	groups := make(map[int]*group)
	for _, record := range records {
		if record.Port < 1 || record.Port > 65535 || record.Port == excludedPort || !record.Address.IsValid() {
			continue
		}
		current := groups[record.Port]
		if current == nil {
			current = &group{
				processes: make(map[processKey]*processBinding),
			}
			groups[record.Port] = current
		}
		if record.Address.IsLoopback() || record.Address.IsUnspecified() {
			current.candidate = true
		}
		if !record.Address.IsLoopback() {
			current.beyond = true
		}
		if record.Process == "" {
			current.unknownOwner = true
		}
		if record.PID > 0 || record.Process != "" {
			key := processKey{PID: record.PID, Name: record.Process}
			binding := current.processes[key]
			if binding == nil {
				binding = &processBinding{}
				current.processes[key] = binding
			}
			binding.loopback = binding.loopback || record.Address.IsLoopback()
			binding.beyond = binding.beyond || !record.Address.IsLoopback()
		}
		if record.Address.IsLoopback() {
			current.loopback = true
		}
	}

	listeners := make([]Listener, 0, len(groups))
	for port, current := range groups {
		if !current.candidate {
			continue
		}
		processes := make([]ProcessIdentity, 0, len(current.processes))
		for key, binding := range current.processes {
			processes = append(processes, ProcessIdentity{PID: key.PID, Name: key.Name, Shadowed: current.loopback && !binding.loopback, ListensBeyondLoopback: binding.beyond})
		}
		sort.Slice(processes, func(i, j int) bool {
			if processes[i].Shadowed != processes[j].Shadowed {
				return !processes[i].Shadowed
			}
			if processes[i].Name != processes[j].Name {
				return processes[i].Name < processes[j].Name
			}
			return processes[i].PID < processes[j].PID
		})
		listeners = append(listeners, Listener{unknownOwner: current.unknownOwner, Port: port, Processes: processes, ListensBeyondLoopback: current.beyond})
	}
	sort.Slice(listeners, func(i, j int) bool { return listeners[i].Port < listeners[j].Port })
	return listeners, nil
}
