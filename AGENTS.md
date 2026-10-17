# Working on Zelport App

Read [README.md](README.md) before making changes. Write documentation, code
comments, UI text, and error messages in English. Record deferred work in
[TODO.md](TODO.md); a TODO entry does not authorize implementation.

Use [DESIGN.md](DESIGN.md) for the implemented architecture,
[PORTS_OVER_RELAY.md](PORTS_OVER_RELAY.md) for Ports over Relay requirements,
and [docs/LOCAL_INTEGRATION_API.md](docs/LOCAL_INTEGRATION_API.md) for the local
read-only integration contract. A proposal is not evidence of implemented
behavior.

## Product and terminology

Zelport App is a local HTTP reverse proxy application with a web interface on
localhost. Its standard build includes all seven internal modules: Docker
Sandboxes, Containers, Local Ports, Remote HTTP, Ports over SSH, Ports over
Relay, and Built-in Relay. Modules are architectural boundaries, not product
variants or user-selected build options.

Zelport Relay is the standalone relay server. Ports over Relay is a service
source in Zelport App. Built-in Relay is the relay server embedded in Zelport
App. Use these names consistently in code, documentation, tests, and UI.

The relay protocol has wire version 1, the exact WebSocket endpoint `/v1/ws`,
and cryptographic domain labels under `zelport-relay/v1/*`. The configuration
schema version is 1. Unsupported versions are rejected. No compatibility layer,
migration, or reuse of credentials from pre-Zelport formats is required.

## Repository structure

- `cmd/zelport`: CLI flags, module factories, startup, listeners, and shutdown.
- `internal/app`: route configuration, persistence, registry, and ownership.
- `internal/feature`: module contracts, request leases, and HTTP helpers.
- `internal/modulekit`: shared lifecycle and reverse-proxy helpers.
- `internal/modules/<id>`: service sources, relay capability, APIs, and assets.
- `internal/sbx`: Docker Sandbox CLI adapter and JSON fixtures.
- `internal/containers`: Docker/Podman discovery and cached inventories.
- `internal/web`: protected administration API, runtime information, and UI.
- `internal/web/assets`: embedded HTML, CSS, and JavaScript; no build step/CDN.
- `internal/sshpipe`: system SSH forwarding through stdin/stdout as `net.Conn`.
- Sibling `zelport-sdk-go`: relay protocol, secure channels, and client.
- Sibling `zelport-relay`: standalone relay server and CLI.

There is no shared `go.work`. Repository targets use `GOWORK=off`. Do not add
local `replace` directives or dependencies on hypothetical external products.

## Runtime invariants

- Keep listeners on loopback. Administration belongs only to the exact
  `localhost` authority for the configured port.
- Preserve mutation protection through exact Origin checks and the per-process
  CSRF token.
- Persist configuration atomically with owner-only permissions. Preserve valid
  configuration on validation or write failures.
- Preserve stable route and provider identities. Do not reconnect a missing
  source merely because a new one has the same name.
- Keep last-known usable routes during discovery failures and distinguish stale
  state from confirmed removal.
- Preserve WebSocket upgrades, streaming, and upstream response statuses.
- Keep remote TLS verification enabled by default; Origin rewriting is opt-in.
- Own and close SSH and relay connections, including upgraded connections.
- Preserve user configuration and services. Tests use temporary configuration
  and controlled backends.
- Preserve relay admission/E2E-secret separation, provider pins, durable catalog
  watermarks, and synchronous sharing revocation.

## UI and module boundaries

Follow [docs/MODULES.md](docs/MODULES.md) and
[docs/MODULE_CONTRACTS.md](docs/MODULE_CONTRACTS.md). Keep module APIs,
settings, assets, dependencies, and lifecycle behind their boundaries while
shipping every standard module in the application build.

When fixing shared UI behavior, audit every affected tab, module-owned section,
and dialog. Polling should retain DOM nodes when displayed content is unchanged
and preserve selection, focus, scroll, expanded sections, and form drafts.
Verify narrow and short viewports as well as unchanged polls and real updates.

The Zelport runtime panel uses Version, Configuration, and Listeners on its first
row and a full-width Modules field on its second row. Do not show build time or
uptime there.

## Verification

Run relevant isolated tests first, then the repository checks:

```sh
go test -race ./...
go vet ./...
make build-full
```

Use `make -C ../workspace check`, `make -C ../workspace check-modules`, and
`make -C ../workspace build` for cross-repository checks and the module matrix.
Build outputs belong in `bin/` and must not be committed.

Run JavaScript and frontend tooling only through `sbx` (Docker Sandbox), never
with Node.js on the host. Verify frontend changes in a browser, including
navigation, forms, polling, theme, and supported viewport sizes.

## Commits and publication

Use Conventional Commits in English: `<type>[optional scope][!]: <description>`.
Keep commits focused and mark breaking changes with `!` or a `BREAKING CHANGE:`
footer. Do not rewrite shared history as part of routine work. The planned clean
history for the first `v1.0.0` publication is a separately authorized release
operation.

Do not commit binaries, archives, local staging caches, credentials, or runtime
configuration. Use synthetic public examples and reserved domains. Review
staged changes before committing.

Use Git author and committer `zdeneksvarc` with
`79550344+zdeneksvarc@users.noreply.github.com`; do not inherit a personal
identity from global Git configuration.

The installer source of truth is the sibling `zelport-installer` repository.
Its verified release workflow synchronizes the generated website copy. Never
edit `zelport-web/install.sh` manually.

## Platform boundaries

Keep shared behavior independent of platform-specific filesystem locations,
permissions, signals, service managers, Unix sockets, and shells. Put native
behavior behind explicit OS boundaries or build-tagged files. Cross-compilation
is not runtime validation.
