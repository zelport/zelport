package local_ports

import (
	"net/netip"
	"testing"
	"time"

	"github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/portorigin"
	"github.com/zelport/zelport/internal/sbx"
)

func testListener(port int, addresses ...string) Listener {
	bindings := make([]ListenerBinding, 0, len(addresses))
	for _, address := range addresses {
		bindings = append(bindings, ListenerBinding{Address: netip.MustParseAddr(address), Protocol: "tcp"})
	}
	return Listener{Port: port, Processes: []ProcessIdentity{}, Bindings: bindings}
}

func testInventory(engine, connection, id, name, binding string, hostPort, containerPort int) containers.Inventory {
	return containers.Inventory{
		Engine: engine, Connection: connection, CLIPath: "/usr/local/bin/" + engine, RuntimeStatus: "available",
		Containers: []containers.Container{{ID: id, Name: name, Status: "running", Ports: []containers.Port{{
			HostIP: loopbackFor(binding), PublishedIP: binding, HostPort: hostPort, ContainerPort: containerPort, Protocol: "tcp",
		}}}},
	}
}

func loopbackFor(binding string) string {
	if netip.MustParseAddr(binding).Is6() {
		return "::1"
	}
	return "127.0.0.1"
}

func TestAttributeListenersPrefersSandboxOverContainer(t *testing.T) {
	listener := testListener(49152, "127.0.0.1")
	snapshot := portorigin.Snapshot{
		Health:     portorigin.Ready,
		Containers: []containers.Inventory{testInventory("docker", "local", "container-id", "nested", "127.0.0.1", 49152, 80)},
		Sandboxes: []sbx.Sandbox{{ID: "sandbox-id", Name: "workspace", Status: "running", Ports: []sbx.Port{{
			HostIP: "127.0.0.1", PublishedIP: "127.0.0.1", HostPort: 49152, SandboxPort: 3000, Protocol: "tcp",
		}}}},
	}

	got := attributeListeners([]Listener{listener}, snapshot)[0]
	if got.Kind != KindDockerSandbox || got.SourceStatus != SourceReady || len(got.Sandboxes) != 1 || len(got.Containers) != 0 {
		t.Fatalf("attribution = %#v", got)
	}
}

func TestAttributeListenersIdentifiesPodmanBehindGVProxy(t *testing.T) {
	listener := testListener(1212, "0.0.0.0")
	listener.Processes = []ProcessIdentity{{PID: 4973, Name: "gvproxy"}}
	snapshot := portorigin.Snapshot{Health: portorigin.Ready, Containers: []containers.Inventory{
		testInventory("podman", "podman-machine-default", "podman-id", "demo", "0.0.0.0", 1212, 80),
	}}

	got := attributeListeners([]Listener{listener}, snapshot)[0]
	if got.Kind != KindContainer || got.SourceStatus != SourceReady || len(got.Containers) != 1 || got.Containers[0].Engine != "podman" {
		t.Fatalf("attribution = %#v", got)
	}
	if len(got.Processes) != 1 || got.Processes[0].Name != "gvproxy" {
		t.Fatalf("process context was lost: %#v", got.Processes)
	}
}

func TestAttributeListenersMatchesLinuxProxyWithoutPID(t *testing.T) {
	listener := testListener(18080, "0.0.0.0")
	snapshot := portorigin.Snapshot{Health: portorigin.Ready, Containers: []containers.Inventory{
		testInventory("podman", "rootless", "rootless-id", "web", "0.0.0.0", 18080, 8080),
	}}

	got := attributeListeners([]Listener{listener}, snapshot)[0]
	if got.Kind != KindContainer || len(got.Containers) != 1 || got.Containers[0].Name != "web" {
		t.Fatalf("attribution = %#v", got)
	}
}

func TestAttributeListenersUsesExactBindingBeforeWildcardAndKeepsFamiliesSeparate(t *testing.T) {
	listener := testListener(8080, "127.0.0.1")
	snapshot := portorigin.Snapshot{Health: portorigin.Ready, Containers: []containers.Inventory{
		testInventory("podman", "local", "wild-v4", "wildcard", "0.0.0.0", 8080, 80),
		testInventory("podman", "local", "exact-v4", "exact", "127.0.0.1", 8080, 81),
		testInventory("podman", "local", "wild-v6", "ipv6", "::", 8080, 82),
	}}

	got := attributeListeners([]Listener{listener}, snapshot)[0]
	if got.Kind != KindContainer || len(got.Containers) != 1 || got.Containers[0].Name != "exact" {
		t.Fatalf("attribution = %#v", got)
	}
}

