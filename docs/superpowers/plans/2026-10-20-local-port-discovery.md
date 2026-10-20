# Local Port Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only, cross-platform inventory of listening TCP ports to the Local Ports service dialog while preserving unrestricted manual port entry.

**Architecture:** The Local Ports module owns a small scanner interface, common normalization, Linux `/proc` and macOS `lsof` adapters, and a GET-only administration endpoint. The dialog loads that endpoint lazily into an accessible disclosure and marks ports whose bind scope extends beyond loopback.

**Tech Stack:** Go standard library, build-tagged Darwin/Linux adapters, embedded JavaScript/CSS, Node's built-in test runner.

**Spec:** `docs/superpowers/specs/2026-10-20-local-port-discovery-design.md`

## Global Constraints

- Manual Aux-port entry continues to accept any integer from 1 through 65535, whether or not a listener exists.
- Discovery must never scan ports, connect to listeners, probe HTTP, mutate configuration, or expose bind addresses and command lines.
- The release targets remain macOS and Linux on arm64 and amd64 with `CGO_ENABLED=0`.
- Project builds, tests, and runtimes run only in Docker Sandbox `zelport-tests.sbx`, never on the macOS host.
- The UI warning text is exactly `The port listens on non-loopback interfaces too.`
- Discovery failure text is exactly `Listening ports couldn't be detected. Enter the Aux port manually.` and must never disable manual entry or submission.
- Existing unrelated working-tree changes are preserved and excluded from feature commits.

## Review Focus

- A wildcard listener recorded in both IPv4 and IPv6 must produce one port row and one warning.
- A loopback listener plus a specific non-loopback listener on the same port must remain selectable and show the warning.
- A listener present without readable process metadata must remain selectable as `Unknown process`.
- A disappearing process or socket during collection must not fail the whole inventory.
- Closing the dialog during a request must prevent late DOM updates and leave no disclosure listeners attached.

---

### Task 1: Listener model, normalization, and parsers

**Files:**
- Create: `internal/modules/local_ports/listeners.go`
- Create: `internal/modules/local_ports/listeners_proc.go`
- Create: `internal/modules/local_ports/listeners_lsof.go`
- Create: `internal/modules/local_ports/listeners_test.go`

**Interfaces:**
- Produces: `listenerScanner.Scan(context.Context) ([]rawListener, error)`.
- Produces: `discoverListeners(context.Context, listenerScanner, int) ([]Listener, error)`, where the integer is the Zelport listener port to exclude.
- Produces: `Listener{Port int, Processes []ProcessIdentity, ListensBeyondLoopback bool}` for the API layer.

- [ ] **Step 1: Write failing normalization tests**

Add table-driven tests proving loopback-only, wildcard, mixed loopback/non-loopback, non-loopback-only exclusion, IPv4/IPv6 deduplication, deterministic process ordering, missing process metadata, and exclusion of the active Zelport listener.

- [ ] **Step 2: Run the focused tests and verify RED**

Run in the sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestDiscoverListeners'`

Expected: FAIL because the listener model and normalization do not exist.

- [ ] **Step 3: Implement the common model and normalization**

Classify loopback, wildcard, and specific addresses without DNS. Merge by TCP port, preserve unique process identities, derive `listens_beyond_loopback`, sort deterministically, and retain listeners whose process metadata is unavailable.

- [ ] **Step 4: Add failing parser tests**

Test representative `/proc/net/tcp`, `/proc/net/tcp6`, and NUL-delimited `lsof -F0pcn` fixtures, including malformed records, IPv6, wildcard binds, missing command fields, and bounded partial records.

- [ ] **Step 5: Run parser tests and verify RED**

Run in the sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestParse(Proc|Lsof)'`

Expected: FAIL because the parsers do not exist.

- [ ] **Step 6: Implement the minimal parsers and verify GREEN**

Run in the sandbox: `GOWORK=off go test ./internal/modules/local_ports`

Expected: PASS.

- [ ] **Step 7: Commit the common discovery layer**

Commit only the four task files with message `feat(local-ports): model listener discovery`.

### Task 2: Linux and macOS collectors

**Files:**
- Create: `internal/modules/local_ports/listeners_linux.go`
- Create: `internal/modules/local_ports/listeners_linux_test.go`
- Modify: `internal/modules/local_ports/listeners_lsof.go`
- Create: `internal/modules/local_ports/listeners_lsof_test.go`
- Create: `internal/modules/local_ports/listeners_darwin.go`

**Interfaces:**
- Consumes: `rawListener` and the parsers from Task 1.
- Produces: build-tagged `newListenerScanner() listenerScanner` on Linux and Darwin.

- [ ] **Step 1: Write failing Linux collector tests**

Use a temporary proc-like tree to prove that listener rows survive unreadable `/proc/<pid>/fd`, readable socket inodes acquire PID/name, vanished descriptors are ignored, and context cancellation stops collection.

- [ ] **Step 2: Run the Linux tests and verify RED**

Run in the sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestProcScanner'`

Expected: FAIL because `procScanner` does not exist.

- [ ] **Step 3: Implement the Linux collector and verify GREEN**

Read `/proc/net/tcp*` first, then best-effort numeric PID directories, `comm`, and `fd` symlinks. Apply explicit record limits and check `ctx.Err()` during traversal.

Run in the sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestProcScanner'`

Expected: PASS.

- [ ] **Step 4: Write failing macOS collector tests**

