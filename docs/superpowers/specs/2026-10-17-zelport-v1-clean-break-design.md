# Zelport v1 clean-break design

## Purpose

Establish Zelport as a clean product and protocol namespace with no compatibility
layer for pre-Zelport clients. The resulting source trees, generated artifacts,
documentation, product metadata and supported binaries must contain only Zelport
names. Git history is explicitly outside this change and will be handled before
the v1 product release.

The product release family is `v0.10.0`. The relay wire protocol starts at
version `1` and uses the `zelport-relay/v1` cryptographic namespace.

## Scope

This change coordinates the sibling repositories in the local Zelport workspace:

- `zelport-sdk-go` owns the relay wire version and cryptographic contexts.
- `zelport-relay` consumes SDK v0.10.0 and accepts only wire version 1.
- `zelport` consumes SDK and relay v0.10.0 and persists only version-1 relay
  records and digest watermarks.
- `workspace` coordinates verification and prevents the retired brand from being
  reintroduced.
- `zelport-omarchy`, the installer, distribution metadata and active
  documentation use product version 0.10.0 without preview labels.

The identity kit and website have no independent runtime version unless an
existing package manifest requires one.
External dependency versions, Go language versions and unrelated configuration
schema versions do not change.

The two unused hypothetical top-level project scaffolds are removed from the
workspace. No surviving source, documentation, build configuration or project
index may name them, reference them or anticipate their APIs. A future hosted
product would require a new design and repository decision from first
principles; this codebase reserves no boundary for it.

## Product terminology and composition

**Zelport** is the product family. **Zelport App** is the main locally running
application distributed as the `zelport` executable and package. Zelport App is
a local HTTP reverse proxy application with integrated service-source modules
and Built-in Relay.

**Zelport Relay** is the standalone relay server distributed as the
`zelport-relay` executable. **Ports over Relay** is one of Zelport App's
service-source modules and acts as a relay client. **Built-in Relay** is the
relay server capability inside Zelport App. The two relay capabilities are
independent and share protocol and server implementations with Zelport Relay
where appropriate.

Zelport App's modules are internal architectural boundaries for source
ownership, dependency isolation, lifecycle management and verification. They are
not product editions and are not presented as a user-selectable build system.
The standard distributed Zelport App contains every implemented feature module:

- Docker Sandboxes;
- Containers;
- Local Ports;
- Remote HTTP;
- Ports over SSH;
- Ports over Relay;
- Built-in Relay.

Build tags and the `zelport-core` executable exist only for development,
dependency-boundary checks, tests and diagnostics. They are not separately
marketed or distributed user products. User-facing build and installation paths
produce the complete Zelport App.

The retired architectural term for the whole application or listener is removed
from source, documentation, UI, tests and identifiers. Public text uses
**Zelport App**, **Zelport instance**, **Zelport service**, **reverse proxy**,
**listener**, **Localhost port** or **endpoint**, according to meaning. Internal
APIs use precise resulting names including `ListenerAuthority`,
`ProxyOptions.ListenerAuthority` and `SelfTarget` listener parameters. The UI
endpoint element uses `endpoint`, and the Omarchy setup document is named
`ZELPORT_SETUP.md`.

## Domain terminology

Every implemented capability related to `ports_over_relay`, relay admission,
relay transport, encrypted relay catalogs, peer traversal or relay-based remote
administration uses the term **relay** exclusively. This applies to source paths,
Go package names and aliases, functions, tests, errors, HTTP headers, scripts,
documentation, CSS classes, JavaScript identifiers, tab IDs and URL fragments.

The resulting relay-domain names include:

- `PORTS_OVER_RELAY.md`;
- `docs/ports-over-relay-v1.md`;
- `docs/ports-over-relay-verification.md`;
- `internal/relay/integration`;
- `scripts/relay-demo.py`;
- the `#relay` module tab and relay-named UI identifiers;
- `X-Zelport-Relay-Trace`.

There are no aliases or redirects for the retired names. Unrelated external
proper nouns retain their real names.

## Protocol identity

`protocol.Version` becomes `1`. Every encoded or persisted relay protocol value
that currently identifies wire version 3 becomes version 1, including:

- relay routes;
- catalog records and envelopes;
- peer contexts;
- authenticated catalog data;
- accepted-record validation;
- persisted digest watermark versions.

The SDK centralizes the protocol identity so that the wire version and
cryptographic contexts cannot drift. The exact contexts are:

