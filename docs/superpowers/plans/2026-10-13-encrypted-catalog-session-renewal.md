# Encrypted Catalog and Session Renewal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not start production changes until plan review and execution-method selection.

**Goal:** Hide catalog contents from relays and renew authenticated E2E keys without reopening healthy application streams.

**Architecture:** WS/WSS admission and relay pairing produce an ordered full-duplex byte connection. A carrier-independent E2E record layer exposes net.Conn to unchanged yamux, with bounded flow control and fresh XXpsk3 handshakes. Catalog envelopes are encrypted independently of their delivery channel.

**Tech Stack:** Go standard library; existing pinned coder/websocket, flynn/noise, hashicorp/yamux, and x/crypto. No new runtime dependencies or frontend build system.

**Spec:** [Approved design](../specs/2026-10-13-encrypted-catalog-session-renewal-design.md).

## Execution status

Tasks 1–7 are implemented and locally verified; see the
[verification report](../../verification/2026-10-13-protocol-v3.md).
The original granular checklist below is retained as the proposed test recipe,
not as a claim that every named test and manual scenario ran verbatim. Actual
coverage, substituted timer/integration approaches, browser coverage limits and
review fixes are recorded in that report and the execution ledger.

- [x] Catalog envelope and AEAD
- [x] Durable semantic watermarks
- [x] v3 catalog delivery and opaque pairing
- [x] Carrier-independent typed peer connection
- [x] Fresh-handshake renewal state machine
- [x] yamux and lifecycle integration
- [x] Local measurements, documentation, build matrix and independent final review

## Global Constraints

- Protocol v3 only, `/v3/ws`; no wire compatibility or downgrade to v2.
- Preserve user configuration, identities, imports, provider pins, E2E/access secrets,
  group-locator derivation, sequence counters and old watermark sequence floors.
- WS/WSS, yamux, NNpsk2/explicit NN admission, XXpsk3 peers; SHA-256/X25519/ChaChaPoly.
- Catalog: per-device XChaCha20-Poly1305, random 24-byte nonce, authenticated routing
  envelope, semantic plaintext digest; shared-group trust is unchanged.
- MaxControl=128 KiB for the whole serialized message, MaxSnapshot=2 MiB,
  MaxServices=256; preserve existing relay/client capacity limits.
- E2E record plaintext <=32768 bytes including type; receive window 256 KiB per
  direction; control queue <=16 frames, each <=4 KiB.
- Soft renewal at 10 minutes or 1 GiB; hard limit 15 minutes or 2 GiB per direction;
  renewal timeout 10 seconds; symmetric Rekey every 1,048,576 records per direction.
- No application replay, no stream restart on successful renewal, immediate
  revocation, no claim of fresh-DH renewal for the outer admission layer.
- E2E implementation has no WebSocket/relay-specific imports; use net.Conn.
- No QUIC, direct paths, NAT traversal, migration, distributed relay, or FIPS work.
- Tests use isolated configurations/backends, not live user services or credentials.
- English documentation, errors and comments. Root/nested modules stay independent.
- Tests/builds use GOWORK=off; output binaries into bin/. Inspect applicable AGENTS.md
  and workspace changes before execution; establish isolation at execution time.

## Review Focus

1. Different ciphertext for the same catalog content must not trigger rollback rejection (Tasks 1–2).
2. Hidden plaintext identifiers must not leak through pairing or error paths (Task 3).
3. An application that stops reading must not prevent control/renewal progress (Tasks 4–5).
4. Writes, deadline changes, and Close during key switch must not deadlock or silently lose accepted bytes (Tasks 4–6).
5. Restart or failed persistence must not silently replace trusted identity or lower a watermark (Tasks 2, 6).

## File responsibilities

- `internal/cloud/protocol/catalog.go`: canonical plaintext validation/digest input.
- New `internal/cloud/protocol/catalog_envelope.go`: bounded envelope parser/encoder;
  relay-visible data only, no group secrets.
