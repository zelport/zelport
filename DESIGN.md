# Zelport App architecture

This document describes the implemented structure of Zelport App. Product
operation belongs in [README.md](README.md), module contracts in
[docs/MODULE_CONTRACTS.md](docs/MODULE_CONTRACTS.md), and the relay wire format
in [docs/ports-over-relay-v1.md](docs/ports-over-relay-v1.md).

## Composition

`cmd/zelport` assembles core with the five standard modules. Core owns the
loopback listeners, configuration, route registry, administration API and web
shell. Modules contribute service sources, settings, route handlers and embedded
UI assets through typed contracts.

The standard build contains Local Ports, Remote HTTP, Ports over SSH, Ports over
Relay and Built-in Relay. `zelport_core` and individual module tags remain for
dependency-boundary tests.

## Packages and ownership

- `cmd/zelport` owns process startup, flags, listener binding and shutdown.
- `internal/app` owns schema-v1 configuration, revisions, routes and leases.
- `internal/feature` defines contracts shared by core and modules.
- `internal/modulekit` provides common lifecycle and reverse-proxy helpers.
- `internal/modules/<id>` owns one module's settings, routes, APIs and assets.
- `internal/web` owns protected administration and the browser shell.
- `internal/sshpipe` isolates system SSH transport.
- `internal/relay/integration` verifies App, SDK and relay behavior together.

The SDK and relay are separately buildable sibling modules. Production source
uses released module versions and contains no local `replace` directive or
workspace dependency.

## Configuration and commits

Core accepts configuration version `1` only. The file contains module-owned raw
settings and generic route envelopes. Modules decode and validate their own
payloads. A commit validates the complete next state, prepares affected modules,
persists atomically, activates prepared state and then revokes replaced
resources. A failed validation, preparation or write leaves the previous live
state intact.

Configuration revision protects interactive edits. Metadata-only commits, such
as relay catalog watermarks, preserve the edit revision so background protocol
progress does not invalidate an open form.

## Request routing and ownership

The listener normalizes the Host authority, reserves exact `localhost` for
administration and resolves other `*.localhost` names through the immutable
route snapshot. Each dispatch acquires a lease. Route replacement, removal or
shutdown revokes the lease and closes registered requests, streams, upgraded
connections, child processes and module resources.

Self-target checks reject backends that resolve to the active Zelport listener.
Modules may dispatch internally only through the typed host contract; they do
not call the public listener to reach another route.

## Administration boundary

Mutation endpoints require exact Origin validation and the per-process CSRF
token. Browser administration is available only on the exact local authority.
The read-only local integration API has its own strict loopback contract.
Authenticated relay administration uses the reserved
`*.zelport.localhost` namespace and an explicit typed bridge; forwarded headers
never grant that authority.

## Service sources

Local, remote, SSH and relay sources adapt external web services to the same
route and lease model. System SSH remains an optional runtime dependency and is
never bundled.

The Local Ports service list fetches one listener inventory for its configured
ports and refreshes it in the background while visible. It shows process details
and a non-loopback warning from listener data. It shows the neutral no-listener
indicator only after a successful discovery. Unavailable discovery is not
evidence that a listener disappeared.
Per-service Edit and Remove actions live in the row's actions menu.

Ports over Relay owns client identity, encrypted catalogs, peer sessions,
imports and remote administration. Built-in Relay owns relay listeners and
admission. They share the public SDK/server implementations but retain separate
configuration and lifecycle boundaries.
New Built-in Relay settings leave access-token admission disabled by default;
the UI warns that reachable clients may connect when relay hosting is enabled.
Explicit saved choices are preserved.

## Relay security boundary

Relay Protocol v1 separates client-to-relay admission from peer-to-peer
authentication and encryption. Relay admission controls who may use a server;
peer identity pins and group secrets protect catalogs and tunneled traffic.
Relay transport never turns an untrusted group member into a restricted tenant:
one group is one mutually trusted administration boundary.

The only WebSocket endpoint is `/v1/ws`. Cryptographic contexts are centralized
in the SDK under `zelport-relay/v1`. Unknown versions and fields are rejected;
there is no negotiation or fallback.

## Frontend

The web UI is embedded static HTML, CSS and JavaScript with no runtime CDN or
frontend build step. Core renders shared navigation and components; modules
provide declarative metadata and module-owned fragments. UI code must follow
[the design system](docs/UI_DESIGN_SYSTEM.md) and remain stable when discovery
updates arrive while a user is interacting with a control.

Warnings use the shared amber outlined triangle and communicate conditions that
deserve attention without presenting them as failures. Errors retain the
separate danger treatment; modules must not introduce their own warning glyphs.

Ordinary Save/Cancel forms do not announce generic unsaved changes. Feedback
appears only when it explains a concrete consequence or recovery action, such
as a configuration conflict with an explicit reload control.

Interactive icon controls use the shared controlled tooltip, never the browser
native `title` tooltip. Tooltip hover delay is **250 ms**; keyboard focus shows
the same tooltip immediately. Tooltips close on pointer leave, Escape, scroll
and resize so they do not cover unrelated content.

## Verification

`make check` runs Go tests, race detection, vet, release/packaging tests and the
browser script suites. `make check-modules` verifies the complete build and
diagnostic boundary combinations. Cross-component relay tests live under
`internal/relay/integration`; native platform acceptance remains a separate
release prerequisite where documented.
