# Native Release Binary Testing

**Date:** 2026-10-17

**Status:** Proposed

**Repository:** `zelport/zelport`

## Purpose

Every Zelport App archive must be executed and acceptance-tested on the operating
system and CPU architecture named by that archive before publication. A
successful cross-compilation is not runtime validation.

The release pipeline must publish the exact bytes that passed these tests. It
must use only free standard GitHub-hosted runners available to the public
repository and must not require self-hosted or larger runners.

## Supported release matrix

| Release platform | Archive | GitHub-hosted runner |
| --- | --- | --- |
| macOS ARM64 | `zelport_<version>_darwin_arm64.tar.gz` | `macos-15` |
| macOS x86-64 | `zelport_<version>_darwin_amd64.tar.gz` | `macos-15-intel` |
| Linux ARM64 | `zelport_<version>_linux_arm64.tar.gz` | `ubuntu-24.04-arm` |
| Linux x86-64 | `zelport_<version>_linux_amd64.tar.gz` | `ubuntu-24.04` |

Runner labels are pinned to explicit operating-system releases. The workflows
must not use `*-latest`, larger runners, or self-hosted machines.

## Pipeline architecture

The pipeline has four ordered stages:

1. **Source verification** runs the repository's existing checks, including Go
   tests with race detection, vetting, packaging tests, and module boundary
   builds.
2. **Release build** cross-compiles all four `CGO_ENABLED=0` binaries from one
   commit, packages them once, generates `SHA256SUMS`, and uploads one temporary
   workflow artifact containing the complete release bundle.
3. **Native acceptance** fans out into the four-runner matrix. Each job
   downloads the same workflow artifact, selects its platform archive, verifies
   it, extracts it, and tests the contained executable without rebuilding it.
4. **Publication** downloads that already-tested bundle and publishes its files
   unchanged. Publication is allowed only for a valid stable version tag and
   only after all four native jobs succeed.

The dependency order is therefore:

```text
source verification -> release build -> native acceptance matrix -> publication
```

No job after native acceptance may invoke `go build`, regenerate an archive, or
rewrite `SHA256SUMS`.

## Workflow events and versions

Pull requests and pushes to `main` run source verification, build all four
archives, and run the native matrix. Their unpublished test bundle uses the
neutral version `0.0.0`, already reserved by the Homebrew test process, and is
identified operationally by the workflow run and commit SHA.

A tag matching `v<major>.<minor>.<patch>` uses the numeric version without `v`
inside the binary and archive names. The release script continues to reject
prerelease labels, leading zeroes, path-like values, and other non-stable
versions. Only tag events enable the publication job.

The temporary workflow artifact has one-day retention. Published archives and
checksums live as immutable GitHub Release assets rather than long-lived Actions
artifacts.

## Native acceptance contract

A cross-platform acceptance harness receives the extracted release executable,
the expected numeric version, and the expected Go platform. It owns all child
processes and uses only temporary configuration and state directories.

For every matrix entry it must verify:

1. `SHA256SUMS` validates the selected archive.
2. The archive contains exactly `zelport`, `LICENSE`, and
   `THIRD_PARTY_NOTICES.txt` with the expected modes.
3. `zelport --version` exits successfully and reports the expected numeric
   version.
4. Zelport App starts as an ordinary user with a temporary `HOME`, configuration
   path, and unused loopback port.
5. `GET /api/state` on the exact localhost authority reports the expected
   version and `darwin/arm64`, `darwin/amd64`, `linux/arm64`, or `linux/amd64`.
   Its listeners include IPv4 loopback and also IPv6 loopback when the runner
   supports IPv6.
6. The standard build exposes exactly `docker_sandboxes`, `containers`,
   `local_ports`, `remote_http`, `ports_over_ssh`, `ports_over_relay`, and
   `built_in_relay`.
7. A temporary local HTTP backend is reachable through a configured Local Ports
   route, proving real reverse-proxy data flow through the release executable.
8. The read-only Local Integration endpoint returns product `zelport`, API
   version `1`, the expected App version, and the configured service.
9. The process accepts `SIGTERM`, exits successfully within the shutdown
   deadline, and emits no panic or fatal runtime failure.

The harness must produce useful diagnostics on failure, including the captured
Zelport log, while never printing configuration secrets or host-specific user
data.

## Platform-specific coverage

The native acceptance contract tests the Zelport App itself. Docker, Podman,
Docker Sandboxes, and system SSH are supported third-party tools, not runtime
requirements, so their absence must not fail this matrix. Existing unit and
module tests continue to cover their adapters with controlled fixtures.

Linux-only process tests may remain as deeper supplemental coverage. A later
workflow may add real external-tool integration, but it is not a publication
gate for this design.

Built-in Relay and Ports over Relay receive a small release-binary smoke test.
The harness starts two isolated Zelport App processes, establishes a local
Relay Protocol v1 path, confirms one end-to-end proxied request, and shuts both
processes down. This proves that the standard release contains the relay client
and server capabilities; protocol conformance remains in the SDK and Relay
repositories.

## Security and permissions

All verification jobs use read-only repository permissions. The publication
job alone receives `contents: write`. Actions are pinned to immutable commit
SHAs, checkout credentials are not persisted, and pull-request jobs receive no
publication credentials.

The workflow does not run repository code on any developer workstation. All
four native tests execute on disposable GitHub-hosted machines.

## Downstream distribution

The App release is the source of truth for downstream distribution. Homebrew
and `zelport-installer` tests run after a successfully published App release and
verify their own installation paths. They must consume the immutable App
release assets and must not rebuild the App.

Failure of a downstream installation test blocks that downstream update, not
the already-verified App artifact. No release URL may be overwritten to repair
a failure.

## Failure semantics

- Any source, build, checksum, archive, or native acceptance failure prevents
  publication.
- A missing or unavailable matrix runner leaves the release unpublished; it is
  not bypassed automatically.
- A release tag whose release already exists fails rather than replacing assets.
- Publication creates a draft release, uploads the complete tested bundle,
  verifies its assets against `SHA256SUMS`, and only then makes the release
  public. A failed upload must never expose a partial public release. This draft
  is an internal atomic-publication mechanism, not a prerelease version or
  user-facing release channel.
- Diagnostic artifacts may be retained for one day, but release executables are
  never committed to Git.

## Documentation changes

Implementation updates `README.md` and `docs/HOMEBREW.md` to state that all four
release archives are natively executed before publication and that downstream
packaging consumes those exact immutable assets.

## Out of scope

- Windows release binaries.
- Paid, larger, or self-hosted runners.
- Code signing, notarization, SBOM generation, and artifact attestations.
- Full Docker, Podman, Docker Sandbox, SSH-server, or Homebrew service-manager
  integration in the native binary matrix.
- Supporting legacy archive names, versions, configurations, or protocols.
