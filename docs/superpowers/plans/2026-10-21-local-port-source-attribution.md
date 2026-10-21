# Local Port Source Attribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Classify Local Ports from cached authoritative container and Docker Sandbox inventories without delaying picker rendering on macOS or Linux.

**Architecture:** A module-neutral `internal/portorigin` catalog refreshes container and sandbox providers independently in the background and exposes immutable last-known-good snapshots with health. Local Ports retains listener binding records, correlates them with cached published mappings, and uses process identity only after complete negative attribution.

**Tech Stack:** Go standard library, existing `internal/containers` and `internal/sbx` clients, embedded JavaScript/CSS, Node built-in test runner.

**Spec:** `docs/superpowers/specs/2026-10-21-local-port-source-attribution-design.md`

## Global Constraints

- Local Ports must not import either feature module under `internal/modules/containers` or `internal/modules/docker_sandboxes`.
- Opening `GET /api/modules/local_ports/listeners` must never execute Docker, Podman, nerdctl, or `sbx`.
- Docker, Podman, nerdctl, and `sbx` refresh independently; one slow or failed provider cannot suppress another provider's result.
- Manual Aux-port entry and selection remain usable for `ready`, `refreshing`, `stale`, `degraded`, and ambiguous attribution.
- Process-name substrings such as `gvproxy` and `docker-proxy` are never classification rules.
- Missing CLIs are not failures; installed but unreachable or malformed providers are degraded.
- Project runtimes, builds, dependency installation, and tests run in Docker Sandbox `zelport-git`, never directly on macOS.
- Existing unrelated working-tree changes are preserved and are not reformatted or reverted.

## Review Focus

- An installed provider that first reports `not_found` and later becomes available must recover on the next refresh; Task 1 adds this lifecycle test.
- A refresh requested during an active refresh must coalesce yet let every waiter observe completion; Task 1 adds this concurrency test.
- A wildcard IPv4 mapping must not claim an IPv6-only listener on the same port; Task 3 adds this address-family test.
- An old saved container `Port` without binding metadata must remain decodable and remappable; Task 2 adds this compatibility test.
- Closing the service while a provider is blocked must cancel and join every goroutine; Task 1 adds this shutdown test.

---

### Task 1: Background source inventory catalog

**Files:**
- Create: `internal/portorigin/catalog.go`
- Create: `internal/portorigin/catalog_test.go`

**Interfaces:**
- Consumes: `containers.Inventory`, independent `containers.Client` backends for Docker, Podman, and nerdctl, plus `sbx.Client` and `sbx.Sandbox`.
- Produces: `portorigin.Health` constants `refreshing`, `ready`, `stale`, and `degraded`.
- Produces: `portorigin.Snapshot{Containers []containers.Inventory, Sandboxes []sbx.Sandbox, Health Health, Generation uint64, UpdatedAt time.Time}`.
- Produces: `Catalog.Start(context.Context, time.Duration)`, `Catalog.RequestRefresh() <-chan struct{}`, `Catalog.Snapshot() Snapshot`, and `Catalog.Close() error`.
- Produces: injectable `ContainerProvider` with `Scan(context.Context) containers.Inventory`; each `containers.Client` satisfies it and is scheduled as an independent provider.
- Produces: injectable `SandboxProvider` with `Info(context.Context) sbx.CLIInfo`, `List(context.Context) ([]sbx.Sandbox, error)`, and `Ports(context.Context, string) ([]sbx.Port, error)`; `sbx.Client` satisfies it.

- [ ] **Step 1: Write failing catalog lifecycle tests**

Add `TestCatalogRefreshesProvidersIndependently`, `TestCatalogPreservesLastKnownGoodAsStale`, `TestCatalogCoalescesConcurrentRefreshRequests`, `TestCatalogRecoversProviderInstalledAfterStartup`, and `TestCatalogCloseCancelsBlockedProviders`. Assert immutable snapshot copies, monotonic generations, independent positive results, and joined shutdown.

- [ ] **Step 2: Run the focused tests and verify RED**

Run in Docker Sandbox: `GOWORK=off go test ./internal/portorigin -run TestCatalog -count=1`

Expected: FAIL because `internal/portorigin` does not exist.

- [ ] **Step 3: Implement the minimal catalog**

Implement the interfaces above. Run providers concurrently under independent child deadlines and commit each provider result as soon as it finishes rather than waiting for the generation's slowest provider. Serialize each provider, preserve successful snapshots on error, coalesce refresh requests into one generation, and return deep copies. Missing CLIs contribute a complete empty result; an installed failing provider contributes degraded or stale health.

- [ ] **Step 4: Verify GREEN and race safety**

Run in Docker Sandbox:

- `GOWORK=off go test ./internal/portorigin -count=1`
- `GOWORK=off go test -race ./internal/portorigin -count=1`

Expected: PASS with no race reports.

- [ ] **Step 5: Commit the catalog**

Commit the two task files with message `feat(discovery): cache local port origins`.

### Task 2: Preserve authoritative binding metadata