- `zelport-relay/v1`
- `zelport-relay/v1/catalog`
- `zelport-relay/v1/client-relay/`
- `zelport-relay/v1/client-relay/open/`
- `zelport-relay/v1/consumer-provider/`
- `zelport-relay/v1/renewal/`

There is no fallback, negotiation, alias, migration or diagnostic branch for the
retired protocol. Version-3 inputs fail ordinary version validation. Existing
relay credentials, encrypted catalogs, sessions and persisted watermarks are not
compatible with the new protocol and must be recreated.

## Product versions

All released Zelport components move to the stable `v0.10.0` family:

- SDK tag: `v0.10.0`
- relay tag: `v0.10.0`
- Zelport App tag: `v0.10.0`
- Omarchy integration tag and manifest: `v0.10.0`
- installer version: `0.10.0`
- Homebrew and installation release references: `0.10.0`

Product and API documentation must no longer describe the release as preview.
Publishing is not part of this implementation. Remote tags, releases, visibility
changes and uploads require a separate explicit release operation.

Release publication must follow dependency order: SDK, relay, Zelport App, Omarchy
integration, then installer, Homebrew and website distribution references.
Generated release URLs and checksums must not be fabricated before their exact
artifacts exist. Source preparation may record the target version, but active
distribution files must remain internally valid at every committed step.

## Documentation and artifact cleanup

Historical implementation plans, specifications, verification reports,
benchmarks and captured profiler output that contain the retired product identity
are removed rather than rewritten. Rewriting captured output would make it
historically inaccurate. Current protocol, operation and upgrade documentation is
rewritten for Zelport v1 and v0.10.0.

All ignored or untracked build directories, staging proxies, archives and
binaries that may contain the retired identity are removed before verification.
Old executables are not renamed because their embedded strings and behavior would
remain stale. Supported executables are rebuilt from current source as:

- `zelport`
- `zelport-core`
- `zelport-relay`

Historical PoC, UI-test and one-off diagnostic executables without current build
targets are deleted and not replaced. Platform release archives retain the
project's existing `zelport_<version>_<os>_<arch>` naming convention.

## Regression guard

The workspace gains a brand-cleanliness check that scans sibling working trees
outside `.git`. It checks file paths and file bytes, including generated binaries
and archives selected for release. The forbidden token is assembled from parts so
the check does not contain or reintroduce the token itself.

The check is included in the shared verification target and reports every
offending path. Git object storage and history are excluded because history
rewriting is deferred to the separate pre-v1-release operation.

## Test strategy

Implementation follows test-driven development:

1. Change focused relay protocol tests to require version 1 and the new
   cryptographic contexts, and confirm that they fail against the current
   implementation.
2. Centralize and update the protocol identity with the minimum code needed to
   pass those tests.
3. Update relay and Zelport App tests to encode, accept and persist version 1, while
   proving that version 3 is rejected without migration.
4. Rename every implemented relay-domain path and identifier to relay
   terminology, and verify that routing, UI navigation and trace validation use
   only relay terminology.
5. Add and exercise the workspace brand-and-domain-cleanliness guard.
6. Remove historical material and generated artifacts, then rebuild only the
   supported binaries.
7. Run SDK, relay and Zelport App race tests and vet; run the relay integration
   harness, feature-module boundary matrix, JavaScript tests, packaging checks and
   supported builds.
8. Scan the final working trees and rebuilt binaries for the retired identity,
   removed project references and incorrect terminology in the relay domain.

The existing unrelated Zelport App working-tree changes must be preserved. No task
may discard, reset or overwrite them.

## Completion criteria

The change is complete when:

- current working trees outside `.git` contain no retired product token in paths
  or bytes;
- protocol version 1 and the `zelport-relay/v1` namespace are used consistently;
- version-3 protocol inputs are rejected and no compatibility code exists;
- implemented relay functionality uses relay-domain names exclusively;
- product text and identifiers consistently distinguish Zelport App, its reverse
  proxy role, Ports over Relay, Built-in Relay and standalone Zelport Relay;
- the retired whole-application architectural term is absent from current paths
  and file bytes;
- standard user builds contain every implemented module, while partial builds
  remain development and verification tools only;
- the unused hypothetical project scaffolds and every reference to them are
  absent from the workspace;
- active product metadata names v0.10.0 without preview qualifiers;
- supported binaries have Zelport names and contain no retired product identity;
- the full coordinated verification suite passes, or every pre-existing or
  environment-specific failure is reported precisely;
- no remote tag, release or artifact has been published by this work.
