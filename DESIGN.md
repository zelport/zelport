# Zelport architecture

This document describes the implemented modular application. Cloud requirements and threat boundaries are in [PORTS_OVER_CLOUD.md](PORTS_OVER_CLOUD.md).
See [module architecture](docs/MODULES.md), [contracts](docs/MODULE_CONTRACTS.md),
and [README.md](README.md) for builds and operation.

## Composition and startup

Core and the five original source modules use the standard library. The relay client adds
pinned WebSocket, Noise and yamux dependencies; relay hosting adds WebSocket and
Noise without yamux. `cmd/zelport` always builds core;
positive-tagged composition files register the selected module factories. An
untagged build has no source modules. `make build-full` enumerates the seven
implemented tags. The independent `experiments/ports-over-cloud` module remains
isolated and has its own external dependencies. No cross-module imports/go.work.

CLI accepts --port, --config, --allow-root and --version. Help and version exit
before the effective-UID guard, configuration, or listeners. UID 0 is rejected
unless --allow-root or ZELPORT_ALLOW_ROOT=1 explicitly permits it; the CLI flag
overrides the environment and allowed root execution always warns on stderr.
This is an accidental-startup guard, not a boundary against an administrator.
The standalone relay uses the same policy with ZELPORT_RELAY_ALLOW_ROOT=1,
including its token creation command. The embedded relay follows the gateway
process policy; the reusable relay library does not enforce CLI policy. Startup validates module descriptors and configuration, binds IPv4 and
IPv6 loopback listeners, and starts modules in deterministic descriptor order.
IPv4-only works when IPv6 is unavailable. Bind errors are explicit. A partial
module start closes the failing module and previous modules. Signal shutdown
revokes requests and upgraded connections, cancels module workers, then closes
the HTTP server. Only included modules can start discovery jobs or adapters.

An explicit `--config` path takes precedence. On macOS and Linux, the default is
`$XDG_CONFIG_HOME/zelport/config.json` when the variable is an absolute path;
otherwise it is `~/.config/zelport/config.json`. Other platforms retain Go's
native user configuration directory. There is no fallback to the former macOS
Application Support location.

## Core contracts and configuration

`internal/feature` defines descriptors, module/host interfaces, opaque route
settings, strict JSON helpers, leases and shared HTTP error/response handling.
`internal/app` owns registry, atomic configuration and active-resource tracking.
`internal/web` owns administration and the embedded frontend shell. It imports no
source modules, CLI adapters, SSH code or cloud protocol.

Version 6 is a clean configuration break: only explicitly versioned v6 files load.
There is no migration of versions 1–5. A missing file means an empty configuration.
Each route has UUIDv4 id, module ID, normalized unique .localhost domain and
module-validated settings. Module settings live under their module ID. A configured
but excluded module fails startup with its required build tag; nothing is dropped.

Core serializes writes using expected global revisions, validates typed module
settings, writes an owner-only temporary file, fsyncs and atomically renames it.
Failed pre-rename writes preserve disk and memory. Successful writes increment
revision and invalidate changed routes before acknowledgment. Callers receive
copies. Metadata-only commits still require exact CAS and advance the persisted
revision, but leave a process-local user-edit revision unchanged. User writes may
cross metadata-only revisions; any intervening user write still conflicts. Settings
callbacks preserve the latest counters/watermarks. Metadata commits cannot change
routes or reset scopes. Coalescing watch channels notify modules outside storage locks. UI state
is explicitly sanitized by modules, never a raw dump of configuration.

## Routing and resource ownership

Gateway matches normalized Host and acquires the route at the same snapshot
revision, so a concurrent rename cannot dispatch to a replacement authority.
Project paths, including /api, always belong to the selected service. Core
local administration serves only the exact localhost authority and configured port.
The optional typed administration bridge accepts authenticated cloud requests under
the reserved `zelport.localhost` namespace; ordinary route dispatch cannot invoke it.

Each request has a lease with a route generation, cancellation context and tracked
connections. Route edit/deletion closes active frontend/backend sockets, including
WebSockets. Concurrent lease Close calls wait for teardown to finish. A resource
registered after closure is closed immediately. Module-scoped generations also
support publication/session revocation without invalidating traffic for
unrelated bookkeeping updates. Export-only dispatch excludes administration and
non-exportable module routes. Middleware and internal leased dispatch are generic;
core contains no cloud authentication implementation.

