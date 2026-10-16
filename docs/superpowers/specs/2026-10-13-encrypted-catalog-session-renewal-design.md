# Encrypted catalogs and renewable E2E sessions

Status: implemented and locally verified on 2026-10-13, with an independent agent code review. Not independently cryptographically audited. See the [verification report](../../verification/2026-10-13-protocol-v3.md) for evidence and execution decisions.

## Intent and scope

Protocol v3 keeps WS/WSS, relay admission Noise, and yamux. It encrypts catalog
contents against the relay and performs fresh authenticated E2E handshakes inside
long-lived peer connections. Successful renewal preserves yamux streams, including
HTTP streaming and WebSockets, but can briefly delay their data. Transport loss or
failed renewal still closes affected streams; application requests are never replayed.

The user setup remains endpoint, optional relay access token, and mandatory 32-byte
E2E secret. There are no new user switches, trust prompts, TOFU stores, or certificates
to administer. WSS retains ordinary certificate/hostname verification; WS retains
Noise admission. Protected admission is NNpsk2; explicitly open admission is NN.
No downgrade retry is permitted. SHA-256, X25519, and ChaCha20-Poly1305 remain the
Noise suite. FIPS, QUIC, distributed relays, public gateways, roles, and changing
XXpsk3 to IKpsk2 are out of scope.

Confirmed architectural direction: implement over the relay first, while keeping
E2E protection independent of the carrier. The subsequent implementation plan was
approved for inline execution and final review.

## Transport boundary

The carrier supplies a reliable, ordered, full-duplex byte connection with bounded
I/O, cancellation, deadlines, and close semantics. For v3 this is the existing
relay-paired WS/WSS connection, after relay admission. E2E session code consumes
that connection and authenticated peer context; it must not import WebSocket
types or depend on relay URLs, access tokens, HTTP headers, or NAT discovery.
Use the existing net.Conn boundary rather than introducing a transport plugin
framework before there is a second implementation.

Relay discovery, admission, connection pairing, and reconnect belong outside the
E2E record layer. Separate the carrier's pairing context from the logical E2E
identity/session context. Where the initial handshake binds relay pairing context,
pass it as explicitly domain-separated canonical binding data, not as a relay
dependency inside the cipher state machine. A future direct carrier must define
its own authenticated binding rather than reusing an unrelated relay capability.

Catalog sealing/opening operates on bounded byte envelopes independently of their
delivery channel. Authorization, identity pins, rollback protection, and renewal
requirements must not become weaker on a direct or local path.

Future direct connectivity could use a QUIC stream behind the byte-connection
boundary. This alone would not remove head-of-line blocking inside yamux or provide
native per-application QUIC streams. ICE/STUN, hole punching, route selection,
relay fallback, and transparent migration between carriers are separate future
work. Neither QUIC nor a coordinator guarantees connectivity through every NAT.
For this version, carrier failure closes the E2E session and its streams; a new
carrier requires a fresh E2E handshake and must never replay application requests.

## Existing implementation and gaps

- `internal/cloud/protocol/catalog.go`: records are authenticated with a shared
  group HMAC but readable through `PeekRecord`.
- `internal/cloud/relay/server.go`: registration uses group/device identity from
  that record; pairing currently carries both static public keys in `PeerContext`.
- `internal/cloud/client/client.go`: clients establish `secure.Peer`, then give it
  to `yamux.Client` or `yamux.Server`. Idle sessions are pruned after approximately
  60 seconds without active streams; busy sessions have no elapsed lifetime cap.
- `internal/cloud/secure/secure.go`: ciphertext decrypts directly to application
  bytes. Rekey after 1,048,576 records does not introduce fresh DH material.
- `internal/modules/ports_over_relay/registry.go`: sequence and watermark writes
  are durable. Current digests cover the encoded record rather than semantic
  plaintext. A random-nonce encrypted record requires different digest semantics.

## Trust boundaries

The relay may inspect, alter, reorder, suppress, or replay anything it sees. It
must not learn catalog hostnames, domains, ports, service IDs, publication flags,
device UUIDs, static public keys, or E2E secrets from the new wire messages.

It still sees IP addresses, stable opaque group/device routing identifiers,
connection relationships, timing, message sizes, and traffic volume. No padding,
traffic-analysis resistance, or concealment of group membership is promised.
It can deny service or hide an entire device, including on first discovery.

