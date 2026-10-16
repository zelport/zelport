# Modularization verification — 2026-10-11

## Delivered boundary

Core builds without service modules. Five positive build tags select Docker
Sandboxes, Containers, Local Ports, Remote HTTP and Ports over SSH. Their
implementation, assets and actions are owned by their module packages. Cloud
production integration is not part of this change.

The generic registry provides v6 persistence, UUID routes, global domain
uniqueness, revision checks, scoped commit notifications and revocable request
leases. Configuration v5 is intentionally unsupported; no user configuration was
modified or migrated. The user's running instance was not restarted.

## Commands completed successfully

- `make check-modules`: race tests, vet, build, version and dependency-graph checks
  for core, each of the five individual tags, Local+Remote, and the full build.
- `make check`: core/full race tests and vet, plus the isolated cloud experiment.
- `make build`: full Zelport and experimental relay/peer commands.
- `make build-core`: untagged core binary.
- Targeted race tests after final source/action and verification additions.

The matrix checks the command dependency graph; root package tests intentionally
exercise module packages even without command build tags. Excluded module APIs
and assets return 404. A core asset test checks that source-specific action text
and settings do not leak into the shared frontend.

## Behavioral coverage

Registry tests cover empty/duplicate modules, startup rollback, unsupported or
missing config versions, absent configured modules, atomic write failure,
persisted UUIDs, domain/revision conflicts, coalesced notifications, scope reset,
settings changes that preserve active streams, domain-rename admission races,
and waiting for active resource cleanup before acknowledging a commit.

Proxy integration tests cover application 403 responses, preserved path/query,
remote authority and Origin rewriting, TLS verification isolation, streaming,
WebSocket forwarding and connection closure on route deletion. Existing CLI
adapter and SSH process/pool suites remain isolated from real services.

Sandbox tests cover stopped saves, automatic reconciliation without duplicate
publication, explicit start and repeated start, stable identity, cached routing
during discovery errors, preserving ports after a post-publication read failure,
shared mapping protection, failed removal intent and successful retry. Container
tests verify exact binding selection and unambiguous same-address fallback.
Administration tests cover exact authority, Origin/CSRF, stale mutation revision,
module absence, sanitized private settings and stale/removed state filtering.

Review findings were reproduced and fixed: upgrade/close races, early cleanup
acknowledgments, ambiguous container fallback, lost cached ports, acceptance of
versionless existing configuration, and domain lookup/admission races.

## Browser evidence and limits

Chrome smoke checks used ports 18082/18083, temporary configurations and fake
sbx/Docker/Podman commands. Verified core-only Dashboard with unavailable-tab
fallback; all full-build tabs; Local and stopped Sandbox saves; Remote TLS/Origin
controls; SSH editor; source-owned action buttons; reload/Back tab preservation;
keyboard navigation; and no console errors. Light desktop styling was inspected.

No live Docker/Podman or remote SSH integration was performed for this refactor.
Dark mode and narrow-screen visual acceptance remain manual checks: the browser
viewport override did not change the observed viewport. Automated browser fault
injection for module-script load failures and slow/out-of-order state requests
has not been added. These are verification limits, not claims of completed tests.

## Implementation decisions

Lease mechanics live in `internal/feature`; the registry owns active lease sets
in `internal/app`. Shared lifecycle/proxy helpers live in `internal/modulekit`.
HTTP transports currently belong to individual requests with keepalive disabled,
so backend connections have clear revocation ownership. This costs extra TCP
connections; reusable pooling requires separate lease-safe design. SSH pools
retain process limits and backoff. No automatic HTTP request replay was added.

Source modules may inspect immutable snapshots/cached inventory while validating;
validators perform no external I/O or configuration writes. Module-specific action
errors currently use plain text, while core API errors include JSON error codes.