`internal/modulekit` provides generic lifecycle and reverse-proxy helpers. Per-request
HTTP transports disable connection reuse to bind backend sockets to a revocable
lease. This trades connection reuse for straightforward ownership and avoids
transparent retry on a reused HTTP connection. Proxy preserves streaming and
WebSocket duplex traffic. Local-source routes preserve Host; Remote uses target
Host. Application statuses, redirects, cookies and bodies pass through unchanged.
Dialing rejects resolved loopback addresses at the gateway port; literal self
routes are rejected at creation or return 503 if present in a loaded config.

## Source modules

- Docker Sandboxes owns `internal/sbx`, cached inventory, serialized publication,
  explicit start, mapping removal and macOS Terminal actions. Identity is pinned
  by sandbox ID; discovery failures retain configuration and last-known usable
  mappings. See the [module documentation](internal/modules/docker_sandboxes/README.md)
  for lifecycle, CLI compatibility and failure presentation.
- Containers owns Docker/Podman/nerdctl discovery through `internal/containers`. It only
  connects already published loopback TCP ports and never creates mappings or
  starts containers. Identity includes engine, connection and full container ID.
  Changed Aux mapping reconnects only to one unambiguous binding on the same
  selected address. Stale inventory preserves known usable routes.
- Local Ports forwards to an Aux port on localhost, trying IPv4 then IPv6. No CLI.
- Remote HTTP supports HTTP/HTTPS and base paths. Certificate verification is on
  by default; opt-out is route-local. Optional Origin rewriting is off by default,
  applies to HTTP and WebSocket, and preserves foreign/missing origins. TLS
  verification is isolated per request. No response URL/cookie rewriting.
- Ports over SSH owns `internal/sshpipe` pools and uses system SSH over stdio with
  `-W`, with no Aux listener. Host/user SSH configuration, strict host-key checking,
  noninteractive authentication, bounded connections/backoff and process cleanup
  retain their existing implementation. Route changes retire old pools.

Unavailable source or unknown mapping yields 503; backend failure yields 502.
Application responses retain their own status, including 403. Errors support HTML
or requested JSON and no-store. Configuration/inventory states are not health probes.

## Cloud module and relay

`internal/modules/ports_over_relay` owns settings, persistent UUID/X25519 identity,
credentials, provider pins, incoming watermarks, catalog status, API and embedded
UI. It imports no source module. The sibling `zelport-sdk-go` implements authenticated protocol and
client lifecycle; `zelport-relay` owns the server and `cmd/zelport-relay` CLI. The client
enters a Zelport binary only with `ports_over_relay`; the relay server
enters only with `built_in_relay`. Shared protocol, transport and Noise packages are included
only when at least one of those tags is present.

Protected admission uses Noise NNpsk2 with the relay access secret over WS/WSS.
Explicit open admission uses Noise NN without a PSK and a distinct prologue; an
empty client token selects it with no downgrade fallback. Open admission does not
authenticate the relay. Embedded hosting defaults to requiring a token; toggling
`require_access_token` commits before synchronously replacing the server generation
on its existing listeners. Stored tokens survive disabling protection. The standalone
CLI defaults to requiring a token from a file or environment variable; explicit
`--allow-public-access` (or its environment equivalent) enables open admission.
Conflicting token sources or a token combined with public access are rejected. Each paired
connection independently runs Noise XXpsk3 with the group-derived PSK and pinned
static identities, then yamux. The relay sees opaque group/device routing identifiers and bridges
peer bytes; it has no group secret, readable catalog, or peer traffic keys. It
rejects duplicate live routing identities and binds one-use joins to admitted
controls and exact routing contexts. Full peer UUID/key contexts stay at endpoints.
The net.Conn E2E facade below yamux owns bounded DATA/CREDIT records and fresh
XXpsk3 renewal, preserving application byte order and streams. Control progress
does not require application reads. Soft limits are 10 minutes/1 GiB; hard limits
are 15 minutes/2 GiB per direction; a renewal has at most 10 seconds to complete.
The admission layer retains its existing symmetric rekey and connection lifetime.
Admission data records combine their length prefix and ciphertext in one write
(with retries for short writes), without delayed batching. Directional locks own
separate, lazily grown buffers for outgoing records, incoming ciphertext and
plaintext; unread plaintext is copied out before reuse. Lengths are checked before
growth. Buffer capacity is bounded by the existing 32 KiB plaintext record limit,
with at most 98,340 bytes of combined slice capacity per connection across these
three buffers. Allocator rounding and other state add to actual memory usage.
The framing and cryptographic state transitions are unchanged.

