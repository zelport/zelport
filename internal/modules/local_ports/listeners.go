package local_ports

import (
	"context"
	"net/netip"
	"sort"
	"time"
)

type ProcessIdentity struct {
	PID  int    `json:"pid,omitempty"`
	Name string `json:"name,omitempty"`
}

// ListenerBinding is retained for source attribution, but is deliberately not
// part of the public Local Ports response.
type ListenerBinding struct {
	Address  netip.Addr `json:"-"`
	Protocol string     `json:"-"`
}

type Listener struct {
	Port                  int                 `json:"port"`
	Processes             []ProcessIdentity   `json:"processes"`
	Bindings              []ListenerBinding   `json:"-"`
	Containers            []ContainerIdentity `json:"containers,omitempty"`
	Sandboxes             []SandboxIdentity   `json:"sandboxes,omitempty"`
	Kind                  string              `json:"kind,omitempty"`
	SourceStatus          string              `json:"source_status,omitempty"`
	ListensBeyondLoopback bool                `json:"listens_beyond_loopback"`
}

// ContainerIdentity describes a published local mapping without making Local
// Ports depend on the Containers feature module.
type ContainerIdentity struct {
	Name          string     `json:"name"`
	Engine        string     `json:"engine"`
	CLI           string     `json:"cli"`
	Runtime       string     `json:"runtime"`
	ContainerPort int        `json:"container_port"`
	Protocol      string     `json:"protocol"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
}

// SandboxIdentity describes a published Docker Sandbox mapping without making
// Local Ports depend on the Docker Sandboxes feature module.
type SandboxIdentity struct {
	Name        string     `json:"name"`
	SandboxPort int        `json:"sandbox_port"`
	Protocol    string     `json:"protocol"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`
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
		bindings  map[ListenerBinding]struct{}
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
				bindings:  make(map[ListenerBinding]struct{}),
			}
			groups[record.Port] = current
		}
		current.bindings[ListenerBinding{Address: record.Address, Protocol: "tcp"}] = struct{}{}
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
		bindings := make([]ListenerBinding, 0, len(current.bindings))
		for binding := range current.bindings {
			bindings = append(bindings, binding)
		}
		sort.Slice(bindings, func(i, j int) bool {
			if bindings[i].Address != bindings[j].Address {
				return bindings[i].Address.Compare(bindings[j].Address) < 0
			}
			return bindings[i].Protocol < bindings[j].Protocol
		})
		listeners = append(listeners, Listener{Port: port, Processes: processes, Bindings: bindings, ListensBeyondLoopback: current.beyond})
	}
	sort.Slice(listeners, func(i, j int) bool { return listeners[i].Port < listeners[j].Port })
	return listeners, nil
}
