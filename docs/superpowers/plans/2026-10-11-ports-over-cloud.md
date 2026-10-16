# Ports over Cloud Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Execution record:** Integrated v1 is implemented. See [verification evidence](../../ports-over-cloud-verification.md) for completed checks, review fixes and explicit coverage/deployment limits. The original checklist below is retained as a specification; it is not a claim that every proposed named test was executed.

**Goal:** After modularization, integrate private, single-developer cloud service sharing as an optional Snitzel module and ship a self-hosted relay CLI.

**Architecture:** Two production commands share root-module cloud packages. Every connection is outbound; relay admission and peer E2E are separate authenticated channels. A service registry dispatches only authorized own routes directly into existing proxy behavior.

**Tech Stack:** Go 1.27, standard-library HTTP/crypto/configuration, coder/websocket, flynn/noise, hashicorp/yamux, embedded HTML/CSS/JavaScript.

**Spec:** [Agreed requirements](../../../PORTS_OVER_CLOUD.md), [v1 technical design](../../ports-over-cloud-v1.md), and [module contracts](../../MODULE_CONTRACTS.md). The protocol is now implemented; this is not a completed independent security review.

## Prerequisite and interfaces

The [modularization plan](2026-10-11-modularization.md) has established the core
and five source modules. See its verification record for test coverage. Use the exact
[normative contracts](../../MODULE_CONTRACTS.md): module-scoped Host, revisioned
Commit, exportable-only Acquire/Dispatch, tracked scopes, and module frontend.
Core v6 envelopes/UUIDs already exist; this plan adds only cloud-owned settings.
Cloud transport packages remain independent of app/web and all source modules.

Register the factory in `cmd/zelport/module_ports_over_relay.go` guarded by
`ports_over_relay`. API/assets live in `internal/modules/ports_over_relay`.
Add the tag to full builds only when integrated. Test core-only, cloud-only,
cloud with every source and full; relay remains independent of client tags.

## Global Constraints

- One Snitzel instance connects to one relay and one E2E group; no accounts/provisioning API.
- Documentation, comments, UI, and errors are English. Communication with the user may remain Czech.
- Preserve loopback administration, Origin/CSRF protection, and atomic configuration. No legacy configuration support or migration; version 6 is a clean break.
- Use temporary configs and controlled backends; never change or restart the user's running instance for tests.
- Publishing defaults off and includes all present/future own services, never administration/imports.
- UUIDs identify devices/services; hostname/domain are labels. Pin the device key with an import.
- No Aux tunnel listener, VPN, P2P, or blind HTTP retries. Preserve streaming, WebSockets, and application errors.
- Mandatory E2E for WS and WSS; relay never receives group token or private peer keys.
- Maintain Sandbox port, Aux port, and Localhost port terminology.
- Follow the exact protocol constants and bounds in the technical design; changes require updating the design and tests together.
- The experiment stays isolated. Root production dependencies are introduced explicitly in Task 2.
- Checkpoint only files belonging to this work; do not commit unrelated existing untracked project files.

## Review Focus

- A rolled-back v6 config or copied device identity must not silently replace a pinned provider (Tasks 1, 3, 4).
- A publish-disable or route edit racing a WebSocket upgrade must close all affected traffic (Tasks 5, 6).
- Sleeping laptops, clock skew, and slow providers must not block healthy providers or create retry storms (Tasks 3, 5).
- Go HTTP transport must not replay a request whose result was lost after backend execution (Task 6).
- Local/Remote intermediaries may strip trace metadata; report limits and bound resulting resource use (Tasks 6, 9).

## File and dependency map

