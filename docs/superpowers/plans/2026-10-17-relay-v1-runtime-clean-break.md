# Relay v1 Runtime Clean Break Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Relay Protocol v1, Zelport Configuration Schema v1, relay-only domain terminology, and the full seven-module Zelport App build invariant executable and test-enforced.

**Architecture:** The SDK owns the single protocol and cryptographic identity; standalone and Built-in Relay consume it; Zelport App persists only fresh schema-v1 state and treats modules as internal composition boundaries. Default compilation includes every module, while the explicit `zelport_core` tag selects diagnostic composition.

**Tech Stack:** Go 1.27, Noise, WebSocket, Make, POSIX shell, Python test harness, Node.js built-in test runner.

**Spec:** `docs/superpowers/specs/2026-10-17-zelport-0.10-relay-v1-clean-break-design.md`

## Global Constraints

- Preserve the unrelated dirty Containers and web-asset changes listed in the spec; inspect their diffs before editing overlapping files.
- Use `GOWORK=off`; do not add `go.work`, `replace` directives, pseudo-versions, compatibility branches, migrations, redirects, or fallback parsers.
- Use `0.10.0` in machine fields and `v0.10.0` only for tags, releases, Go module versions, and human release labels.
- Never embed the retired product, relay-domain, typo, or whole-application tokens in new source, fixtures, comments, errors, filenames, or commit messages. Assemble those tokens from string fragments only in the temporary audit commands.
- Every standard user build must include all seven modules. Partial module compositions require `zelport_core` and are diagnostic only.
- Commit with `zdeneksvarc <79550344+zdeneksvarc@users.noreply.github.com>` and Conventional Commits. Do not rewrite existing shared history.

## Review Focus

- Confirm every version-bearing encoded array, authenticated datum, endpoint, persisted watermark, and configuration marker is exactly version 1.
- Confirm cryptographic labels are centralized and exactly match the spec; an accidental future change must fail a test.
- Confirm direct `go build` and `go install` semantics, not just Make targets, produce the complete App.
- Confirm every rejected old version fails without modifying its input file or generating replacement credentials.

---

### Task 1: Freeze the SDK protocol identity in failing tests

**Files:**
- Modify: `zelport-sdk-go/protocol/route_test.go`
- Modify: `zelport-sdk-go/protocol/catalog_test.go`
- Modify: `zelport-sdk-go/protocol/catalog_envelope_test.go`
- Modify: `zelport-sdk-go/protocol/administration_test.go`
- Modify: `zelport-sdk-go/protocol/trace.go`
- Modify: `zelport-sdk-go/secure/secure_test.go`
- Modify: `zelport-sdk-go/secure/catalog_test.go`
- Modify: `zelport-sdk-go/secure/peer_conn_test.go`
- Modify: `zelport-sdk-go/secure/peer_renewal_test.go`
- Modify: `zelport-sdk-go/secure/peer_benchmark_test.go`
- Add: `zelport-sdk-go/secure/identity_test.go`

- [ ] Add table tests proving `protocol.Version == 1`, all encoded arrays begin with `1`, and versions `0`, `2`, and `3` fail normal validation.
- [ ] Add exact-value tests for the six cryptographic contexts in the spec, including protected/open relay admission, catalog associated data, peer traversal, and renewal.
- [ ] Assert the trace header is `X-Zelport-Relay-Trace` and trace errors use relay terminology.
- [ ] Run `GOWORK=off go test ./protocol ./secure`; verify the new assertions fail against the current implementation for the intended reasons.
- [ ] Commit the red tests: `test(sdk): define relay protocol v1 identity`.

### Task 2: Centralize and implement Relay Protocol v1 in the SDK

**Files:**
- Modify: `zelport-sdk-go/protocol/types.go`
- Modify: `zelport-sdk-go/protocol/wire.go`
- Modify: `zelport-sdk-go/protocol/catalog.go`
- Modify: `zelport-sdk-go/protocol/catalog_envelope.go`
- Modify: `zelport-sdk-go/protocol/route.go`
- Modify: `zelport-sdk-go/protocol/trace.go`
- Modify: `zelport-sdk-go/client/client.go`
- Modify: `zelport-sdk-go/transport/websocket.go`
- Modify: `zelport-sdk-go/secure/secure.go`
- Modify: `zelport-sdk-go/secure/catalog.go`
- Modify: `zelport-sdk-go/secure/peer_handshake.go`
- Modify: `zelport-sdk-go/secure/peer_renewal.go`