- New `internal/cloud/secure/catalog.go`: routing/key derivation and catalog AEAD.
- `internal/cloud/protocol/wire.go`, `types.go`: v3 routing context/messages.
- `internal/cloud/relay/server.go`: opaque registration and route-only pairing.
- `internal/cloud/client/client.go`: encrypted publish/accept and logical peer lookup.
- `internal/modules/ports_over_relay/{config,registry}.go`: durable semantic watermarks.
- New `internal/cloud/secure/{peer_handshake,peer_records,peer_conn,peer_renewal}.go`:
  handshake binding; frame codec; carrier facade/credit; renewal state machine.
- Existing `secure.go`: common derivation and admission; retain existing admission
  Conn behavior. Do not force peer-specific framing onto admission connections.
- Existing `internal/cloud/{secure,client,relay,integration}` tests: targeted additions
  beside their owners, plus integration and benchmarks.
- README/DESIGN/PORTS_OVER_CLOUD and module contracts: final implemented behavior.

## Task 1: Catalog envelope and AEAD

**Files:** modify protocol/catalog.go; create protocol/catalog_envelope.go,
secure/catalog.go, protocol/catalog_envelope_test.go, secure/catalog_test.go.

**Interfaces:**
- `protocol.CanonicalRecord(r Record) ([]byte, error)` and
  `protocol.ParseCanonicalRecord(raw []byte, now time.Time) (Record, error)`.
- `protocol.CatalogEnvelope {Version int; Group, RouteID string; Nonce, Ciphertext []byte}`;
  `EncodeCatalogEnvelope(e CatalogEnvelope) ([]byte,error)`;
  `ParseCatalogEnvelope(raw []byte) (CatalogEnvelope,error)`.
- `secure.CatalogRouteID(secret []byte, deviceID string) (string,error)`;
  `secure.SealCatalog(r protocol.Record, secret []byte) ([]byte,error)`;
  `secure.OpenCatalog(raw, secret []byte, now time.Time) (protocol.Record, []byte, error)`
  returns validated record and canonical plaintext. No relay package imports secure
  catalog decryption or receives the E2E secret.

- [ ] Write TestCatalogEnvelopeRoundTrip, TestCatalogRandomizedSemanticIdentity,
  TestCatalogEnvelopeTamper, TestCatalogRoutingBinding and TestCatalogBounds.
  Assert identical plaintext has different nonce/ciphertext but identical canonical
  digest; all fields listed private in the spec are absent from the decoded envelope.
  Assert wrong secret, changed group/route, bad nonce length and noncanonical encodings fail.
- [ ] Run `GOWORK=off go test ./internal/cloud/protocol ./internal/cloud/secure -run Catalog`;
  confirm failures expose missing interfaces/behavior, not broken fixtures.
- [ ] Implement exact spec array layouts, purpose strings, HKDF salts, AEAD and
  canonical sort. Preserve existing v2 functions temporarily only until Task 3
  switches production call sites; do not expose dual wire support in the release.
  The new canonical helpers validate version 3 explicitly, independently of the
  active wire Version constant, which remains 2 until Task 3.
- [ ] Test random-source failure using an unexported injectable sealing helper
  taking io.Reader; production always uses crypto/rand.Reader.
- [ ] Repeat the focused command and race variant; all pass. Record a scoped diff
  checkpoint; never stage unrelated workspace files.

## Task 2: Durable semantic watermarks

**Files:** modify client/client.go; modules/ports_over_relay/config.go, registry.go;
add cases to api_test.go and client_test.go.

**Interfaces:** retain `AcceptRecord func(protocol.Record, []byte) error`, but the
byte argument becomes authenticated canonical plaintext when v3 is wired in Task 3.
Add `Watermark.DigestVersion int json:"digest_version,omitempty"`; zero denotes
legacy v2 digest, 3 denotes canonical v3 digest. Accept no other version.

- [ ] Add TestCatalogSemanticWatermarkRestart, TestLegacyWatermarkFloor,
  TestCatalogPersistenceFailurePreservesTrust and TestCatalogProviderPinPreserved.
  Assert same sequence/same plaintext idempotence, changed-content rejection,
  lower-sequence rejection, equal legacy floor rejection, and higher-sequence
  transition retaining existing identities/imports.