The module publishes XChaCha20-Poly1305 encrypted UUID/domain records from Host.OwnServices;
imports are non-exportable. Sequence/watermarks persist before records are exposed.
Records refresh every 20 seconds, expire after 60 seconds, and tolerate 5 seconds
of future skew.
Before encryption, each canonical record is independently encoded with raw DEFLATE
(`compress/flate`, BestSpeed) only when smaller. An authenticated payload byte
selects raw (0) or DEFLATE (1). Recipients authenticate first, then decode with a
128 KiB plaintext limit and reject malformed, trailing or unsupported encodings.
Canonical plaintext remains the basis for replay digests. All Zelport peers must
use this payload format; legacy unmarked payloads are rejected. Relay envelope
framing is unchanged and this compression change needs no relay update.
Application traffic is not compressed by this feature.
Catalog fetches include up to 32 opaque route-ID/ciphertext-digest hints for
already authenticated, accepted and unexpired records. Relay omits exact matches
without changing full-snapshot size limits. Client omissions never refresh expiry,
identity pins or watermarks. Newly encrypted publication still occurs every
20 seconds; there is no separate heartbeat authorization. Updated clients require
an updated relay supporting the optional `known` control-message field. Invalid individual records produce a visible warning without
tearing down healthy peer sessions. History is bounded to 1,024 device identities.

Each request opens an acknowledged encrypted stream. The provider checks current
publishing state, UUID, source authority and scope, then serves HTTP on net.Pipe
through Host.Dispatch. The caller holds its lease through HTTP response
serialization; no Aux socket is opened. Source handlers retain their existing
behavior. Active streams are tracked before acknowledgment, so sharing disable,
connection/group replacement and source edits close affected traffic. Imports
retain their configuration offline and return 503. Reconnect uses capped backoff;
requests are never deliberately retried after lost responses.

Authenticated traversal metadata detects repeated service visits/eight-hop overflow
with 508. Header/open-envelope conflicts also fail closed. Intermediaries stripping
this metadata can defeat detection; stream/session limits bound resource use.
The [technical design](docs/ports-over-cloud-v1.md) defines the wire and limits;
[verification](docs/ports-over-cloud-verification.md) distinguishes tests from
independent security assurance and untested deployment conditions.

## Optional relay hosting

`internal/modules/built_in_relay` owns the `built_in_relay` build tag, settings, access token,
protected API and Settings section. It creates no routes and does not import the
relay client. `github.com/zelport/zelport-relay/relay.Runtime` is shared with `cmd/zelport-relay`:
Prepare reserves sockets, Start accepts traffic, and Close synchronously closes
HTTP and upgraded relay connections. The embedded default is disabled, port 8090,
all interfaces. The standalone default remains loopback. Unsupported IP families
are skipped only in wildcard mode; bind conflicts roll back every reserved socket.

Module edits serialize reservation, atomic configuration commit and server replacement.
Failure preserves the previous server and settings. Startup bind failure is runtime
error state rather than fatal module initialization. Credentials persist independently
of enabled state and are revealed only through a protected POST. State includes no
secret. Shared E2E administration permissions remain those of the existing client.

## Administration and frontend

Core provides GET /api/state and revisioned route save/delete endpoints. Each
module mounts protected /api/modules/<id>/ endpoints and its own embedded assets.
Exact Origin plus per-process CSRF protects all mutations; admin responses have
no-store, nosniff and restrictive CSP. Missing module API/assets return 404.

The embedded shell contains Dashboard, Settings and shared components/styles only. Module
scripts add tabs or Settings sections, editors and inventories via same-origin JavaScript module imports.
No CDN or frontend build. URL fragments preserve selection through reload/Back;
unknown or omitted tabs use Dashboard. Render refreshes preserve open editors and
accordion state during polling. On every entry to Settings from another tab
(including browser Back/Forward and unavailable-module redirects), all Settings
accordions start closed. Built-in Relay presents all its settings together without
a nested Advanced disclosure. Remaining on Settings
does not reset them, and closing accordions does not discard form drafts. An explicit
module shortcut such as a header popover's Open settings action may open its target
section after navigation. Module activation and polling follow section visibility.
One core scheduler polls cached state only for visible tabs that
request it; Dashboard polls if any included module needs polling. Local/Remote
alone do not poll. Manual discovery actions are separate protected mutations.