Keep the command-independent `lsofScanner` in `listeners_lsof.go` so its behavior can run under Linux tests. Inject executable lookup and command execution. Prove known system paths precede `PATH`, arguments are exactly `-nP -iTCP -sTCP:LISTEN -F0pcn`, output is capped, non-zero exit becomes a typed discovery error, and cancellation is propagated.

- [ ] **Step 5: Run the macOS tests and verify RED**

Run in the sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestLsofScanner'`

Expected: FAIL because `lsofScanner` does not exist.

- [ ] **Step 6: Implement the macOS collector and cross-compile**

Invoke `lsof` directly without a shell, use a bounded writer, and parse only field output. Do not fall back to formatted `lsof`, `netstat`, or `ss` tables.

Run in the sandbox:

- `GOWORK=off go test ./internal/modules/local_ports`
- `GOWORK=off CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go test -c -o /tmp/local_ports_darwin_amd64.test ./internal/modules/local_ports`
- `GOWORK=off CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go test -c -o /tmp/local_ports_darwin_arm64.test ./internal/modules/local_ports`

Expected: all commands succeed.

- [ ] **Step 7: Commit the platform collectors**

Commit only the five task files with message `feat(local-ports): collect system listeners`.

### Task 3: Local Ports administration API

**Files:**
- Modify: `internal/modules/local_ports/module.go`
- Modify: `internal/modules/local_ports/module_test.go`

**Interfaces:**
- Consumes: `newListenerScanner()` and `discoverListeners(...)` from Tasks 1–2.
- Produces: `GET /api/modules/local_ports/listeners` returning `{"listeners":[...]}`.

- [ ] **Step 1: Write failing endpoint tests**

Inject a fake scanner and prove GET returns stable JSON, non-GET returns 405, scanner failure returns a safe 503 JSON error, the request context is passed through, and the current Zelport listener port is excluded.

- [ ] **Step 2: Run the endpoint tests and verify RED**

Run in the sandbox: `GOWORK=off go test ./internal/modules/local_ports -run 'TestListenersAPI'`

Expected: FAIL because the endpoint does not exist.

- [ ] **Step 3: Implement the endpoint**

Give each scan a two-second child context. Return no-store JSON through the module's existing admin boundary; never persist results in module state.

- [ ] **Step 4: Verify GREEN and module routing**

Run in the sandbox:

- `GOWORK=off go test ./internal/modules/local_ports`
- `GOWORK=off go test ./internal/web -run 'TestCoreAndAbsentModuleAssets|TestModuleMutationSecurityAndConflict'`

Expected: PASS.

- [ ] **Step 5: Commit the API**

Commit only `module.go` and `module_test.go` with message `feat(local-ports): expose listener inventory`.

### Task 4: Lazy listener picker and warning UI

**Files:**
- Modify: `internal/modules/local_ports/assets/module.js`
- Modify: `internal/modules/local_ports/assets/style.css`
- Modify: `internal/web/assets/ui.js`
- Create: `scripts/local-ports.test.mjs`
- Modify: `scripts/ui-primitives.test.mjs`

**Interfaces:**
- Consumes: `GET /api/modules/local_ports/listeners` and the shared `disclosureMenu`, `inputAdornment`, `warningIndicator`, and icon helpers.
- Produces: an Aux-port adornment that loads on first open, refreshes in place, selects a port without submitting, and renders the exact non-loopback tooltip.
- Produces: optional `dispose()` support for service-dialog field groups so closing a dialog aborts in-flight discovery and releases disclosure listeners.

- [ ] **Step 1: Write failing UI tests**

Prove manual entry remains unchanged, first opening performs one GET, repeated opening uses the loaded result, refresh performs another GET, selection updates only the input, unknown processes remain visible, non-loopback rows carry the exact tooltip, loading uses the shared spinner, failures show the exact non-blocking message while leaving input and submission usable, and closing the dialog aborts discovery, ignores late results, and disposes the menu.

- [ ] **Step 2: Run the UI test and verify RED**

Run in the sandbox: `node --test scripts/local-ports.test.mjs`

Expected: FAIL because the picker does not exist.

- [ ] **Step 3: Implement the picker and scoped styles**

Use existing UI primitives and module-scoped CSS. Extend `serviceModule` only with the optional field-group cleanup hook. Keep the list sorted by the API, show `Unknown process` when empty, mark the refresh action busy, and preserve the domain draft, input value, focus, and menu scroll during refresh.

- [ ] **Step 4: Verify GREEN and regression suites**

Run in the sandbox:

- `node --test scripts/local-ports.test.mjs scripts/ui-primitives.test.mjs scripts/render-stability.test.mjs`
- `GOWORK=off go test -race ./...`
- `GOWORK=off go vet ./...`
- `GO=go sh scripts/check-module-builds.sh`

Expected: all commands pass without warnings.

- [ ] **Step 5: Rebuild and restart the sandbox preview**

Build with `GOWORK=off go build -o bin/zelport ./cmd/zelport`, restart it bound to `0.0.0.0:9090`, and verify `curl -fsS http://127.0.0.1:9090/api/state` succeeds from inside the sandbox and `curl -fsS http://localhost:9090/` succeeds from macOS.

- [ ] **Step 6: Commit the UI**

Stage only feature-specific hunks from the five task files and commit them with message `feat(local-ports): choose a listening port`.

- [ ] **Step 7: Run complete project verification**

Run in the sandbox: `make verify`

Expected: PASS. Record any native macOS runtime test that remains CI-only; Darwin parser tests and both Darwin cross-builds must already be green.
