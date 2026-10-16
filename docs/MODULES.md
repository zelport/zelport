# Compile-time feature modules

**Status: implemented for six source/import modules and optional relay hosting.**
Use this boundary when extending [Ports over Relay](../PORTS_OVER_CLOUD.md).
Exact Go, persistence, lease and frontend interfaces are specified in
[MODULE_CONTRACTS.md](MODULE_CONTRACTS.md). These contracts are normative for the
implementation plans.
The [modularization plan](superpowers/plans/2026-10-11-modularization.md) defines the work.

## Build behavior

The common core is always present. An untagged build contains no service modules,
including Local Ports. There is no `core`, `minimal`, `custom`, or `all` tag.
Modules are selected only through explicit descriptive Go build tags.

| Feature | Build tag | Module directory |
| --- | --- | --- |
| Docker Sandboxes | `docker_sandboxes` | `internal/modules/docker_sandboxes` |
| Containers (Docker, Podman and experimental nerdctl) | `containers` | `internal/modules/containers` |
| Local Ports | `local_ports` | `internal/modules/local_ports` |
| Remote HTTP | `remote_http` | `internal/modules/remote_http` |
| Ports over SSH | `ports_over_ssh` | `internal/modules/ports_over_ssh` |
| Ports over Relay | `ports_over_relay` | `internal/modules/ports_over_relay` |
| Relay hosting | `built_in_relay` | `internal/modules/built_in_relay` |

```sh
# Core only: administration without active service sources.
go build -o bin/zelport-core ./cmd/zelport

# Explicit feature selection.
go build -tags 'local_ports,remote_http' -o bin/zelport ./cmd/zelport

# Full distribution: enumerates implemented feature tags in the Makefile.
make build-full
```

`make build-full` includes all seven implemented features, including
`ports_over_relay` and `built_in_relay`. It must not advertise an absent module.
Default release artifacts use this full build. `make build-core` produces
`bin/zelport-core`; `make build` builds the full gateway.
The staging Makefile additionally verifies the sibling relay and SDK. Bare Go commands remain core-only.
The standalone relay command does not inherit Zelport feature selection. Embedded
relay hosting is independently selected with `built_in_relay`.

Go module requirements remain shared source-level metadata. Excluding a module
must exclude its packages, implementation, API, UI assets, and feature dependencies
from the Zelport binary dependency graph; this does not require splitting go.mod
or hiding dependencies from the repository's go.sum.

## Configuration with absent modules

A build starts with configuration for modules it does not include. Their settings
and routes remain preserved when unrelated configuration is saved. They are not
initialized, proxied, advertised or exported. Their route IDs and domains remain
reserved; requests to these domains return HTTP 503.

The Dashboard reports these routes as `unsupported`, disables opening and editing
them, and lists absent configured modules. The desktop integration API also reports
these routes as `unsupported`, without exposing opaque module settings. Zelport Integration
keeps Zelport available and prevents launching these inactive services. Missing
relay support does not imply a disconnected gateway.

Common configuration validation still applies, including the schema version,
module identifiers, route IDs and domains. Included modules validate their own
settings normally. Absent-module settings are validated when a later build includes
the module; valid preserved routes then become active again. This behavior does not
suppress malformed JSON, unsupported schema versions or errors in included modules.

## Core responsibilities

- CLI startup, version, loopback listeners, shutdown and runtime information.
- Exact localhost administration authority, Origin/CSRF protection and safe errors.
- Atomic versioned configuration storage and globally unique domains/UUID routes.
- Module registration and lifecycle orchestration.
- Domain lookup, common HTTP proxy helpers, WebSocket and streaming infrastructure.
- Administration shell, Dashboard, Settings, shared CSS/components and selected-module list.

Core provides facilities but no Sandbox, Container, Local, Remote, SSH or Cloud
service implementation. No feature-specific fields or imports in core models.
No dynamic Go plugins or runtime loading of modules excluded at compilation.

## Dependency and registration boundary

Use `internal/feature` for module contracts and descriptors; implement the registry
in `internal/app`. It depends
only on the standard library; modules depend on it, not the reverse. `internal/app` and `internal/web` are core consumers of this registry.

Create one tagged file per feature under `cmd/zelport`, each containing its exact
`//go:build` constraint and registering that module's factory in the composition
root. Do not use a core registry that imports every feature. Registry construction returns
errors for invalid/duplicate descriptors; installation order is deterministic.