Group members remain mutually trusted: possession of the E2E secret permits
catalog authentication and full group administration. AEAD does not turn shared
secret authentication into per-device signatures or protect against a malicious
member. Existing provider pins and handshake identity checks remain mandatory.
A future public gateway needs a separate authorization design.
Catalog envelopes use keys derived from the long-lived group secret: later
compromise of that secret permits decryption of recorded catalog envelopes.
They do not gain forward secrecy from periodic peer-session renewal.

## Encrypted catalog

### Routing identifiers and wire envelope

Keep the existing group-locator derivation unchanged so local group/import identity
is stable. Derive a separate routing key with HKDF-SHA-256, salt
`snitzel-cloud/v3`, info `catalog-routing`. A device route ID is the lowercase
hex HMAC-SHA-256 of its canonical UUID bytes under that key. The relay cannot
reverse it to a UUID without the group secret; it is deliberately linkable within
the group. It is a routing address, not an authentication credential.

Envelope, encoded as a strict fixed-field JSON array:

`[3, group_locator, device_route_id, nonce_base64url, ciphertext_base64url]`

Both locators are exactly 32 bytes encoded as 64 lowercase hex characters.
Unknown versions, wrong field counts, malformed encodings, and oversized envelopes
are rejected before expensive work. There is no plaintext catalog fallback.

Use XChaCha20-Poly1305 for catalog AEAD, with a fresh random 24-byte nonce from
`crypto/rand` for every encryption. Derive a separate 32-byte per-device key with
HKDF-SHA-256, salt `snitzel-cloud/v3`, info equal to the canonical encoding of
`["catalog-encryption", group_locator, device_route_id]`. The input secret is the
E2E secret, never the access token. XChaCha uses the already pinned x/crypto
package; it is for catalog envelopes, not a change to the Noise cipher suite.
Random generation failure aborts publication. Sequences are not encryption nonces.

Associated data is the canonical encoding of
`["snitzel-cloud/v3/catalog", 3, group_locator, device_route_id]`.

Plaintext contains the complete validated catalog record, including version,
group, actual device UUID, hostname, static public key, sequence, timestamps,
gateway port, publish flag, and sorted services. Use the current canonical array
layout with version 3. Reject disagreement between the authenticated plaintext
and envelope group/device route ID after decryption.

### Limits and lifetime

Keep MaxServices=256, MaxControl=128 KiB for a COMPLETE encoded control message,
and MaxSnapshot=2 MiB for the accumulated encoded catalog envelopes. Base64,
AEAD tag, nonce, envelope, and message overhead all count. Validate the final
serialized registration size before sending it; rejecting too-large catalogs
must preserve last-known usable state. Test exact boundary cases.

The relay retains only one opaque envelope per registered live control connection,
replacing it on publish and deleting it on disconnect. It cannot check plaintext
expiry, service validation, sequence ordering, or membership and must not pretend
to do so. Keep admission, 32-device, queue, connection, and handshake bounds.
Client verification retains 60-second validity and the existing five-second
future-clock allowance. Connection activity timeout still removes dead registrations.

### Replay protection and persistence

Processing order: bounded parse, AEAD verification, plaintext validation and
routing binding, expiry, provider pin and durable sequence/digest checks, then
expose the record. Persist acceptance before making it usable. Persistence failure
must not advance in-memory trust or replace an accepted record.

The digest is SHA-256 of the canonical authenticated plaintext, never ciphertext.
Same sequence plus identical canonical plaintext is idempotent even with a new
nonce. Same sequence with changed content or a smaller sequence is rejected.

Preserve configured routes, secrets, device keys, sequence counters, and provider
pins. Add a digest-version discriminator to stored watermarks; absent means the
existing v2 encoding. Preserve the old sequence floor and require a strictly higher
sequence to replace a v2 watermark with a v3 semantic digest. New publication
already reserves a strictly increasing sequence durably. No reset of rollback
protection is allowed. Keep the existing group locator and local import identity.
Only counter/watermark-only changes may use MetadataOnly.

## Pairing without catalog leakage

Introduce a relay-only route context:

`[3, group_locator, consumer_route_id, provider_route_id, session_nonce]`

Use this for connect/offer/join and relay matching. Control and join capabilities
remain random, connection-bound, and single-use where required. Remove full
`PeerContext` and static keys from ALL relay-visible pairing messages.

Each endpoint resolves opaque route IDs against its decrypted validated catalog,
checks current membership/expiry and expected identity, and constructs the full
peer context locally. Bind BOTH the route context and full identity context into
the E2E handshake prologue. Do not transmit the existing plaintext peer-context
preface containing keys. Unknown or mismatched identities fail closed; a relay
cannot substitute a different peer by editing routing fields.

