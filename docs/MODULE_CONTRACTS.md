# Module contracts v1

This document defines the contracts between Zelport App core and feature
modules. It complements [DESIGN.md](../DESIGN.md) and is normative for module
lifecycle, request ownership and administration integration.

## Descriptor and registration

Every module exposes one stable descriptor containing its ID, display name,
placement, maturity and UI metadata. IDs are unique and use lowercase
snake_case. Registration order is deterministic; duplicate IDs, invalid
placement or invalid maturity fail startup.

The standard build registers five modules. `zelport_core` builds may register a
subset for dependency-boundary verification. Core contracts do not change with
composition.

## Configuration ownership

Core owns the schema-v1 envelope:

- `version` is exactly `1`;
- `revision` protects concurrent writes;
- `modules` maps module IDs to raw settings;
- `routes` stores generic identity, module, domain and raw settings.

Unknown top-level fields and unsupported versions are errors. Each module
strictly decodes and validates its own settings and route payloads. Modules do
not read or write the configuration file directly. Every settings key and route
owner must name a module registered in the running build.

## Commit lifecycle

A state change follows this order:

1. validate the complete candidate state;
2. ask affected modules to prepare resources without exposing them;
3. persist the candidate atomically with owner-only permissions;
4. activate prepared module state;
5. publish the new immutable route snapshot;
6. revoke replaced leases and close retired resources.

Any error before persistence aborts prepared work and leaves the live state
unchanged. A metadata-only module commit may advance protocol state without
changing the edit revision.

## Routes and leases

Core normalizes `*.localhost` domains, enforces uniqueness and resolves one
route snapshot per request. Dispatch acquires a generation-bound lease before
calling a module. Modules register requests, streams, connections, processes and
other closeable work with that lease.

Route replacement/removal, module disablement and App shutdown revoke the lease.
Registration after revocation must fail and close the submitted resource.
Modules must not retain untracked work beyond lease lifetime.

## HTTP dispatch

Modules receive a normalized request plus route-owned settings. They preserve
streaming, cancellation, WebSocket upgrades and application status codes unless
their documented source semantics require a transformation. Shared reverse
proxy helpers apply consistent headers, flushing and error mapping.

Backends are checked against the active listener to prevent direct self-targets.
Module-to-module dispatch uses the typed host contract and never calls the public
listener.

## Administration API

Module endpoints are registered under module-owned paths and inherit exact local
authority, Origin and CSRF enforcement from core. Read endpoints must return
stable typed models. Mutation endpoints accept the current edit revision and
must use the commit lifecycle above.

Remote relay administration is a separate typed capability. It is granted only
after peer authentication and reserved-domain validation; request headers cannot
grant it.

The read-only local integration API is owned by core and documented in
[LOCAL_INTEGRATION_API.md](LOCAL_INTEGRATION_API.md). Modules contribute data to
that response but cannot make it writable.

## Frontend contract

Module HTML, CSS and JavaScript are embedded at compile time and namespaced by
module ID. Core owns shared elements, dialog behavior, focus management,
notifications and component helpers. Modules use semantic status values and do
not depend on another module's private markup.

Refreshes preserve active focus, selections and open controls when their backing
identity still exists. Destructive actions require explicit confirmation and
report completion or failure accessibly.

## External adapters

CLI and engine adapters have bounded execution, deterministic parsing and
synthetic fixtures. Metadata availability, runtime connectivity and discovery
are separate states. A transient outage keeps last-known usable inventory where
the module contract permits and visibly marks it stale.

System SSH is a supported third-party tool, not a bundled dependency. Core must
remain independently testable without it.

## Shutdown

Shutdown stops new dispatch, revokes all leases, closes module resources and
then closes listeners. Module `Close` methods are idempotent and bounded. Tests
cover repeated close, cancellation, failed preparation and partial external-tool
failure.