- [ ] Run tagged tests and confirm intended failures:
  `GOWORK=off go test -tags ports_over_relay ./internal/modules/ports_over_relay ./internal/cloud/client -run 'Catalog|Watermark'`.
- [ ] Implement version-aware comparison and canonical digest persistence; validate
  configurations without clearing peers, pins or counters. Keep MetadataOnly solely
  for actual counter/watermark-only changes. Persistence precedes usable cache updates.
  Exercise v3 acceptance through a separate internal helper until Task 3 switches
  its production caller and authenticated-byte contract together. Existing v2
  callers must continue writing legacy digests; never label signed-envelope bytes
  as DigestVersion 3 during this intermediate checkpoint.
- [ ] Update positional Watermark literals and fixture semantics explicitly.
  Add concurrent acceptance tests so a delayed lower sequence cannot overwrite a newer
  in-memory or persisted value; serialize the acceptance transaction if necessary.
- [ ] Run the tagged command with `-race`; all cases pass. Checkpoint only this task.

## Task 3: Switch catalog delivery and pairing to v3

**Files:** protocol/{types,wire}.go; transport/websocket.go; client/client.go;
relay/server.go; secure/secure.go; related unit/integration fixtures.

**Interfaces:**
- `protocol.RelayRoute {Version int; Group, ConsumerRoute, ProviderRoute, Session string}`;
  methods `Bytes() ([]byte,error)` and `ParseRelayRoute([]byte) (RelayRoute,error)`.
- Relay-facing Message uses `Route *RelayRoute` rather than `Peer *PeerContext`.
- Full PeerContext remains client-side/E2E. Update
  `secure.Peer(ctx context.Context, raw net.Conn, initiator bool, identity protocol.Identity,
  secret []byte, peer protocol.PeerContext, route protocol.RelayRoute) (net.Conn,error)`.
  Task 5 adds a lifecycle authorization callback through PeerOptions.

- [ ] Add TestV3OpaqueRegistrationAndPairing, TestV3RejectsV2,
  TestPairingRouteSubstitution, TestCatalogWireSize and TestRelayCannotReadCatalog.
  Capture decrypted admission messages on a controlled server: no device UUID,
  hostname, static key, service identifier/domain/port or publication flag appears
  in registration, offers, joins, or error text.
- [ ] Run `GOWORK=off go test ./internal/cloud/... -run 'V3|Pairing|CatalogWire|CannotRead'`;
  verify failures before changes.
- [ ] Wire Task 1 encryption and Task 2 acceptance into publish/catalog; introduce
  validated route contexts; relay parses only opaque envelopes. Resolve route IDs
  from validated client catalogs; check pins and expiry before pairing.
- [ ] Remove plaintext PeerContext preface. Bind canonical route+full identity
  context in the v3 peer prologue. Set Version=3, `/v3/ws` and all domain-separated
  v3 prologues; retain group-locator derivation. Remove production PeekRecord and
  signed-only catalog paths. Update existing golden vectors intentionally.
- [ ] Check full encoded control size before enqueue/send, including escaping and
  Base64; preserve last-known good publication on failure. Test queue/snapshot bounds,
  duplicate registration, wrong access token, open admission and no downgrade.
- [ ] Run `GOWORK=off go test -race ./internal/cloud/...` and tagged module tests;
  both standalone/shared embedded server paths must pass. Checkpoint the v3 catalog milestone.

## Task 4: Carrier-independent typed peer connection

**Files:** create secure/peer_records.go, peer_conn.go and peer_conn_test.go;
extract peer handshake helpers into peer_handshake.go without changing admission framing.

**Interfaces:** internal `peerFrame {kind byte; body []byte}` with fixed type constants
DATA/CREDIT/RENEW_REQUEST/HS1/HS2/HS3/FINISH; encode/decode bounded plaintext payloads.
Internal `peerConn` implements net.Conn with a sole reader and serialized priority
writer. `Close() error` cancels and joins owned workers idempotently.
`newPeerConn(ctx context.Context, raw net.Conn, tx, rx *noise.CipherState,
state peerHandshakeState, options peerOptions) (*peerConn,error)` remains unexported.
Define peerHandshakeState in peer_handshake.go: original/current [32]byte binding,
uint64 epoch, initiator role, identity, secret, peer/route contexts.