**Files:**
- Modify: `internal/containers/types.go`
- Modify: `internal/containers/client.go`
- Modify: `internal/containers/client_test.go`
- Modify: `internal/containers/status_test.go`
- Modify: `internal/sbx/client.go`
- Modify: `internal/sbx/client_test.go`

**Interfaces:**
- Produces: `containers.Port.Binding() (netip.Addr, bool)` while keeping `Address()` dialable through loopback.
- Produces: `sbx.Port.Binding() (netip.Addr, bool)` with the same semantics.
- Preserves: decoding older `containers.Port` JSON that has no new binding metadata.
- Consumed by: Task 3 correlation.

- [ ] **Step 1: Write failing binding-preservation tests**

Add table tests for IPv4/IPv6 loopback and wildcard bindings. Assert that wildcard input remains distinguishable through `Binding()`, while `Address()` still returns a loopback dial target. Add a compatibility test decoding the previous four-field `Port` JSON and remapping it by its existing host IP.

- [ ] **Step 2: Run the focused tests and verify RED**

Run in Docker Sandbox: `GOWORK=off go test ./internal/containers ./internal/sbx -run 'Test.*Binding|Test.*Compatibility' -count=1`

Expected: FAIL because binding metadata and `Binding()` do not exist.

- [ ] **Step 3: Implement binding preservation**

Add a JSON-compatible binding field to each port model. Parsers capture the runtime-reported bind address before deriving a loopback dial address. An absent compatibility field falls back to `HostIP`. Do not change the rejection of specific non-local interface bindings.

- [ ] **Step 4: Verify GREEN and existing engine parsers**

Run in Docker Sandbox: `GOWORK=off go test ./internal/containers ./internal/sbx -count=1`

Expected: PASS.

- [ ] **Step 5: Commit binding metadata**

Commit only the six task files with message `feat(discovery): preserve published bind scope`.

### Task 3: Correlate listeners with cached origins

**Files:**
- Modify: `internal/modules/local_ports/listeners.go`
- Create: `internal/modules/local_ports/attribution.go`
- Modify: `internal/modules/local_ports/listeners_test.go`
- Modify: `internal/modules/local_ports/module_test.go`

**Interfaces:**
- Consumes: `portorigin.Snapshot` and the `Binding()` methods from Task 2.
- Produces: unexported `ListenerBinding{Address netip.Addr, Protocol string}` records retained on each normalized listener with `json:"-"`.
- Produces: `attributeListeners([]Listener, portorigin.Snapshot) []Listener`.
- Produces: top-level response fields `attribution_health`, `attribution_generation`, and `attribution_updated_at` without exposing raw socket paths or provider errors.
- Produces: per-row `kind` values `local_process`, `container`, `docker_sandbox`, or `unknown`, and `source_status` values `ready`, `stale`, `refreshing`, `degraded`, or `ambiguous`.

- [ ] **Step 1: Write failing listener-binding tests**

Extend normalization tests to prove raw IPv4 loopback, IPv6 loopback, wildcard, and mixed bindings survive internal grouping but never appear in JSON.

- [ ] **Step 2: Run the normalization tests and verify RED**

Run in Docker Sandbox: `GOWORK=off go test ./internal/modules/local_ports -run TestDiscoverListeners -count=1`

Expected: FAIL because normalized listeners discard binding records.

- [ ] **Step 3: Retain internal binding records and verify GREEN**

Implement `ListenerBinding` and deterministic deduplication. Keep the public one-row-per-port response unchanged apart from the attribution fields introduced later in this task.

- [ ] **Step 4: Write failing correlation tests**

Add tests for sandbox precedence, a Podman mapping over macOS `gvproxy`, Linux proxy/rootless/no-PID fixtures, exact versus wildcard matching, IPv4/IPv6 isolation, duplicate CLI aliases, distinct-runtime ambiguity, positive stale evidence, complete negative local classification, and refreshing/degraded/stale-negative unknown classification.

- [ ] **Step 5: Run correlation tests and verify RED**

Run in Docker Sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestAttribute|TestContainerEnricher|TestSandboxEnricher' -count=1`

Expected: FAIL because `attributeListeners` and health-aware classification do not exist.

- [ ] **Step 6: Implement correlation and remove request-time enrichers**

Match normalized protocol, address family, bind scope, and host port. Deduplicate container mappings only by connection fingerprint, container ID, binding, host port, container port, and protocol. Keep available process metadata as context, but emit `Local process` only for complete successful negative attribution.

- [ ] **Step 7: Verify GREEN**

Run in Docker Sandbox: `GOWORK=off go test ./internal/modules/local_ports -count=1`

Expected: PASS.

- [ ] **Step 8: Commit correlation**

Commit the four task files with message `feat(local-ports): attribute listener origins`.

### Task 4: Wire lifecycle, API refresh, and non-blocking UI

**Files:**
- Modify: `internal/modules/local_ports/module.go`
- Modify: `internal/modules/local_ports/module_test.go`
- Modify: `internal/modules/local_ports/assets/module.js`
- Modify: `internal/modules/local_ports/assets/style.css`
- Modify: `scripts/local-ports.test.mjs`