## Renewable E2E record layer

### Placement and ownership

Keep one stable `net.Conn` facade under yamux. Initial peer authentication and
renewal use Noise XXpsk3 with the existing peer PSK purpose and pinned identities.
A new HandshakeState generates fresh ephemeral keys for every generation.

The E2E connection owns one wire reader, one serialized priority writer, a bounded
application receive queue, and a lifecycle context. Only these owners touch cipher
states and underlying reads/writes. yamux must never receive control records.
Do not call today's blocking `secure.Peer` concurrently on the socket or nest new
secure.Conn wrappers recursively.

Encrypted record payloads have a type byte followed by a type-specific bounded
body: DATA, CREDIT, RENEW_REQUEST, HS1, HS2, HS3, FINISH. Headers needed to interpret
control operations are inside authenticated ciphertext. Keep at most 32 KiB of
plaintext per record including the type; outer length is bounded before allocation.
Epoch numbers are uint64 in fixed network byte order; overflow closes the connection.
Epoch 0 is the initial session. Unknown types and unexpected state transitions fail.

### Backpressure and control progress

Each direction has a 256 KiB application receive-credit window. DATA counts against
credit; control messages do not. Return byte credit only when the facade's Read
actually consumes DATA. CREDIT cannot increase outstanding allowance above the
fixed window; invalid deltas fail. The receiver never has to block its wire reader
on an application queue: exceeding granted credit is a protocol violation.

Keep control buffers bounded (at most 16 queued frames, each at most 4 KiB).
Coalesce credit updates and renewal requests. Prioritize control at DATA record
boundaries, never in the middle of a record. Do not pre-encrypt queued DATA: cipher
selection happens only when the writer emits it. This prevents stale-key frames
from crossing a generation boundary and prevents full application buffers from
blocking handshake progress. Extra credit is deliberately below yamux because
control records must remain processable even when yamux stops consuming.

Close, deadlines, and context cancellation wake credit waiters, queued writers,
and readers. A Write reports only bytes accepted under a bounded ownership policy;
no unbounded copies or silently acknowledged unsent data. SetReadDeadline affects
application reads; internal handshake deadlines are independently enforced and
must not erase a caller's earlier write deadline.

### Renewal state machine

Only the original consumer initiates. A provider crossing its soft threshold sends
one RENEW_REQUEST for the current epoch. Concurrent triggers coalesce; overlapping
handshakes and unexpected epochs are rejected. Handshake flood attempts cannot
create more than one renewal per connection.

1. Consumer stops emitting DATA at a record boundary and emits HS1 using old TX.
2. Provider receives HS1, stops its own DATA at a record boundary, and sends HS2
   using old TX. In-flight old DATA before each marker remains valid and ordered.
3. Consumer receives HS2, verifies it, and emits HS3 using old TX. It derives the
   new cipher pair, changes RX to the new epoch, then changes TX and emits FINISH
   as the first new-key record. Application DATA remains paused.
4. Provider reads HS3 using old RX, verifies the pinned identity, derives new keys,
   changes RX, and verifies the consumer FINISH. It then changes TX and sends its
   FINISH as the first new-key record, after which it may resume DATA.
5. Consumer verifies provider FINISH and resumes DATA. Both discard old cipher
   references and obsolete handshake state; do not claim guaranteed zeroization
   of all copies in Go memory.

The ordered stream defines exact key boundaries; do not try both old and new keys.
FINISH includes the epoch, role, and completed new handshake hash, authenticated
under new keys. Renewal prologue binds protocol, full peer context, original
session binding, previous handshake hash, and next epoch. Preserve the initial
handshake binding explicitly instead of discarding it. Validate current local
identity/group configuration and peer authorization before completing renewal.
Revocation cancels immediately, independently of any renewal timeout.

Control-frame emission also follows the direction barrier: after HS1/HS2, queue
ordinary CREDIT until the corresponding new-key FINISH has been sent. Allow only
the next expected handshake frame on that direction. Existing credit is not reset
at renewal: byte stream ownership and queued data survive, cryptographic counters
are reset only for genuinely new keys.

Timeout, verification error, unexpected frame, or partial write closes the peer
session and all its streams. There is no rollback to old keys after renewal starts.
Symmetric Noise Rekey remains every 1,048,576 encrypted records per direction
within an epoch; both DATA and control records count. It does not reset the epoch
age or traffic budget.

### Proposed policy constants