| Files | Responsibility |
| --- | --- |
| `internal/cloud/protocol/{types,catalog,wire,trace}.go` | Wire schemas, versioning, strict encoders and limits |
| `internal/cloud/secure/{keys,admission,peer,conn}.go` | KDF, Noise state machines, bounded encrypted records |
| `internal/cloud/relay/{server,registry,pairing}.go` | Admitted WS connections, catalog routing, tunnel pairing |
| `internal/cloud/client/{client,catalog,sessions,streams}.go` | Outbound channels, verified snapshots, session lifecycle |
| `internal/modules/ports_over_relay/{module,config,store}.go` | Module settings, identity, pins and import validation through Host.Commit |
| `internal/modules/ports_over_relay/{registry,proxy,trace,api}.go` | Host dispatch adapter, scopes, imports, middleware and protected module API |
| `internal/modules/ports_over_relay/assets/{module.js,style.css}` | Setup, catalog/import UI and Dashboard editor hook |
| `cmd/zelport/module_ports_over_relay.go`, `cmd/zelport-relay/main.go` | Tagged module registration, relay CLI and shutdown |
| `internal/cloud/integration/*_test.go`, `scripts/cloud-demo.py` | Adversarial/process verification using temporary state |
| Root `go.mod`, new `go.sum`, `Makefile`, docs | Dependencies, builds and operating instructions |

Cloud packages depend only on protocol/secure packages and supplied callbacks, never app/web. Place tests beside each component; integration tests use public component interfaces. Do not mechanically copy the experimental peer file.

### Task 1: Cloud module settings and stable identity

**Files:** Create `internal/modules/ports_over_relay/{module,config,store}.go`,
`config_test.go`, `store_test.go`, `internal/cloud/protocol/types.go` and tagged
`cmd/zelport/module_ports_over_relay.go`.

**Interfaces:** `New() feature.Module`. Private `Settings` owns enabled/relay URL/
tokens/publish, persistent device UUID/key, outgoing sequence and provider pin
watermarks. Private `ImportSettings` owns group/device/service/key and rewrite_origin.
Protocol Identity/Import/Service types stay in cloud packages. Encode these under
module/route Settings JSON; do not add fields to feature.Route. `store.Update(ctx
context.Context, options feature.CommitOptions, mutate func(*Settings) error) error`
uses Host.Snapshot and Host.Commit; retry only pure internal counter updates on
ErrConflict, never externally executed or user actions.

- [ ] Add `TestCloudSettingsWriteFailure`, `TestCloudServiceIdentityLifecycle`,
  `TestCloudSecretState`: rejected/failed commits preserve state, export IDs equal
  core UUIDs, rename preserves identity, and State exposes neither token nor key.
- [ ] Run `go test ./internal/modules/ports_over_relay`; observe failure before
  implementing settings validation, disabled `{}` defaults and sanitized State.
- [ ] Implement generation of identity/group token only on explicit setup and
  persist before publication. Module Start owns Host.Watch and module context;
  Close is safe before full startup. Descriptor is non-exportable, order 60,
  tab cloud, polling 15 seconds. Empty cloud-only build creates no own services.
- [ ] Add `TestCloudIdentityRecovery`, `TestCloudSettingsAtomic`,
  `TestCloudOldGroupImports`, `TestCloudConcurrentCounterAndUserEdit`: malformed
  identity fails, no implicit group rebinding, and revision conflicts cannot lose
  pins, counters or user settings. Counter commits reset no connection scopes.
- [ ] Run module race tests, core-only build and cloud-only build; checkpoint.

### Task 2: Wire contracts and authenticated transport

**Files:** Create protocol `wire.go`, secure package files and corresponding tests; modify root `go.mod`, add root `go.sum`; update `AGENTS.md` dependency statement only when dependencies actually change.

**Interfaces:** `secure.Derive(secret []byte, purpose string) ([32]byte, error)`; `secure.Admit(ctx context.Context, conn net.Conn, initiator bool, token []byte, hello protocol.Hello) (net.Conn, error)`; `secure.Peer(ctx context.Context, conn net.Conn, initiator bool, identity protocol.Identity, groupToken []byte, peer protocol.PeerContext) (net.Conn, error)`. `Hello` and `PeerContext` contain exactly the version, role/context and identity fields specified in the design. Both functions take ownership and close failed connections.