A module owns typed decoding/validation of its configuration, inventory/state,
refresh lifecycle, route handler, optional API handlers, and frontend assets. See the contract for typed state,
revision checks, scoped settings and connection ownership.
Existing CLI adapters may remain in `internal/sbx`, `internal/containers`, and
`internal/sshpipe` initially, but only the corresponding module imports them.
Tag-specific main files wire factories; core startup has no per-feature branches.

Use a core route envelope: UUID `id`, `module`, normalized `domain`, and opaque
JSON `settings`. Configuration additionally has module-keyed settings documents.
The owning module validates settings before an atomic core save. No legacy schema
migration or compatibility layer is required. Unknown module configuration is an
explicit load error naming the required build tag, never silently discarded.
Globally unique domains apply across all included modules.

Expose immutable route snapshots and commit notifications through the registry.
Callbacks run after persistence and outside storage locks. Modules reconcile
concurrent edits and own cancellation of active/hijacked connections. Failed
initialization closes already-started modules; shutdown closes all modules.
No browser tab needs to be open for a module's background work to run.

## Administration and frontend

Core serves only registered descriptors and module assets. Modules mount admin
endpoints under `/api/modules/<module-id>/` through core middleware; they cannot
bypass exact Host, Origin or CSRF checks. Existing common route CRUD endpoints
use registry validation. Project domains retain their own `/api` namespace.

Keep feature markup/JavaScript/CSS in each module's embedded `assets` directory.
The core HTML contains only the shell and Dashboard, not hidden feature forms.
Use same-origin JavaScript modules loaded from registered asset URLs; no CDN,
frontend build step, or inline eval. Core supplies reusable DOM/API helpers.
Each module adds its tab, editor and inventory view, or contributes a Settings
section through `UIPlacement: "settings"` with an empty TabID. Dashboard renders a generic
summary supplied by modules and invokes their editor hook.

Retain URL tab selection, history navigation, purple light/dark styling, operation
indicators, and current refresh semantics. Missing/unknown tab hashes resolve to
Dashboard. State includes compiled module IDs so the UI and runtime details can
show actual capabilities. Missing modules expose neither endpoints nor assets.

## Cloud integration boundary

Ports over Relay is a normal optional module. It subscribes to generic committed
route snapshots and dispatches by stable route UUID through core's registered
service interface. It does not import Sandbox, Container, Local, Remote or SSH
modules. Own routes expose an exportable capability; cloud imports are always
non-exportable and administration is never a registered service.

Core handles generic route ownership, cancellation and service dispatch; the cloud
module owns authentication, catalog, credentials, tunnels, Origin settings and
trace verification. Core may carry generic request context/middleware hooks but
must not import cloud protocol or cryptographic types. Cloud settings remain an
opaque module document persisted atomically with the route envelopes.

Relay-host-only builds include neither the client nor yamux. Builds with neither
`built_in_relay` nor `ports_over_relay` include no cloud transport dependencies.

Cloud-only builds can import services even though there are no own services to
publish. Adding any source module enables publication through the same registry.
The provider dispatch boundary must retain cloud's authorization/withdrawal gate
and account for active WebSockets independently of the common route handler.

## Validation

Test core-only, each individual implemented module, all existing modules together,
and selected combinations. Also test cloud-only and cloud
with each source module as well as full builds. Inspect `go list -deps` for
`./cmd/zelport` in each build: absent features and their specific adapters/third-party
libraries must not enter the binary graph. Root `go test ./...` can still discover
module packages; this is distinct from an untagged Zelport binary importing them.

Check the embedded asset registry and actual HTTP endpoints, not just tab hiding.
Core-only startup must work without Docker, Podman, sbx or SSH installed. Regression
tests must preserve existing full-build HTTP, WebSocket, streaming, discovery,
security, route editing and cancellation behavior using fresh isolated configs.

## Module maturity and navigation

Descriptors must declare `stable` or `experimental` maturity. Docker Sandboxes,
Local Ports and Remote HTTP are stable; Containers, Ports over SSH, Ports over
Relay and Relay hosting are experimental.
Only stable standalone module tabs are available by default. The Settings
**Module UI availability** accordion stores user choices without stopping backends.
Configured services and module-specific guards prevent hiding a used interface.
Relay hosting has its own Settings accordion and functional enable setting.
See MODULE_CONTRACTS.md for the availability API and pure guard contract.