## Verification and limits

The build matrix tests core, each of seven individual modules, mixed Local/Remote, cloud with
each source, and full builds using race tests, vet, builds and binary dependency-graph checks.
Regressions cover authentication boundaries, v6 persistence/conflicts, mapping
selection/stale preservation, concurrent revocation/Hijack/domain rename, HTTP
403, Origin behavior, TLS isolation, streaming and upgraded connection deletion.
Existing CLI fixture/timeout and SSH process tests remain in their adapter packages.

Browser checks use isolated configurations and fake CLI inventories, never the
user's running services. macOS is the verified platform; real remote SSH and
public Internet cloud deployment remain unverified. Integrated cloud tests use
controlled WS/WSS services; neither those nor the historical transport experiment
constitute a completed independent cryptographic review.

## Cloud administration (protocol v3)

The cloud module always advertises one typed administration service while enabled,
independently of application publication. Authenticated catalog service triples
include their kind; typed opens carry bounded external authority and receiver-side
service links. The verified Noise peer context is passed separately from open JSON.
The provider dispatches through `AdministrationHost`, never through a caller-supplied
localhost address. Core and web do not import cloud cryptography.

Peer keys, administration UUIDs and historical hostname claims are persisted before
aliases become usable. Their changes advance the effective edit revision; repeated
catalog watermarks remain metadata writes. Internal updates use strict revision
comparison to avoid overwriting concurrent counters. Ownership is bounded to 1024
peers and 4096 alias bindings. Duplicate names fail closed; UUID aliases remain distinct.

Both ends validate the receiving authority. Mutations require one exact Origin and
a process-keyed CSRF token scoped to consumer, provider and origin. The trusted
context cannot be supplied in forwarded headers. Administration pipes belong to the
client scope, so publication withdrawal does not close them but cloud reconfiguration
does. One request uses one stream; failed mutations are never replayed automatically.

Remote state retains provider runtime facts and adds a managed-device banner and
receiver-authorized import links. Unmapped applications have no clickable URL.
Remote UI hides peer administration links to avoid chained administration.
See [operation and upgrades](README.md) and the
[design specification](docs/superpowers/specs/2026-10-12-cloud-administration-design.md).

## Relay port ownership

The shared relay Runtime reserves a fixed port before opening listeners with a
cooperative, nonblocking file lock in the OS account home under
`.zelport/relay-locks/`. It uses flock on macOS/Linux/BSD and LockFileEx on Windows.
The lock is retained through listener, upgraded connection and worker shutdown;
failed preparation releases it. Server generation replacement retains the runtime
and its reservation. Lock files are never unlinked during normal operation, to
avoid competing locks on different inodes. Unsupported platforms fail explicitly.

This is per-account cooperation across embedded and standalone relays, not a
system-wide exclusion guarantee against unrelated processes. Application config
paths and XDG cache overrides do not select the lock namespace. Normal Go socket
reuse is preserved. Only the empty bind host selects both TCP families; explicit
IPv4/IPv6 wildcard addresses select tcp4/tcp6 respectively. Internal ephemeral-port
tests lock the assigned port after the first bind and close it on lock failure.

Service lists keep their DOM nodes attached when their displayed content is
unchanged, preserving text selection, focus and scroll position. Configuration
revisions and discovery timestamps alone do not trigger a redraw; current state
is still updated for subsequent actions.

Discovery continues automatically without an update-time row. Modules can place
a Refresh icon in the shell navigation toolbar through their mount context.
Only the active module supplies its action. The shell keeps a Refresh icon in the same
place on every tab: Dashboard reloads the current state, while Settings and modules
without a refresh action show a disabled control with an explanatory tooltip.
Discovery error notices contain only the diagnostic disclosure.
The Refresh control is disabled while its request is pending.

Across the shell and module UIs, unchanged displayed content retains its DOM.
Dashboard services, relay connection controls, peer administration and catalogs
render independently. Runtime values and status text update only when changed;
expiry transitions remain part of catalog rendering even without new API data.
Module render calls still receive current state and revision on every poll, so
action freshness and credential invalidation do not depend on DOM updates.

Dashboard service state cells display only the state label for every module and
every state, without diagnostic text or diagnostic tooltips. Explanations belong
to module views; changes to undisplayed diagnostics do not redraw the Dashboard.

