package local_ports

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/portorigin"
	"github.com/zelport/zelport/internal/sbx"
)

const (
	KindLocalProcess  = "local_process"
	KindContainer     = "container"
	KindDockerSandbox = "docker_sandbox"
	KindUnknown       = "unknown"

	SourceReady      = "ready"
	SourceStale      = "stale"
	SourceRefreshing = "refreshing"
	SourceDegraded   = "degraded"
	SourceAmbiguous  = "ambiguous"
)

type containerMatch struct {
	identity ContainerIdentity
	key      string
	rank     int
	stale    bool
}

type sandboxMatch struct {
	identity SandboxIdentity
	key      string
	rank     int
}

func attributeListeners(listeners []Listener, snapshot portorigin.Snapshot) []Listener {
	result := make([]Listener, len(listeners))
	for i, listener := range listeners {
		listener.Processes = append([]ProcessIdentity{}, listener.Processes...)
		listener.Bindings = append([]ListenerBinding{}, listener.Bindings...)
		listener.Containers = nil
		listener.Sandboxes = nil

		sandboxes := matchingSandboxes(listener, snapshot.Sandboxes)
		if len(sandboxes) > 0 {
			if len(sandboxes) > 1 {
				listener.Kind = KindUnknown
				listener.SourceStatus = SourceAmbiguous
			} else {
				listener.Kind = KindDockerSandbox
				listener.SourceStatus = positiveStatus(snapshot.Health, false)
				listener.Sandboxes = []SandboxIdentity{sandboxes[0].identity}
			}
			result[i] = listener
			continue
		}

		containers := matchingContainers(listener, snapshot.Containers)
		if len(containers) > 0 {
			if len(containers) > 1 {
				listener.Kind = KindUnknown
				listener.SourceStatus = SourceAmbiguous
			} else {
				listener.Kind = KindContainer
				listener.SourceStatus = positiveStatus(snapshot.Health, containers[0].stale)
				listener.Containers = []ContainerIdentity{containers[0].identity}
			}
			result[i] = listener
			continue
		}

		listener.Kind, listener.SourceStatus = negativeClassification(snapshot.Health)
		result[i] = listener
	}
	return result
}

func matchingContainers(listener Listener, inventories []containers.Inventory) []containerMatch {
	matches := make([]containerMatch, 0)
	bestRank := 0
	seen := make(map[string]struct{})
	for _, inventory := range inventories {
		for _, container := range inventory.Containers {
			if container.Status != "" && container.Status != "running" {
				continue
			}
			for _, port := range container.Ports {
				if port.HostPort != listener.Port || normalizedProtocol(port.Protocol) != "tcp" {
					continue
				}
				binding, ok := port.Binding()
				if !ok {
					continue
				}
				rank := bindingRank(listener.Bindings, binding, port.Protocol)
				if rank == 0 || rank < bestRank {
					continue
				}
				if rank > bestRank {
					matches = matches[:0]
					clear(seen)
					bestRank = rank
				}
				fingerprint := inventory.Endpoint + "\x00" + inventory.Connection + "\x00" + inventory.Namespace
				key := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%s", fingerprint, container.ID, binding, port.HostPort, port.ContainerPort, normalizedProtocol(port.Protocol))
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				cli := inventory.Engine
				if inventory.CLIPath != "" {
					cli = filepath.Base(inventory.CLIPath)
				}
				matches = append(matches, containerMatch{
					identity: ContainerIdentity{
						Name: container.Name, Engine: inventory.Engine, CLI: cli, Runtime: inventory.RuntimeStatus,
						ContainerPort: port.ContainerPort, Protocol: normalizedProtocol(port.Protocol),
					},
					key: key, rank: rank, stale: inventory.Stale,
				})
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].key < matches[j].key })
	return matches
}

func matchingSandboxes(listener Listener, boxes []sbx.Sandbox) []sandboxMatch {
	matches := make([]sandboxMatch, 0)
	bestRank := 0
	seen := make(map[string]struct{})
	for _, box := range boxes {
		if box.Status != "" && box.Status != "running" {
			continue
		}
		for _, port := range box.Ports {
			if port.HostPort != listener.Port || normalizedProtocol(port.Protocol) != "tcp" {
				continue
			}
			binding, ok := port.Binding()
			if !ok {
				continue
			}
			rank := bindingRank(listener.Bindings, binding, port.Protocol)
			if rank == 0 || rank < bestRank {
				continue
			}
			if rank > bestRank {
				matches = matches[:0]
				clear(seen)
				bestRank = rank
			}
			key := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d\x00%s", box.ID, binding, port.HostPort, port.SandboxPort, rank, normalizedProtocol(port.Protocol))
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			matches = append(matches, sandboxMatch{
				identity: SandboxIdentity{Name: box.Name, SandboxPort: port.SandboxPort, Protocol: normalizedProtocol(port.Protocol)},
				key:      key, rank: rank,
			})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].key < matches[j].key })
	return matches
}

func bindingRank(bindings []ListenerBinding, published netip.Addr, protocol string) int {
	best := 0
	for _, listener := range bindings {
		if normalizedProtocol(listener.Protocol) != "tcp" || !protocolAllowsAddress(protocol, published) || listener.Address.Is6() != published.Is6() {
			continue
		}
		rank := 0
		switch {
		case published.IsUnspecified():
			rank = 1
		case listener.Address == published:
			rank = 2
		case listener.Address.IsUnspecified():
			rank = 1
		}
		if rank > best {
			best = rank
		}
	}
	return best
}

func protocolAllowsAddress(protocol string, address netip.Addr) bool {
	switch strings.ToLower(protocol) {
	case "tcp4":
		return address.Is4()
	case "tcp6":
		return address.Is6()
	default:
		return true
	}
}

func normalizedProtocol(protocol string) string {
	switch strings.ToLower(protocol) {
	case "tcp", "tcp4", "tcp6":
		return "tcp"
	default:
		return strings.ToLower(protocol)
	}
}

func positiveStatus(health portorigin.Health, explicitlyStale bool) string {
	if explicitlyStale || health == portorigin.Stale {
		return SourceStale
	}
	return SourceReady
}

func negativeClassification(health portorigin.Health) (string, string) {
	switch health {
	case portorigin.Ready:
		return KindLocalProcess, SourceReady
	case portorigin.Stale:
		return KindUnknown, SourceStale
	case portorigin.Degraded:
		return KindUnknown, SourceDegraded
	default:
		return KindUnknown, SourceRefreshing
	}
}
