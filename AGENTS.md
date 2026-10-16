# Working on Zelport

## Temporary commit message override

Until the user explicitly revokes this instruction, use exactly
`chore: initialize preview` for every new commit, regardless of change type.
This temporary instruction takes precedence over the general commit message
rules below. Do not rewrite existing commits solely to apply this instruction.

## Language and references

Record user-discussed ideas and deferred improvements in [TODO.md](TODO.md).
Include the context, intended benefit and unresolved decisions. A TODO entry
does not authorize implementation. Update or remove entries when resolved.

Write documentation, code comments, UI text, and error messages in English.
Use [README.md](README.md) for operation and development commands,
[DESIGN.md](DESIGN.md) for the implemented architecture, and
[PORTS_OVER_CLOUD.md](PORTS_OVER_CLOUD.md) for cloud requirements and security boundaries.
A proposal is not evidence of implemented behavior.

## Repository structure

- `cmd/zelport`: CLI flags, tagged module factories, startup, listeners, and shutdown.
- `internal/app`: generic route configuration, persistence, registry, and resource ownership.
- `internal/feature`: module contracts, request leases, and HTTP helpers.
- `internal/modulekit`: shared module lifecycle and proxy helpers.
- `internal/modules/<id>`: source implementations, reconciliation, APIs, and embedded frontend assets.
- `internal/sbx`: Docker Sandbox CLI adapter and JSON fixtures.
- `internal/containers`: Docker/Podman discovery and cached inventories.
- `internal/web`: protected administration API, runtime information, and the generic frontend shell.
- `internal/web/assets`: embedded HTML, CSS, and JavaScript; no frontend build or CDN.
- `internal/sshpipe`: system SSH client forwarding through stdin/stdout as `net.Conn`.
- `internal/cloud/integration`: cross-component integration tests.
- Sibling `zelport-sdk-go`: protocol, Noise channels, WebSocket transport and client.
- Sibling `zelport-relay`: server and self-hosted CLI; never receives the E2E secret.

- `../../experiments/ports-over-cloud`: isolated Go module for the cloud PoC; with
  experimental relay/peer commands, transport tests, and a temporary-state demo.
- `Makefile`: gateway checks and builds.

Core and the five original source modules primarily use the Go standard library.
Windows platform adapters additionally use golang.org/x/sys/windows for native
paths, ACLs and file replacement. Cloud
transport adds pinned websocket, Noise and yamux dependencies to the root module;
feature-exclusion checks must keep them out of builds without cloud. Keep the
experimental module independently pinned. Do not add cross-module
imports or `replace` directives to couple the experiment to production code.
There is no shared `go.work`; repository targets use `GOWORK=off` for isolation.

## Invariants

- Keep the gateway listeners on loopback. Administration belongs only to the exact
  `localhost` authority for the configured port. The authenticated cloud module may
  call the typed administration bridge for reserved `*.zelport.localhost` aliases;
  never infer this permission from headers. Project paths belong to applications.
- Preserve mutation protection through exact Origin checks and the per-process CSRF token.
- Persist configuration atomically with owner-only file permissions. Preserve existing
  configuration on validation or write failures, and reject unsupported configuration versions explicitly. Configuration v6
  intentionally breaks the old schema; no legacy compatibility or migration is required.
- Preserve stable route and provider identities. Never reconnect a missing sandbox or
  container solely because a new one has the same name.
- Keep last-known usable routes during discovery failures; distinguish stale state from
  confirmed removal. Configuration and port mappings do not prove application health.
- Preserve WebSocket upgrades, streaming, and application response statuses.
- Keep remote TLS verification enabled by default and Origin rewriting explicitly opt-in.
- Manage the lifetime of owned SSH connections/processes, including upgraded connections.
- Preserve the user's configuration and services. Use temporary configurations and
  controlled backends for tests, not the running user's routes.

## Terminology

- **Sandbox port**: application port inside a Docker Sandbox.
- **Aux port**: intermediate host port used as a proxy upstream.
- **Localhost port**: Zelport's gateway/administration port, default `8080`.

Use **Container port** for ordinary Docker/Podman containers. SSH forwarding does
not create a local Aux port.

## Verification

For code changes, run the relevant checks and the project verification commands:

```sh
go test -race ./...
go vet ./...
go build -o bin/zelport ./cmd/zelport
```

Use `make -C ../workspace check`, `make -C ../workspace check-modules`, and
`make -C ../workspace build` to cover the active projects and compile-time feature matrix.
Dependencies now use public module versions. Direct repository commands work
without staging. The workspace wrapper preserves published pins. Bare `go build` produces core only; use
`make build-full` for the complete application. Gateway `go test ./...` does not discover sibling SDK/relay tests or the
separate historical experiment. See its README for the process-level demo and RESULTS.md for limits; passing
PoC tests is not a production security audit.