- [ ] Add TestPeerCreditWindow, TestPeerBlockedReaderControlProgress,
  TestPeerPartialWrite, TestPeerDeadlinesAndClose and TestPeerCarrierIndependence.
  Assert 256 KiB cap, invalid/overflow credit rejection, <=16 control frames,
  <=4 KiB control body, exact byte order and no control bytes delivered to Read.
- [ ] Run `GOWORK=off go test ./internal/cloud/secure -run 'PeerCredit|PeerBlocked|PeerPartial|PeerDeadlines|PeerCarrier'`;
  confirm expected failures.
- [ ] Implement typed framing and bounded application queues. Allocate credit only
  to DATA; return consumed credit, coalesce increments, prioritize controls at record
  boundaries. Preserve old admission Conn independently. Do not reset credit on rekey.
- [ ] Implement facade deadlines without overwriting earlier caller write deadlines;
  cancellation wakes ALL readers, writers and credit waiters. Account for partial
  raw writes as fatal, propagate errors and never replay bytes after connection failure.
- [ ] Run tests over net.Pipe and localhost TCP with deliberately short reads/writes,
  blocked consumers and concurrent Close. Run race tests; record worker/queue bounds.
  Checkpoint the independently testable facade before enabling it in production.

## Task 5: Fresh-handshake renewal state machine

**Files:** create secure/peer_renewal.go, peer_renewal_test.go;
modify peer_handshake.go, peer_conn.go and secure.Peer call sites.

**Interfaces:**
- `secure.PeerOptions {Authorize func(protocol.PeerContext) error}`;
  final Peer signature is Task 3 signature plus `options PeerOptions`.
- Unexported `peerOptions` carries policy, clock/timer abstraction and random source
  for deterministic tests; production factory supplies fixed spec policy and crypto/rand.
- Internal `renewalPolicy` specifies time/byte soft/hard thresholds, timeout, and
  symmetric rekey record interval with the exact Global Constraints defaults.
- Authorize verifies current identity/group and peer eligibility; callback must
  not call back into peerConn or run under its state/writer lock. Client shutdown
  and effective configuration changes cancel the lifecycle context immediately.

- [ ] Write table-driven TestRenewalTransitions for every legal transition and
  rejection of unexpected types/epochs/roles. Test initial epoch 0, uint64 overflow,
  identity changes and prologue/channel-binding substitution.
- [ ] Confirm failures with `GOWORK=off go test ./internal/cloud/secure -run Renewal`.
- [ ] Implement HS1/HS2/HS3 old-key barriers and new-key FINISH exactly as specified;
  new HandshakeState per epoch, fixed consumer initiator, coalesced provider request,
  no key trial/fallback and no application DATA between barrier and confirmation.
  Preserve old DATA queued before markers; no pre-encryption of queued writes.
- [ ] Add TestRenewalSaturatedDuplex, TestRenewalUnreadApplication,
  TestRenewalHardLimits, TestRenewalCloseEveryTransition and TestRenewalSymmetricBoundary.
  Assert exact data, same facade, independent per-direction counters, unchanged
  credit, automatic multiple renewals and prompt cancellation at each handshake phase.
- [ ] Add timing tests using a monotonic fake clock; test writes crossing soft/hard
  byte boundaries, timers while idle/blocked, earliest deadline wins, and no reset
  of lifetime budgets by symmetric Rekey. Test late/duplicate FINISH and delayed peer.
- [ ] Run `GOWORK=off go test -race ./internal/cloud/secure`; add bounded fuzz targets
  for frame/state parsing. Checkpoint only after deterministic failure tests pass.

## Task 6: Integrate with yamux and lifecycle

**Files:** client/client.go, secure/secure.go; client/client_test.go;
new integration/session_renewal_test.go; existing cloud_test.go,
administration_test.go, relay_runtime_test.go; relevant module lifecycle tests.

