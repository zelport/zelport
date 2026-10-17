# Zelport App modules

Modules are internal architectural boundaries for source ownership, dependency
isolation, lifecycle and verification. They are not user-selectable product
editions. Every standard build and distribution of Zelport App contains all
seven implemented modules.

| Capability | Module ID | Diagnostic build tag | External tool |
| --- | --- | --- | --- |
| Docker Sandboxes | `docker_sandboxes` | `docker_sandboxes` | `sbx` |
| Containers | `containers` | `containers` | Docker or Podman; optional `nerdctl` |
| Local Ports | `local_ports` | `local_ports` | none |
| Remote HTTP | `remote_http` | `remote_http` | none |
| Ports over SSH | `ports_over_ssh` | `ports_over_ssh` | system SSH |
| Ports over Relay | `ports_over_relay` | `ports_over_relay` | reachable relay |
| Built-in Relay | `built_in_relay` | `built_in_relay` | none |

External tools are supported, not bundled. Their absence affects only the
source that uses them and must be reported as actionable runtime status.

## Build behavior

```sh
make build          # complete Zelport App
make build-core     # diagnostic core-only executable
make check-modules  # complete build plus boundary combinations
```

Ordinary `go build ./cmd/zelport` also selects the complete composition.
`zelport_core` explicitly selects diagnostic core, and individual module tags
may be combined with it for boundary tests. Release builds never use
`zelport_core`.

An excluded module contributes no implementation, API registration, assets or
adapter dependencies to that diagnostic binary. A configuration referencing an
excluded module is preserved but cannot run; standard user builds do not expose
that state.

## Core responsibilities

Core owns configuration schema v1, revisions, route uniqueness, listener
authority, administration security, immutable route snapshots, leases and
shutdown. It does not know module-specific settings or backend semantics.

Each module owns:

- its descriptor, maturity and placement;
- settings and route payload validation;
- prepare/activate/abort/close lifecycle;
- request dispatch and resource registration;
- module API and embedded UI fragments;
- summaries and status exposed through shared models.

The detailed interfaces and ordering rules are in
[MODULE_CONTRACTS.md](MODULE_CONTRACTS.md).

## UI composition

The browser shell renders shared navigation, status, dialogs and service lists.
Modules contribute declarative descriptors and namespaced assets. IDs, event
names and CSS selectors stay module-owned to prevent coupling. A module status
must distinguish configuration, external-tool availability, discovery health
and service health rather than collapsing them into one boolean.

## Relay boundaries

Ports over Relay and Built-in Relay are separate modules. The client module owns
groups, identities, encrypted catalogs, imports and peer sessions. Built-in
Relay owns relay listener/admission settings. Enabling either module never
implicitly enables the other.

Both depend on released `zelport-sdk-go`/`zelport-relay` modules without local
source replacements. Cross-component behavior is verified under
`internal/relay/integration`.