Build into `bin/`. Verify frontend changes in a browser, including affected
navigation and form behavior. For documentation-only changes, check accuracy,
English wording, and links; rebuilding the application is unnecessary.
Report which checks ran and distinguish isolated tests from live integration.
Optional live discovery checks and current integration limitations are documented
in [README.md](README.md).

## Compile-time modules

Follow [docs/MODULES.md](docs/MODULES.md) and
[docs/MODULE_CONTRACTS.md](docs/MODULE_CONTRACTS.md). Core is always built;
no feature is included without its explicit tag. Full release builds enumerate
all seven implemented feature tags. Keep feature APIs, settings, assets,
dependencies and lifecycle out of core. Ports over Relay (`ports_over_relay`) uses the same module boundary. Preserve admission/E2E secret
separation, provider pins, durable catalog watermarks and synchronous sharing
revocation. Mark only counter/watermark writes MetadataOnly; never use that flag
for effective settings, credentials, authorization or route changes.

## Shared UI fixes

When fixing UI behavior, audit every tab and shared component for the same
problem, including module-owned sections and dialogs. Polling must retain DOM
nodes when displayed content is unchanged. Update current state and revisions
independently of rendering; preserve selection, focus, scroll and form drafts.
Verify both unchanged polls and real state changes across affected views.

Settings cards must share full-width content alignment: align switches, field
rows and separators with the card content edges. Do not introduce module-specific
form width caps that break alignment with adjacent settings cards. Keep naturally
short controls such as ports compact within the shared row layout.

Audit modal changes across service editors, removal, relay settings and token
rotation at both narrow and short viewports. Picker menus must stay inside the
dialog layout; action rows must not cover fields or options. New mapping forms
must prompt for a choice, while editing preserves the saved selection. Display
operation errors inside the owning modal and reveal collapsed sections containing
invalid required fields.

## Commit messages

Use [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/)
for all new commits. Write commit messages in English.

Format: `<type>[optional scope][!]: <description>`.

Use `feat` for new features and `fix` for bug fixes. Use `docs`, `build`, `ci`,
`test`, `refactor`, `perf`, `style`, or `chore` where appropriate. Use lowercase
types and short, meaningful scopes. Keep each commit focused on one logical
change and write a concise description of what it does.

Mark breaking changes with `!` before the colon or a `BREAKING CHANGE:` footer.
Explain the impact and migration steps in the body when needed. Squash commit
titles must follow the same convention.

Examples:

- `feat(homebrew): support user services`
- `fix(relay): preserve client identities on reconnect`
- `docs: clarify installation instructions`
- `feat(config)!: require configuration version 6`

Do not rewrite existing shared history merely to change commit message format.

## Preparing changes for publication

Keep source repositories independent of the frozen source container. Do not
commit binaries, generated release archives, local staging caches, credentials,
or user runtime configuration. Review staged changes and run the repository's
relevant checks before committing. Preparation for a push does not authorize
changing repository visibility or publishing releases.

## Future Windows support

Develop Zelport with future Windows support in mind. Keep platform-independent
HTTP and application behavior separate from filesystem locations, permissions,
process signals and service-manager integration. Put platform-specific behavior
behind explicit OS boundaries or build-tagged files. Do not assume systemd, XDG,
Unix sockets, POSIX file modes or a Unix shell in shared code. Preserve Linux and
macOS behavior while making these boundaries clear.

Cross-compile the full gateway for Windows when changing shared startup or OS
integration. Cross-compilation is not runtime validation and does not establish
Windows support. Track unresolved native paths, persistence, process lifecycle
and backend availability in TODO.md before advertising Windows compatibility.

## Privacy in published material

Use synthetic examples in documentation, tests, screenshots and verification
reports. Use reserved example domains such as `desktop.example.test`, generic
paths such as `~/.config/zelport/`, and temporary test data. Never publish real
internal hostnames, personal home-directory paths, private email addresses,
credentials or user configuration. Check tracked files and staged diffs before
publication, including generated source bundles.

For this repository, use the GitHub account identifier `zdeneksvarc` as the Git
author and committer name, and `79550344+zdeneksvarc@users.noreply.github.com`
as the email address. Verify both identities before committing. Do not inherit
a personal identity from global Git configuration.

## Installer synchronization

The canonical installer is `install.sh` in `zelport/zelport`.
Whenever it changes, publish the identical file to `zelport/zelport-web`
in the same update. The website serves it at https://zelport.com/install.sh.
Run the gateway installer tests before publication and compare SHA-256 hashes
across both repositories. After deployment, verify that the public URL serves
the same bytes. Do not maintain a separate website-specific installer.

Increment `ZELPORT_INSTALLER_VERSION` for every published change to `install.sh`,
including release-pin changes. Use semantic versioning independently of the
application release: patch for fixes and pin updates, minor for compatible
features, major for breaking installer behavior. Keep the startup banner and
its test expectations current.