**Interfaces:** client establish/offer use final secure.Peer and PeerOptions;
yamux still receives net.Conn. No transport imports enter E2E files.
Use an unexported client test construction path to inject reduced renewal policy;
no production flag, environment switch or global mutable test policy.

- [ ] Add TestSessionRenewalPreservesStreams: concurrent HTTP streaming, WebSocket
  echo and multiple application streams retain IDs and exact payload order across
  at least three renewals, in both initiating roles.
- [ ] Add TestRenewalRevocation and TestRenewalTransportLossNoReplay: disable sharing,
  remove service, disconnect client, stop relay or change secret during renewal;
  active tunnels close and post-failure mutating requests are never replayed.
- [ ] Confirm failing integration tests before wiring peerConn into Peer.
- [ ] Wire initial XXpsk3 result/binding into the new facade, including typed initial
  key confirmation. Preserve admission ready/framing independently. Add authorization
  callbacks using current validated peer identities/configuration; preserve lifecycle
  cancellation, session slots, idle pruning and request deadlines.
- [ ] Run isolated standalone and embedded relay scenarios, protected/open admission,
  WS and WSS with trusted test CA. Keep wrong-token/wrong-secret, metadata secrecy,
  replay persistence and full group administration tests active.
- [ ] Run `GOWORK=off go test -race -tags ports_over_relay,relay ./internal/cloud/... ./internal/modules/ports_over_relay ./internal/modules/built_in_relay`.
  Assert all workers and slots return after disconnect; checkpoint integration.

## Task 7: Resource measurements, documentation and release verification

**Files:** new secure/peer_benchmark_test.go; docs/verification/2026-10-13-protocol-v3.md;
README.md, DESIGN.md, PORTS_OVER_CLOUD.md, docs/ports-over-cloud-v1.md;
experiments/ports-over-cloud/README.md (historical status only); affected UI copy/tests.

- [ ] Add BenchmarkPeerHandshake, BenchmarkPeerRenewal and BenchmarkPeerThroughput;
  measure allocs, throughput and maximum observed application pause separately.
  Record Go version, OS/CPU, concurrent peers/streams and test policy. Do not equate
  microbenchmarks with WAN behavior or deterministic Go zeroization.
- [ ] Run sustained bounded local transfer with repeated renewal and disconnect cycles;
  sample heap/goroutine/queue sizes after warmup and after quiescence. Investigate
  continued growth; distinguish retained RSS from reachable live allocations.
- [ ] Update implemented documentation only after verification. Explain opaque
  metadata, shared-secret catalog history exposure, relay-admission lifetime,
  v2 rejection, unsupported transparent path migration and failure semantics.
  Preserve the independent PoC as historical v2; no cross-module imports/replace.
- [ ] Run `go test -race ./...`, `go vet ./...`, `go build -o bin/snitzel-core ./cmd/zelport`,
  `make check`, `make check-modules`, `make build`, existing Node frontend tests and
  new relevant tests. Record actual outputs and any platform/network coverage gaps.
- [ ] Browser-check isolated client/server status, wrong version/error reporting,
  reconnect, catalog import, remote administration and unchanged settings drafts.
  Do not modify the user's running configuration during verification.
- [ ] Review whole diff against spec and five Review Focus items; obtain independent
  review of key boundaries, credit liveness, nonce handling, metadata leakage and
  persistence ordering before claiming completion. No protocol safety claim solely
  from green tests. Update spec status only to reflect achieved evidence.

## Execution and checkpoints

The catalog and renewal milestones share v3 context and identity contracts; execute
Tasks 1–6 in order rather than parallelizing mutations to shared protocol files.
Each task follows red/green tests and a scoped diff review. Use scoped commits when
repository instructions permit; never create a repository-wide initial commit or
stage pre-existing work merely to satisfy a checkpoint. Task 7 verifies the final
combination. A failing liveness or key-boundary test is a blocker, not a reason to
relax deadlines or silently retain old keys.

Plan self-review: every spec section is assigned above; constants and signatures
are shared explicitly, the transport boundary is exercised without WebSocket,
and admission refresh, direct paths and migration remain excluded.