**Interfaces:**
- Consumes: `portorigin.Catalog` and `attributeListeners` from Tasks 1 and 3.
- Produces: eager and 15-second periodic catalog refresh from `Module.Start`, with cancellation and joining from `Module.Close`.
- Produces: `GET /listeners` that performs only OS listener scanning plus cached attribution.
- Produces: `POST /listeners/refresh` that requests a coalesced background generation and responds after completion or request cancellation without clearing rendered rows.
- Produces: `Kind: Unknown source` tooltip with safe health text.

- [ ] **Step 1: Write failing module lifecycle and latency tests**

Inject a catalog with blocked providers. Assert `GET /listeners` returns cached Podman mappings for ports 1212 and 1313 without invoking or waiting for providers; assert a slow sandbox provider cannot suppress them. Assert `POST /listeners/refresh` coalesces requests, `Close` joins refresh work, and non-GET/non-POST methods retain safe method handling.

- [ ] **Step 2: Run Go tests and verify RED**

Run in Docker Sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestListenersAPI|TestLocalPortAttributionLifecycle' -count=1`

Expected: FAIL because the module still creates request-time enrichers and has no refresh endpoint.

- [ ] **Step 3: Implement module lifecycle and API**

Construct the default module with a neutral catalog, start it independently of feature-module availability, and replace request-time CLI work with `Snapshot()`. Keep the two-second deadline only around OS listener collection. Return normalized health, timestamps, and source status without raw provider errors.

- [ ] **Step 4: Verify Go GREEN**

Run in Docker Sandbox: `GOWORK=off go test ./internal/modules/local_ports -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing UI tests**

Assert first render uses the returned cached rows immediately, a blocked refresh never clears or disables selection, completion updates rows in place, `Unknown source` appears only for unconfirmed attribution, positive stale container/sandbox kinds remain visible, and disposal aborts the waiting HTTP request without leaking UI listeners.

- [ ] **Step 6: Run UI tests and verify RED**

Run in Docker Sandbox: `node --test scripts/local-ports.test.mjs`

Expected: FAIL because the UI has no attribution health or background refresh contract.

- [ ] **Step 7: Implement UI behavior and verify GREEN**

Reuse the existing kind-icon and delayed-tooltip primitives. Keep current rows during refresh, update after completion, and preserve the current port value, focus, scroll, warning order, and `kind → Choose` icon adjacency.

Run in Docker Sandbox: `node --test scripts/local-ports.test.mjs scripts/ui-primitives.test.mjs scripts/render-stability.test.mjs`

Expected: PASS.

- [ ] **Step 8: Commit lifecycle and UI**

Commit the five task files with message `feat(local-ports): render cached source attribution`.

### Task 5: Cross-platform and native acceptance verification

**Files:**
- Modify if required by verified behavior: `DESIGN.md`
- Modify if required by test harness: existing relevant test files only

**Interfaces:**
- Consumes: the completed source-attribution behavior from Tasks 1–4.
- Produces: a macOS arm64 test binary at `/Users/zdeneksvarc/Git/github.com/zelport/zelport-darwin-arm64-test` after automated verification succeeds.

- [ ] **Step 1: Run the complete automated verification**

Run in Docker Sandbox:

- `GOWORK=off go test -race ./...`
- `GOWORK=off go vet ./...`
- `node --test scripts/*.test.mjs`
- `GO=go sh scripts/check-module-builds.sh`
- `GOWORK=off CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/zelport-darwin-arm64-test ./cmd/zelport`

Expected: every command exits 0 with no race report, vet error, test failure, or cross-build failure.

- [ ] **Step 2: Copy and launch the verified macOS binary**

Copy the sandbox-built artifact to `/Users/zdeneksvarc/Git/github.com/zelport/zelport-darwin-arm64-test`, preserve executable mode, stop only the test process previously started from that exact path if necessary, and launch it on port 7070 with the user's existing configuration.

- [ ] **Step 3: Verify the native macOS acceptance case**

Using the authenticated local API and integrated browser, prove:

- ports 1212 and 1313 are `Kind: Container`, identify `dummy-1212` and `dummy-1313`, and map to `80/tcp` through Podman;
- opening the picker returns and renders before a deliberately blocked refresh finishes;
- Docker Sandbox and ordinary local-process rows keep their correct kinds;
- the layout and tooltips remain correct at narrow and wide viewport widths;
- no request-time Podman or `sbx` process is spawned by repeated picker opens.

- [ ] **Step 4: Record the durable design rule**

Update `DESIGN.md` only if the source-attribution and non-blocking-discovery rules are not already represented. State the rule, not the implementation history.

- [ ] **Step 5: Run final verification and commit acceptance changes**

Run in Docker Sandbox: `make verify`

Expected: PASS. Commit only Task 5 documentation or test-harness changes, if any, with message `test(local-ports): verify native source attribution`.

