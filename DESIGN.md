# Zelport App architecture

This document describes the implemented structure of Zelport App. Product
operation belongs in [README.md](README.md), module contracts in
[docs/MODULE_CONTRACTS.md](docs/MODULE_CONTRACTS.md), and the relay wire format
in [docs/ports-over-relay-v1.md](docs/ports-over-relay-v1.md).

## Composition

`cmd/zelport` assembles core with all seven implemented modules. Core owns the
loopback listeners, configuration, route registry, administration API and web
shell. Modules contribute service sources, settings, route handlers and embedded
UI assets through typed contracts.

The standard build always contains Docker Sandboxes, Containers, Local Ports,
Remote HTTP, Ports over SSH, Ports over Relay and Built-in Relay. Build tags and
`zelport_core` exist for tests, diagnostics and dependency-boundary verification;
they do not define user-facing editions.

## Packages and ownership

- `cmd/zelport` owns process startup, flags, listener binding and shutdown.
- `internal/app` owns schema-v1 configuration, revisions, routes and leases.
- `internal/feature` defines contracts shared by core and modules.
- `internal/modulekit` provides common lifecycle and reverse-proxy helpers.
- `internal/modules/<id>` owns one module's settings, routes, APIs and assets.
- `internal/web` owns protected administration and the browser shell.
- `internal/containers`, `internal/sbx` and `internal/sshpipe` isolate external
  tool adapters.
- `internal/portorigin` owns the feature-neutral, cached view used to attribute
  host listeners without importing one feature module from another.
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

Local, remote, container, sandbox and SSH sources adapt external services to the
same route/lease model. Adapters report tool and discovery failures without
silently deleting the last usable inventory. External tools remain optional
runtime dependencies of their respective sources and are never bundled.

Local Ports reads container and sandbox attribution from an asynchronously
refreshed shared snapshot; an administration request never starts or waits for
an external runtime CLI. A listener is classified as a local process only after
a complete successful discovery finds no matching published mapping. Positive
last-known mappings retain their kind while stale, whereas incomplete negative
evidence is reported as unknown. Address family, bind scope and protocol are
part of correlation, and competing runtime identities remain ambiguous rather
than being guessed.

Ports over Relay owns client identity, encrypted catalogs, peer sessions,
imports and remote administration. Built-in Relay owns relay listeners and
admission. They share the public SDK/server implementations but retain separate
configuration and lifecycle boundaries.

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

## Verification

`make check` runs Go tests, race detection, vet, release/packaging tests and the
browser script suites. `make check-modules` verifies the complete build and
diagnostic boundary combinations. Cross-component relay tests live under
`internal/relay/integration`; native platform acceptance remains a separate
release prerequisite where documented.