func TestAttributeListenersKeepsBestMatchPerAddressFamily(t *testing.T) {
	listener := testListener(8080, "127.0.0.1", "::")
	snapshot := portorigin.Snapshot{Health: portorigin.Ready, Containers: []containers.Inventory{
		testInventory("podman", "machine", "v4-id", "ipv4", "127.0.0.1", 8080, 80),
		testInventory("docker", "local", "v6-id", "ipv6", "::", 8080, 80),
	}}

	got := attributeListeners([]Listener{listener}, snapshot)[0]
	if got.Kind != KindUnknown || got.SourceStatus != SourceAmbiguous {
		t.Fatalf("dual-stack competing attribution = %#v", got)
	}
}

func TestAttributeListenersDeduplicatesAliasesButRejectsDistinctRuntimes(t *testing.T) {
	listener := testListener(8080, "127.0.0.1")
	duplicate := testInventory("docker", "local", "same-id", "web", "127.0.0.1", 8080, 80)
	alias := duplicate
	alias.Engine = "nerdctl"
	alias.CLIPath = "/usr/local/bin/nerdctl"
	snapshot := portorigin.Snapshot{Health: portorigin.Ready, Containers: []containers.Inventory{duplicate, alias}}

	got := attributeListeners([]Listener{listener}, snapshot)[0]
	if got.Kind != KindContainer || len(got.Containers) != 1 {
		t.Fatalf("deduplicated attribution = %#v", got)
	}

	snapshot.Containers = append(snapshot.Containers, testInventory("podman", "machine", "other-id", "other", "127.0.0.1", 8080, 80))
	got = attributeListeners([]Listener{listener}, snapshot)[0]
	if got.Kind != KindUnknown || got.SourceStatus != SourceAmbiguous || len(got.Containers) != 0 {
		t.Fatalf("ambiguous attribution = %#v", got)
	}
}

func TestAttributeListenersHealthAwareNegativeAndPositiveResults(t *testing.T) {
	listener := testListener(8080, "127.0.0.1")
	cases := []struct {
		name       string
		health     portorigin.Health
		wantKind   string
		wantStatus string
	}{
		{name: "complete negative", health: portorigin.Ready, wantKind: KindLocalProcess, wantStatus: SourceReady},
		{name: "refreshing", health: portorigin.Refreshing, wantKind: KindUnknown, wantStatus: SourceRefreshing},
		{name: "degraded", health: portorigin.Degraded, wantKind: KindUnknown, wantStatus: SourceDegraded},
		{name: "stale negative", health: portorigin.Stale, wantKind: KindUnknown, wantStatus: SourceStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := attributeListeners([]Listener{listener}, portorigin.Snapshot{Health: tc.health})[0]
			if got.Kind != tc.wantKind || got.SourceStatus != tc.wantStatus {
				t.Fatalf("attribution = %#v", got)
			}
		})
	}

	stale := testInventory("podman", "machine", "id", "web", "127.0.0.1", 8080, 80)
	stale.Stale = true
	stale.UpdatedAt = time.Unix(100, 0).UTC()
	got := attributeListeners([]Listener{listener}, portorigin.Snapshot{Health: portorigin.Stale, Containers: []containers.Inventory{stale}})[0]
	if got.Kind != KindContainer || got.SourceStatus != SourceStale || len(got.Containers) != 1 || got.Containers[0].VerifiedAt == nil || !got.Containers[0].VerifiedAt.Equal(stale.UpdatedAt) {
		t.Fatalf("positive stale attribution = %#v", got)
	}

	current := stale
	current.Stale = false
	got = attributeListeners([]Listener{listener}, portorigin.Snapshot{Health: portorigin.Stale, Containers: []containers.Inventory{current}})[0]
	if got.Kind != KindContainer || got.SourceStatus != SourceReady {
		t.Fatalf("unrelated provider failure tainted current match = %#v", got)
	}

	verified := time.Unix(200, 0).UTC()
	got = attributeListeners([]Listener{testListener(49152, "127.0.0.1")}, portorigin.Snapshot{
		Health: portorigin.Stale, SandboxesStale: true, SandboxesUpdatedAt: verified,
		Sandboxes: []sbx.Sandbox{{ID: "sandbox", Name: "workspace", Status: "running", Ports: []sbx.Port{{
			HostIP: "127.0.0.1", PublishedIP: "127.0.0.1", HostPort: 49152, SandboxPort: 3000, Protocol: "tcp",
		}}}},
	})[0]
	if got.Kind != KindDockerSandbox || got.SourceStatus != SourceStale || got.Sandboxes[0].VerifiedAt == nil || !got.Sandboxes[0].VerifiedAt.Equal(verified) {
		t.Fatalf("stale sandbox attribution = %#v", got)
	}
}