- [ ] Add golden tests `TestKDFVectors`, `TestWireBounds`, `TestAdmissionReplay`, `TestPeerIdentityBinding`: exact purpose separation, framing boundaries, captured hello/ready replay, wrong tokens/UUID/keys/version/session, and no application bytes before both confirmations.
- [ ] Run `go test ./internal/cloud/secure ./internal/cloud/protocol`; verify failures before implementing transport.
- [ ] Pin reviewed libraries and implement strict wire codecs, NNpsk2 admission, XXpsk3 peer handshake, fixed prologues, bounded record connections and counters. Use libraries' cryptographic primitives without custom cipher constructions.
- [ ] Add `TestCiphertextTamperReplay`, `TestRekeyBidirectional`, `TestHandshakeCancellation`, `TestAdmissionSecretCapture`; test malformed lengths before allocation and no raw credential/plaintext in captures. Use official/library vectors where applicable and freeze application vectors.
- [ ] Run `go test -race ./internal/cloud/secure ./internal/cloud/protocol` and short fuzz smoke runs for decoders. Review authentication order against the design, then checkpoint.

### Task 3: Authenticated catalog and traversal envelopes

**Files:** Create `internal/cloud/protocol/catalog.go`, `trace.go` and tests; extend `internal/modules/ports_over_relay/store.go` persistence; create `internal/cloud/client/catalog.go` and tests.

**Interfaces:** `protocol.Record` contains all catalog fields in the design. `SignRecord(record Record, key []byte) ([]byte, error)` and `VerifyRecord(encoded, key []byte, now time.Time) (Record, error)`. `protocol.Trace` and `AdvanceTrace(encoded string, key []byte, device, service string, now time.Time) (string, error)` enforce eight visits; empty input creates a trace. The module store persists the next outgoing sequence and incoming pinned-provider watermarks through Host.Commit before exposing their result.

- [ ] Add `TestCatalogForgery`, `TestCatalogReplayRestart`, `TestCatalogDuplicateIDs`, `TestCatalogClockSkew`: 60-second expiry, 5-second skew, lower sequence/conflicting equal sequence rejection, exact equal record acceptance, and persistence failure rollback.
- [ ] Run `go test ./internal/cloud/protocol ./internal/cloud/client ./internal/modules/ports_over_relay -run 'TestCatalog'`; verify missing behavior fails.
- [ ] Implement exact-byte authenticated payloads, strict bounded parsing, verified snapshots and durable watermarks. No record supplied by relay may change an existing pin. Preserve stale entries for display while prohibiting new tunnel authorization.
- [ ] Add and implement `TestTraceForgery`, `TestTraceRepeatedService`, `TestTraceEighthAndNinthHop`, `TestTraceExpiryDoesNotKillStream`: signed lineage, repeated pair → 508, eighth allowed, ninth denied, and header/envelope mismatch rejection.
- [ ] Run relevant race tests; review handling of old backups and withheld catalogs, then checkpoint.

### Task 4: Relay admission, pairing, and CLI

**Files:** Create relay package/tests, `cmd/zelport-relay/main.go`, `main_test.go`; update `Makefile` production relay target.

**Interfaces:** `relay.Options` owns access token, limits, logger; `relay.New(opts Options) (*Server, error)`; Server implements `http.Handler` and `Close() error`. Protocol defines typed control requests/responses for the seven operations in the design with correlation IDs and explicit error codes. Relay owns all accepted connections until close.

- [ ] Add `TestRelayRequiresAdmission`, `TestRelayDuplicateRegistration`, `TestRelayPairingCapability`, `TestRelayLimits`: no catalog before confirmation; duplicate UUID cannot replace active registration; join is one-use and bound to exact participants/session; excess work is rejected promptly.
- [ ] Run `go test ./internal/cloud/relay ./cmd/zelport-relay`; confirm failures.
- [ ] Implement single `/v1/ws` endpoint, admitted control channels, bounded in-memory registry, chunked catalog, event hints, 10-second pairing cleanup, relay bridging without E2E termination, and disconnect cleanup.
- [ ] Implement documented CLI flags, exclusive token-file creation, English errors, version, signal shutdown; require valid access-token file even for loopback. Add tests for missing/invalid token, occupied port, shutdown and no secret logging.
- [ ] Run race tests and `go build -o bin/snitzel-relay ./cmd/zelport-relay`; test both explicit public bind policy and default loopback in isolated fixtures, then checkpoint.

### Task 5: Client session lifecycle and service authorization

**Files:** Create remaining client package files/tests; extend protocol stream-open definitions.