Dashboard service rows are retained by route ID. A change to one service updates
only its affected cells, preserving selection in other cells and rows. Existing
row order survives polling; newly discovered services append, and removed route
IDs are removed. Edit actions use the latest route snapshot.

Navigation stays directly below the shared header on every tab, with only its
bottom divider; there is no divider between the header and navigation. The hero
and localhost endpoint appear below navigation only on Dashboard. At narrow
widths, Dashboard rows retain the same DOM
but stack their domain, source, target, state and Edit action without horizontal
scrolling. State badges share neutral, healthy, pending and error tones; a
`configured` route remains neutral because configuration does not prove health.

Boolean feature settings use labelled switches. Module UI availability saves
immediately; service and relay forms retain explicit Save actions and display
`Unsaved changes` after edits. Polling does not reset drafts. Dialogs share a
labelled close control and visible footer actions; Escape first closes an open
mapping picker, then the dialog. Changing views closes open dialogs. Sandbox
action menus close on Escape, outside clicks and opening another menu. Security
tooltips dismiss on scroll and resize so they cannot cover unrelated content.
Sandbox editors preselect the effective usable
mapping, including the automatic mapping when none was explicitly pinned.

Relay credentials use shared input adornments for reveal and copy actions. The
relay client's access and encryption fields are grouped in a disclosure inside
its settings dialog. Revealed credentials are cleared on navigation and revision
changes. The built-in relay header indicator is hidden while hosting is disabled.
Unavailable catalog providers explain disabled imports, and unavailable peer
administration addresses are displayed without active links.

Service editors can use a newer configuration revision when their original
service is unchanged. A concurrent edit to that same service blocks saving and
offers an explicit reload of saved values; drafts are retained until that choice.
A removed service is never silently recreated by an old editor.

Background refresh errors are tracked separately from mutation errors. Any
successful state refresh clears transient read errors across tabs, including
those caused by a process restart. Validation and failed-save messages remain
until their owning action clears them; reconnecting must not hide those errors.

Primary navigation exposes Dashboard, Services and Settings on one row. Services
is a disclosure containing the original source-module controls, preserving their
hashes and lifecycle. Only enabled modules appear; no available sources disables
Services. Current module selection highlights both Services and its menu item.
A module heading identifies the current source. Escape, selection, outside clicks
and focus leaving the disclosure close it; arrow keys, Home and End move focus.
Refresh stays at the right. Experimental modules use inline flask icons with native tooltips and accessible maturity labels.
Navigation uses native buttons and aria-current, with labeled content regions.

Containers exposes separate CLI and runtime status for Docker, Podman and nerdctl.
CLI lookup is independent of runtime probing; a missing CLI leaves the runtime
unchecked. Runtime info succeeds before container enumeration, so inventory
failures do not falsely mark a previously verified runtime unavailable. Engine
status details render independently from container groups and ignore scan times.
The UI displays one status row per CLI and treats an absent optional CLI neutrally.
Unavailable runtimes use a warning unless configured routes depend on that engine.

Experimental nerdctl discovery supports native Linux only. It reads the effective
namespace and containerd UUID from native JSON info, pins the namespace for ps and
inspect, and rechecks the UUID before publishing an inventory. Connection identity
includes namespace and daemon UUID. VM wrappers are not accepted as proof that
guest published ports are reachable on the Zelport host. See
[Containers documentation](internal/modules/containers/README.md) for limitations.
Docker/Podman groups sharing a verified local socket and full container ID are
combined for display only; saved route identities and backend lookups stay intact.

### Docker Sandboxes CLI metadata

The sandbox module caches `sbx version --json` client metadata and the resolved
executable path separately from inventory discovery. It probes at startup and
manual refresh, not each periodic reconciliation. Version failures do not suppress
successful discovery. Inventory failures preserve cached routes and CLI metadata;
per-sandbox port errors remain diagnostics. The module-owned status header keeps
its DOM nodes and open details across polls and status transitions.

## Shared UI design system

[UI design system](docs/UI_DESIGN_SYSTEM.md) defines typography, density, surfaces,
controls, responsive layout and interaction rules. Core owns the shared tokens and
components. Source-module service lists and Dashboard/relay catalog tables share
15px main text, 8px vertical row padding and separators. Tables use explicit
`thead`/`tbody`; narrow layouts stack labeled values without shrinking the text.
Module styles must not override generic core components. The development gallery
in `scripts/ui/design-system.html` provides isolated visual states without APIs or
user configuration. It is not a production asset.