These are conservative product defaults to validate by tests and measurement,
not security bounds taken from a cryptographic proof:

- Request fresh handshake after 10 minutes OR 1 GiB of encrypted record plaintext
  in either direction, whichever occurs first (control bytes count).
- Hard stop at 15 minutes OR 2 GiB in either direction without completing renewal.
- Renewal exchange timeout: 10 seconds, bounded by remaining hard lifetime.
- Timers use a monotonic clock; the receiver enforces its own hard limits even if
  the peer never requests renewal. Test hooks shorten thresholds deterministically.
- Preserve idle pruning. Success resets generation age/bytes, not application
  deadlines, identities, import state, or authorization.

No configurable UI policy and no lifetime-based reconnect on successful renewal.
An application with a shorter timeout may still observe a failure during a pause.
Do not promise transparent survival of a broken transport.

## Relay admission lifetime

This scope refreshes E2E peer keys, not client-to-relay Noise admission keys.
Admission retains the existing symmetric Rekey and connection-bound lifetime.
Periodically reconnecting admission sockets would break their contained peer
streams. Document this boundary explicitly; do not label the entire stack as
having fresh-DH renewal. A later separately designed admission renewal can reuse
reviewed record-layer concepts, but must not be silently included in this change.

## Versioning and implementation boundaries

Use `/v3/ws`, v3 Hello/prologues, v3 catalog envelope and pairing formats. Reject v2;
no legacy network compatibility or downgrade negotiation. Keep local IDs, build
tags, user configuration, imports, identities, and group secret stable. The
independent PoC module remains a historical v2 experiment, labeled as such; do not
couple its imports or dependencies to production.

Separate catalog envelope encoding from plaintext validation; separate relay route
context from full E2E peer context; separate E2E record framing, renewal state
machine, and net.Conn facade. Keep relay hosting and client independently tagged.
Both standalone and embedded relay use the same updated server implementation.

## Verification and acceptance

- Catalog: randomized encryption of identical plaintext, AEAD tampering, routing
  substitution, wrong group secret, malformed fields, byte limits, duplicate
  sequence/content cases, restart watermarks, write failure, expiry, and pin changes.
- Relay capture: inspect decrypted admission messages and relay-held records;
  assert hostname/domain/port/device UUID/static key/service metadata do not leak
  through registration, pairing, errors, or logs. Verify opaque routing still works.
- Renewal: multiple epochs under bidirectional saturated transfer, long WebSockets,
  HTTP streaming and concurrent yamux streams; exact bytes/order and no stream reopen.
- Run E2E framing/renewal tests on an in-memory full-duplex connection and a local
  TCP connection without WebSocket or a relay, then run the same invariants through
  the WS/WSS relay integration. Verify carrier failure cancels all E2E workers.
- Exercise unread application queues, credit exhaustion, both-side triggers,
  frame boundaries, old symmetric rekey boundary, delayed/lost/invalid handshake
  messages, wrong identities, old epoch replay, Close and all deadlines at every
  transition. Assert bounded queues and no stranded goroutines.
- Enforce hard age/byte limits and failed renewal without request replay. Test
  sharing revocation, route removal, client disable, token changes, and relay stop
  during every renewal phase.
- End-to-end tests with two isolated clients against standalone and embedded
  relays; protected and explicitly open admission; WS and WSS with verification.
- Measure handshake latency, pause duration, throughput and memory before/after
  repeated renewals. No performance or leak-free claim based on unit tests alone.
- Existing race/vet, core build into bin, make check/check-modules/build, frontend
  regressions and browser checks for affected state/error behavior remain required.
- Obtain protocol/state-machine review before production claims. Passing tests
  is not independent cryptographic assurance.

## Alternatives considered

Draining old yamux sessions cannot renew an indefinitely open WebSocket. Resetting
all sockets on a timer breaks the agreed continuity requirement. QUIC and distributed
relay routing expand scope without being required for catalog confidentiality or
fresh peer keys. The implemented record layer preserves the current deployment model
at the cost of explicit control framing, credit management, and renewal states.

## References

- [Current requirements](../../../PORTS_OVER_CLOUD.md)
- [Current architecture](../../../DESIGN.md)
- [Noise specification: channel binding and rekey](https://noiseprotocol.org/noise.html#advanced-features)

Implementation evidence and its limits are recorded in the verification report.
This design does not promise FIPS compliance,
metadata anonymity, protection from malicious group members, or recovery from an
attacker that retains access to an endpoint's secrets and execution state.
