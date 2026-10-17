# Zelport v1 clean-break design

## Purpose

Establish Zelport as a clean product and protocol namespace with no compatibility
layer for pre-Zelport clients. The resulting source trees, generated artifacts,
documentation, product metadata and supported binaries must contain only Zelport
names. Git history is explicitly outside this change and will be handled before
the v1 product release.

The product release family is `v0.10.0`. The cloud wire protocol starts at
version `1` and uses the `zelport-cloud/v1` cryptographic namespace.

## Scope

This change coordinates the sibling repositories in the local Zelport workspace:

- `zelport-sdk-go` owns the wire version and cryptographic contexts.
- `zelport-relay` consumes SDK v0.10.0 and accepts only wire version 1.
- `zelport` consumes SDK and relay v0.10.0 and persists only version-1 cloud
  records and digest watermarks.
- `workspace` coordinates verification and prevents the retired brand from being
  reintroduced.
- `zelport-omarchy`, the installer, distribution metadata and active
  documentation use product version 0.10.0 without preview labels.

The identity kit, website, cloud service scaffold and cloud SDK scaffold have no
independent runtime version unless an existing package manifest requires one.
External dependency versions, Go language versions and unrelated configuration
schema versions do not change.

## Protocol identity

`protocol.Version` becomes `1`. Every encoded or persisted cloud protocol value
that currently identifies wire version 3 becomes version 1, including:

- relay routes;
- catalog records and envelopes;
- peer contexts;
- authenticated catalog data;
- accepted-record validation;
- persisted digest watermark versions.

The SDK centralizes the protocol identity so that the wire version and
cryptographic contexts cannot drift. The exact contexts are:

- `zelport-cloud/v1`
- `zelport-cloud/v1/catalog`
- `zelport-cloud/v1/client-relay/`
- `zelport-cloud/v1/client-relay/open/`
- `zelport-cloud/v1/consumer-provider/`
- `zelport-cloud/v1/renewal/`

There is no fallback, negotiation, alias, migration or diagnostic branch for the
retired protocol. Version-3 inputs fail ordinary version validation. Existing
cloud credentials, encrypted catalogs, sessions and persisted watermarks are not
compatible with the new protocol and must be recreated.

## Product versions

All released Zelport components move to the stable `v0.10.0` family:

- SDK tag: `v0.10.0`
- relay tag: `v0.10.0`
- gateway tag: `v0.10.0`
- Omarchy integration tag and manifest: `v0.10.0`
- installer version: `0.10.0`
- Homebrew and installation release references: `0.10.0`

Product and API documentation must no longer describe the release as preview.
Publishing is not part of this implementation. Remote tags, releases, visibility
changes and uploads require a separate explicit release operation.

Release publication must follow dependency order: SDK, relay, gateway, Omarchy
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

1. Change focused protocol tests to require version 1 and the new cryptographic
   contexts, and confirm that they fail against the current implementation.
2. Centralize and update the protocol identity with the minimum code needed to
   pass those tests.
3. Update relay and gateway tests to encode, accept and persist version 1, while
   proving that version 3 is rejected without migration.
4. Add and exercise the workspace brand-cleanliness guard.
5. Remove historical material and generated artifacts, then rebuild only the
   supported binaries.
6. Run SDK, relay and gateway race tests and vet; run the relay integration
   harness, gateway feature-module matrix, JavaScript tests, packaging checks and
   supported builds.
7. Scan the final working trees and rebuilt binaries for the retired identity.

The existing unrelated gateway working-tree changes must be preserved. No task
may discard, reset or overwrite them.

## Completion criteria

The change is complete when:

- current working trees outside `.git` contain no retired product token in paths
  or bytes;
- protocol version 1 and the `zelport-cloud/v1` namespace are used consistently;
- version-3 protocol inputs are rejected and no compatibility code exists;
- active product metadata names v0.10.0 without preview qualifiers;
- supported binaries have Zelport names and contain no retired product identity;
- the full coordinated verification suite passes, or every pre-existing or
  environment-specific failure is reported precisely;
- no remote tag, release or artifact has been published by this work.
