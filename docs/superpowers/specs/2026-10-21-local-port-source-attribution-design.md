# Local Port Source Attribution Design

## Purpose

Local Ports must identify whether a detected TCP listener is backed by a local
process, a published container port, or a Docker Sandbox mapping on macOS and
Linux. Classification must use authoritative runtime inventories rather than
host proxy process names such as `gvproxy`, `docker-proxy`, `rootlessport`, or
`conmon`.

Opening the port picker must not wait for Docker, Podman, nerdctl, or `sbx`.
An explicit refresh may keep a background request active, but the already rendered
list, manual Aux-port entry, and selection remain available while it runs.

## Current failure

The Local Ports request currently performs listener discovery, Docker Sandbox
discovery, and container discovery sequentially under one two-second context.
It creates a new container catalog for every request. On macOS the sandbox work
can consume the deadline before Podman is queried, so a Podman-published port is
returned as the host process `gvproxy` even though Podman reports the mapping
correctly.

The Containers feature already keeps an independent background catalog, but
Local Ports cannot import or depend on that feature module. Feature availability
must not control source attribution.

## Architecture

A module-neutral source-attribution service will live outside the feature
modules. It owns read-only providers for:

- Docker, Podman, and nerdctl published-port inventories through
  `internal/containers`;
- Docker Sandbox published-port inventories through `internal/sbx`.

Local Ports owns the service lifecycle and starts it when the module starts, so
classification works whether or not the Containers or Docker Sandboxes feature
modules are enabled. The service is reusable by other modules but imports no
feature module.

Each provider refreshes independently in the background. Refreshes use
provider-specific deadlines and never run on the Local Ports HTTP request path.
A successful result atomically replaces that provider's snapshot. A failed
refresh preserves its last successful snapshot and marks it stale with a safe
status. Only one refresh per provider may run at a time.

The service performs an eager refresh at startup and periodic refreshes every
15 seconds. A UI refresh requests another coalesced background refresh. The
refresh action remains busy until that generation completes or its provider
deadlines expire, then fetches a new listener response. Existing rows are never
cleared while it runs. This avoids spawning multiple runtime CLIs when users
open or repeatedly refresh the picker.

## Request and rendering latency

`GET /api/modules/local_ports/listeners` performs only operating-system listener
collection and in-memory attribution against the latest immutable snapshots. It
must never execute Docker, Podman, nerdctl, or `sbx`.

The endpoint retains a strict two-second safety deadline for the operating-system
scanner, but normal rendering is not coupled to any runtime CLI deadline. Cached
attribution is an in-memory lookup. The response may therefore contain pending
or stale attribution while a background refresh is running, but it must return
the listener list without waiting for that refresh.

## Matching and precedence

Runtime-published mappings are authoritative. Host process metadata is only a
fallback because virtualization and rootless networking commonly expose a proxy
process rather than the workload.

Matching uses a normalized host binding key containing protocol, address family,
bind scope, and host port. It must not match on the port number alone. Listener
normalization therefore retains its internal binding records even though the UI
continues to render one selectable row per Aux port.

The display precedence is:

1. Docker Sandbox mapping;
2. container mapping;
3. confirmed local process;
4. unknown source.

An exact loopback binding wins over a wildcard candidate for the same address
family. IPv4 and IPv6 mappings are correlated independently and then folded into
the existing port row. If distinct authoritative workloads claim bindings that
remain indistinguishable after normalization, the result is ambiguous and must
not be labeled as a local process.

Container results from multiple CLIs are deduplicated only when their runtime
connection fingerprint, container ID, protocol, host binding, host port, and
container port are identical. Merely sharing a container name or host port is
not sufficient. Distinct runtime endpoints remain distinct sources.

## Classification confidence

Each response includes attribution health independently of the selected display
source:

- `ready`: every installed provider relevant to local attribution completed a
  successful refresh;
- `refreshing`: no complete first snapshot exists yet;
- `stale`: a previous successful snapshot is being used after a refresh failure;
- `degraded`: one or more installed providers failed and no trustworthy previous
  snapshot exists.

`Local process` is shown only when all installed attribution providers have a
successful current snapshot and none reports a matching published mapping. If
discovery is refreshing, stale, degraded, or ambiguous, absence of a runtime
match is not proof of locality; the kind is `Unknown source`. Available process
metadata may still be shown as diagnostic context, but it is not presented as
the authoritative kind.

A confirmed container or sandbox match remains usable when an unrelated provider
is unavailable because positive authoritative evidence does not depend on
proving the absence of other sources.

When the provider that supplied a positive match later fails, its last-known-good
container or sandbox identity remains visible with `stale` health and a
last-verified timestamp. It is never silently converted to `Local process`.

## UI contract

The picker renders available listeners immediately from the endpoint response.
Existing kind icons and normalized tooltips remain the presentation mechanism.

- Confirmed local listener: `Kind: Local process`.
- Confirmed container mapping: `Kind: Container` plus container, mapping, CLI,
  and runtime information.
- Confirmed Docker Sandbox mapping: `Kind: Docker Sandbox` plus sandbox,
  mapping, CLI, and runtime information.
- Unconfirmed or ambiguous listener: `Kind: Unknown source` plus available
  process information and a concise attribution-status explanation.

An attribution refresh updates the existing list after its background generation
finishes; it does not block or clear the first render. Manual selection is always
allowed because choosing an Aux port does not require knowing its source.

## Platform behavior

On macOS, Docker and Podman commonly publish through a Linux VM. The engine's
published-port inventory supplies workload identity; `lsof` supplies only the
host binding and proxy process metadata.

On Linux, the same engine inventory is authoritative regardless of whether the
host socket is represented by the container process, `docker-proxy`, a rootless
forwarder, or no readable PID. Native Podman is pinned to local mode; remote
container endpoints are never treated as local published ports.

No classifier infers a runtime from process-name substrings.

## Error handling and resource bounds

- Missing CLIs are recorded as not installed and do not make attribution
  degraded.
- Installed but unreachable runtimes are isolated from other providers.
- Malformed or partial CLI output never replaces a last-known-good snapshot.
- Provider errors exposed to the UI are normalized and do not include command
  lines, environment values, socket paths, or credentials.
- Shutdown cancels active refreshes and waits for owned goroutines.
- Refresh requests are coalesced so repeated UI actions cannot create an
  unbounded number of CLI processes.

## Verification

Automated tests must cover:

- the reproduced macOS failure: a slow sandbox provider cannot prevent cached
  Podman mappings for ports 1212 and 1313 from being classified as containers;
- the listener endpoint never invoking runtime CLIs and returning while a
  provider is deliberately blocked;
- eager, periodic, manual, coalesced, cancelled, failed, and last-known-good
  refresh behavior;
- Docker, Podman, and nerdctl independently installed, missing, unavailable,
  and concurrently available;
- duplicate aliases for the same runtime mapping and conflicting mappings from
  distinct runtimes;
- IPv4 loopback, IPv6 loopback, wildcard, mixed-scope, and same-port bindings;
- macOS proxy-process fixtures and Linux proxy/rootless/no-PID fixtures;
- `Local process` only with complete successful negative attribution;
- `Unknown source` for pending, degraded, stale-negative, and ambiguous cases;
- immediate picker rendering while attribution refresh is blocked;
- race-enabled Go tests, JavaScript UI tests, Linux tests, Darwin cross-builds,
  and a native macOS verification against the two Podman fixture containers.