**Interfaces:** `client.Options` contains settings, identity, persistence callbacks and registry. `Registry` exposes `Catalog(context.Context) (protocol.Record, error)` and `Open(context.Context, protocol.OpenRequest) (net.Conn, error)`. OpenRequest includes service UUID, source authority and trace. Registry.Open atomically authorizes and registers a provider stream owner before returning its in-memory connection. `client.New(opts Options) (*Client, error)`, `Run(ctx context.Context) error`, `Open(ctx context.Context, ref protocol.Import, request protocol.OpenRequest) (net.Conn, error)`, `Snapshot() Status`, and `Close() error`. The client wraps returned registry connections in the acknowledged encrypted stream protocol.

- [ ] Add `TestClientReconnect`, `TestProviderIsolation`, `TestCanceledOpen`, `TestUnpublishedService`: fake clock asserts capped jitter/backoff and healthy reset; stalled provider cannot block another; cancellation prevents late work; catalog alone cannot authorize a service.
- [ ] Run `go test ./internal/cloud/client`; verify tests fail.
- [ ] Implement admitted outbound channels, catalog refresh/events, per-provider session establishment, Noise/yamux ownership, one request per stream, explicit open acknowledgment, heartbeat, limits and cleanup. Import-only devices still register authenticated identity records.
- [ ] Add `TestClientCloseActiveStreams`, `TestSlowConsumerBounds`, `TestDuplicateIdentity`, `TestGroupChangeClosesSessions`; verify all limits and no unbounded goroutine queues. Sessions cannot outlive client shutdown.
- [ ] Run client/relay race tests together; review concurrency ownership and checkpoint.

### Task 6: Module adapter, imports and revocation

**Files:** Create `internal/modules/ports_over_relay/{registry,proxy,trace}.go`
and their tests. Core contracts are already provided by modularization.

**Interfaces:** The module implements Task 5 Registry. Catalog reads
Host.OwnServices and signs exportable UUID/domain summaries. Open reads the private
publishing snapshot, captures the sharing Scope, and calls Host.Acquire with that
revision and source UUID. It validates expected authority, tracks both ends of an
in-memory connection on the Lease, then serves HTTP using Host.Dispatch. No public
listener or feature-specific imports. ServeRoute proxies cloud imports through
client.Open. Trace middleware is installed once with Host.AddMiddleware.

- [ ] Add `TestCloudDispatchExcludesAdminAndImports`, `TestCloudOriginalAuthority`,
  `TestCloudOriginOptIn`, `TestCloudApplication403`; run module tests to verify
  failures before implementing dispatch.
- [ ] Implement source authority/Origin processing and explicit stream ownership.
  Track all client sessions through Host.Track against the client scope. Publication
  disable commits publish=false with ResetScopes=[sharing]; connection settings
  replacement resets sharing and client. Scope-resource closure completes before API success.
  The core contract's scope reset supplies this behavior; no cloud-specific core API.
- [ ] Add `TestCloudDisableUpgradeRace`, `TestCloudEditClosesOldStreams`,
  `TestCloudFutureOwnService`, `TestCloudStoppedSandbox`: test both sides of commit/
  Acquire races, actual hijacked connections, watch coalescing and no implicit start.
- [ ] Add `TestCloudNoRetryAfterSideEffect`, `TestCloudFailureAfterHeaders`,
  `TestCloudProviderRename`, `TestCloudTwoInstanceLoop`: source UUID never resolves
  by stale domain, response loss never retries, and registered middleware catches
  actual cycles across Local/Remote routes. Keep source-combination tests in integration.
- [ ] Run module/client race tests plus tagged integration tests. Inspect dependencies
  to prove cloud adapter imports no source module; prove no Aux listener appears.

### Task 7: Protected API and Ports over Cloud interface

**Files:** Create `internal/modules/ports_over_relay/api.go`, `api_test.go`,
`assets/module.js`, `assets/style.css` and embed declarations.

**Interfaces:** Module Admin() implements paths relative to
`/api/modules/ports_over_relay/`: GET/PUT settings, POST credentials, POST refresh.
Core applies security before invoking it. Common route endpoints use cloud-owned
ImportSettings validation. State returns only sanitized catalog/status/editor data.
Frontend exports the exact mount contract; Dashboard uses core RouteView summaries.