- [ ] Introduce one SDK-owned protocol identity definition for version, endpoint path, trace header, base namespace, and the five derived context labels.
- [ ] Replace every hard-coded wire/catalog version with `protocol.Version`; remove version-specific helper paths that exist only to accept a previous protocol.
- [ ] Make the client always dial `/v1/ws`; empty/root relay URLs may be normalized only to that current endpoint.
- [ ] Use exactly the six approved contexts and no fallback derivation, negotiation, or aliases.
- [ ] Replace relay-domain comments, identifiers, and errors with relay terminology.
- [ ] Run `gofmt` on changed Go files.
- [ ] Run `GOWORK=off go test -race ./...`, `GOWORK=off go vet ./...`, and `make check` in `zelport-sdk-go`.
- [ ] Commit: `feat(protocol)!: reset relay protocol to v1` with a `BREAKING CHANGE:` body stating that previous wire data and credentials are not accepted.

### Task 3: Make standalone Zelport Relay serve only v1

**Files:**
- Modify: `zelport-relay/relay/server.go`
- Modify: `zelport-relay/relay/relay_test.go`
- Modify: `zelport-relay/relay/catalog_cache_test.go`
- Modify: `zelport-relay/scripts/test_integration_harness.py`
- Modify: `zelport-relay/scripts/check-sdk-integration.sh`
- Modify: `zelport-relay/tests/sdk-client/client_integration_test.go.txt`
- Modify: `zelport-relay/tests/sdk-client/catalog_lifecycle_test.go.txt`
- Modify: `zelport-relay/tests/sdk-client/catalog_benchmark_test.go.txt`
- Modify: `zelport-relay/tests/sdk-client/relay_benchmark_test.go.txt`

- [ ] First change tests to require `/v1/ws` and 404 for all earlier endpoint versions, then run `GOWORK=off go test ./relay` and observe the focused failure.
- [ ] Change the server to accept only `GET /v1/ws`; preserve origin, capacity, and handshake protections.
- [ ] Update all protocol fixtures to version 1 and the integration harness placeholder to neutral `v0.0.0`.
- [ ] Stage the committed SDK tree as local module `v0.10.0`; point the relay requirement to `v0.10.0` without a `replace` directive and refresh `go.sum` through that proxy.
- [ ] Run `make check`, `make check-integration`, and `make build VERSION=0.10.0` in `zelport-relay`.
- [ ] Verify `bin/zelport-relay --version` reports numeric `0.10.0` inside its existing output format.
- [ ] Commit: `feat(relay)!: serve relay protocol v1`.

### Task 4: Reset Zelport Configuration Schema to v1

**Files:**
- Modify: `zelport/internal/app/config.go`
- Modify: `zelport/internal/app/config_test.go`
- Modify: `zelport/internal/app/modules_test.go`
- Modify: `zelport/internal/app/unsupported_test.go`

- [ ] Add tests for a new absent configuration producing schema version 1 with empty modules/routes and for only explicit version 1 loading successfully.
- [ ] Add rejection tests for missing, `0`, `2`, `3`, and `6`; compare file bytes before and after each failure to prove no rewrite or migration occurred.
- [ ] Change the default and validator to version 1 and make the error direct users to create a fresh version-1 configuration.
- [ ] Keep strict unknown-field, duplicate-field, and trailing-data rejection intact.
- [ ] Run `GOWORK=off go test -race ./internal/app`.
- [ ] Commit: `feat(config)!: reset configuration schema to v1`.

### Task 5: Require Relay Protocol v1 state inside App configuration

**Files:**
- Modify: `zelport/internal/modules/ports_over_relay/config.go`
- Modify: `zelport/internal/modules/ports_over_relay/config_test.go`
- Modify: `zelport/internal/modules/ports_over_relay/registry.go`
- Modify: `zelport/internal/modules/ports_over_relay/registry_test.go`
- Modify: `zelport/internal/modules/ports_over_relay/api_test.go`
- Modify: `zelport/internal/modules/ports_over_relay/administration_test.go`

- [ ] Add a required `protocol_version` field to established Ports over Relay settings and tests that missing, zero, or non-1 values fail with a precise fresh-configuration error.
- [ ] Make fresh identity creation set `protocol_version: 1`; require `DigestVersion == 1` and remove acceptance of an omitted digest version.
- [ ] Restrict relay URL validation to empty path, `/`, or `/v1/ws`; test every earlier versioned path as invalid.
- [ ] Make catalog acceptance persist `DigestVersion: 1` and reject version-3 catalog fixtures without altering stored state.
- [ ] Run `GOWORK=off go test -race ./internal/modules/ports_over_relay`.
- [ ] Commit: `feat(relay)!: require protocol v1 configuration state`.

### Task 6: Rename relay-domain and listener-domain implementation identifiers

**Files:**
- Move: the retired relay-domain integration directory to `zelport/internal/relay/integration/`
- Move: the retired relay-domain demo script to `zelport/scripts/relay-demo.py`
- Modify: `zelport/internal/feature/module.go`
- Modify: `zelport/internal/app/manager.go`
- Modify: `zelport/internal/endpoint/endpoint.go`
- Modify: `zelport/internal/modulekit/proxy.go`
- Modify: `zelport/internal/integration/proxy_test.go`
- Modify: module implementations under `zelport/internal/modules/`
- Modify: `zelport/internal/modules/ports_over_relay/assets/module.js`
- Modify: `zelport/internal/modules/ports_over_relay/assets/style.css`
- Modify: `zelport/internal/web/assets/index.html`
- Modify: `zelport/internal/web/assets/app.js`
- Modify: affected Go and JavaScript tests

- [ ] Before touching dirty assets, record `git diff --` for each overlapping file and preserve the user's behavior changes while renaming only in-scope identifiers.
- [ ] Rename the integration package/directory and demo script; update imports and test names without shims.
- [ ] Rename the current whole-application authority API to `ListenerAuthority`, its proxy option to `ProxyOptions.ListenerAuthority`, and listener/self-target parameters according to the spec.
- [ ] Rename the UI endpoint element, the Ports over Relay tab to `relay`, and all relay-domain CSS/JavaScript identifiers and URL fragments.
- [ ] Change CLI, systemd, errors, comments, and tests to say App, reverse proxy, listener, Localhost port, or endpoint according to meaning.
- [ ] Run Go formatting, Node tests, the Python relay demo test mode, and focused Go tests.
- [ ] Commit: `refactor!: align relay and listener terminology`.

### Task 7: Make standard App compilation include all modules

**Files:**
- Modify: `zelport/cmd/zelport/module_*.go`
- Add: `zelport/cmd/zelport/modules_standard_test.go`
- Modify: `zelport/Makefile`
- Modify: `zelport/scripts/check-module-builds.sh`
- Modify: `workspace/Makefile`

- [ ] Write a composition test that expects the seven approved module IDs from an ordinary build and a shell test that inspects direct `go list -deps ./cmd/zelport` output.
- [ ] Invert build constraints so ordinary builds register every module; use `!zelport_core || <module_tag>` semantics so `zelport_core` alone is empty and `zelport_core,<module>` selects an individual boundary.
- [ ] Make `make build` call ordinary full compilation without feature tags; make `make build-core` explicitly pass `zelport_core` and output development-only `bin/zelport-core`.
- [ ] Rewrite the module matrix to test `zelport_core`, each `zelport_core,<module>`, selected combinations, and the ordinary full build.
- [ ] Prove `go build ./cmd/zelport`, `go install ./cmd/zelport`, `make build`, and release packaging all link the same seven modules.
- [ ] Run `make check-modules`, `make test`, `make vet`, and `make build VERSION=0.10.0` in `zelport`.
- [ ] Commit: `build!: make the complete app the standard build`.

### Task 8: Verify runtime integration as one clean-break slice

**Files:**
- Modify as failures require: runtime tests only; do not broaden behavior.

- [ ] Stage clean committed SDK and relay trees as exact local `v0.10.0` modules using Go module ZIP rules and record their `h1:` sums.
- [ ] Update App `go.mod` to SDK and relay `v0.10.0`, refresh `go.sum` through the local file proxy, and prove no workspace or local replacement is required.
- [ ] Run, in order: SDK `make check`; relay `make check check-integration`; App `make check check-modules`; App `node --test scripts/*.test.mjs`; workspace `make check check-modules build`.
- [ ] Exercise standalone Relay and Built-in Relay against `/v1/ws`; confirm every previous endpoint path is 404 and version-3 records fail.
- [ ] Build `zelport`, `zelport-relay`, and diagnostic `zelport-core`; scan printable strings for retired identity and terminology tokens assembled from fragments.
- [ ] Commit dependency pins and any strictly necessary integration fixture updates: `build(deps): pin relay components to v0.10.0`.

### Task 9: Runtime self-review gate

**Files:**
- Review: all changes from Tasks 1–8

- [ ] Compare the diff against the spec sections “Product architecture”, “Domain terminology”, “Protocol identity”, and “Test strategy”.
- [ ] Verify no compatibility, migration, fallback, or legacy-named alias was introduced.
- [ ] Verify dirty user changes are still present with `git diff` and were not reformatted outside the required lines.
- [ ] Run `git diff --check` in SDK, Relay, App, and workspace.
- [ ] Do not tag, push, or publish from this plan; publication is gated by the coordinated plan.