- [ ] Add `TestCloudAPISecurity`, `TestCloudSecretRedaction`, `TestCloudDomainCollision`, `TestCloudOfflineRename`: exact authority, Origin and CSRF for mutations/reveal, no secrets on GET or errors, collision 409 across all modules, offline edit preserves bindings.
- [ ] Run `go test ./internal/modules/ports_over_relay -run 'TestCloud(API|Secret|Domain|Offline)'`; confirm failures, then implement endpoint validation and atomic updates.
- [ ] Build embedded setup/catalog/import UI, explicit token generation/copy, publishing switch, hostname grouping/UUID suffix, alias edit and opt-in Origin rewrite. Include source service/domain and saved unavailable imports; no hidden automatic import/rename.
- [ ] Extend Dashboard and `#cloud` URL tab selection, 15-second visible-state refresh, and operations/errors using existing CSS/components.
- [ ] Verify in a browser with isolated configs: create/join, duplicate hostname/domain, offline import edit, toggle publish during WS, reload/Back, light/dark and narrow layouts. Run module/API race tests, capture evidence, then checkpoint.

### Task 8: Adversarial protocol and lifecycle integration

**Files:** Create `internal/cloud/integration/protocol_test.go`, `lifecycle_test.go`, `traffic_test.go`.

**Interfaces:** Tests instantiate actual relay/client/core components with temporary persistent configs, injectable clocks and controlled HTTP/HTTPS/WS applications. Malicious relay fixture knows admission secret but never group secret. Source-specific integration files use matching positive build constraints; transport tests need no source module.

- [ ] Write `TestMaliciousRelayCannotSubstitute`, `TestForgedCatalogAndPinnedKey`, `TestDowngradeAndReplay`: modified routing/key/service/version, split handshake, stale records after withdrawal and replayed join/admission must fail without backend requests.
- [ ] Run integration tests and fix implementation defects with regression tests; do not weaken requirements to make the experiment's behavior pass.
- [ ] Write `TestCloudTrafficMatrix`: WS and WSS relay, HTTP methods/bodies/path/query/403, streaming, bidirectional WebSocket, concurrent services, cancellation, Origin off/on and foreign Origin preservation, custom Localhost port.
- [ ] Write `TestCloudLifecycleMatrix`: provider/relay restart, network partition, sleep-like outage, token rotation, retained offline imports, no self-import/republication, disable closes streams even if withdrawal cannot reach relay.
- [ ] Run `go test -race ./internal/cloud/integration` and decoder fuzz smoke tests. Review wire vectors and capture assertions, document remaining review limits, then checkpoint.

### Task 9: Process verification and operating documentation

**Files:** Create `scripts/cloud-demo.py`; update `README.md`, `DESIGN.md`, `PORTS_OVER_CLOUD.md`, `AGENTS.md`, `Makefile`; add `docs/ports-over-cloud-verification.md`.

- [ ] Build a self-cleaning demo with temporary configs, two integrated Snitzel processes, relay and controlled backends on ephemeral ports. Never copy the user's config or kill a process not owned by the demo.
- [ ] Exercise a controlled WSS reverse proxy, both original/renamed imports, restart/revocation, active WS shutdown, and an indirect cycle. Add a stripped-trace case that checks bounded failure/cleanup and records the detection limitation, without claiming a guaranteed 508.
- [ ] Run `go test -race ./...`, `go vet ./...`, `make build-full`, `go build -o bin/snitzel-relay ./cmd/zelport-relay`, then `make check-modules`, `make check`, `make build` and `python3 scripts/cloud-demo.py`. Expect zero exit status throughout.
- [ ] Document CLI setup/token handling, WSS reverse proxy example, import/Origin semantics, identity backup/reset, rotation, platform support actually tested, protocol limits and residual metadata/DoS risks. Mark only verified implemented behavior as implemented.
- [ ] Self-review coverage against every requirements row; obtain code/security-focused review appropriate to the cryptographic changes, resolve findings and rerun affected checks. Record evidence and limitations; do not call this an independent cryptographic audit.
- [ ] Check all relative documentation links and record a scoped final checkpoint. Report completion without restarting the user's existing instance unless separately requested.
