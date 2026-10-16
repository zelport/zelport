# Ports over Cloud

The client is displayed as **Ports over Relay**. Its module ID, build tag, API
namespace and configuration key are `ports_over_relay`; transport uses protocol v3. The same relay protocol works in LAN or on
the Internet. Optional `relay` hosting adds a server in Settings, independently of
the client; the standalone relay remains available. See [README](README.md#settings-and-relay-hosting).
Neither mode adds mDNS or direct peer connectivity, and neither gives the relay the
E2E secret. Existing group administration trust boundaries continue to apply.


**Status: implemented v3 requirements, not an independent security audit.**
The root application now includes the optional cloud module and a separate relay.
The [technical design](docs/ports-over-cloud-v1.md) records the chosen protocol;
the [verification record](docs/ports-over-cloud-verification.md) identifies actual
evidence and outstanding assurance/deployment limitations. The original
[implementation plan](docs/superpowers/plans/2026-10-11-ports-over-cloud.md) remains
an execution specification, not a claim that every proposed test scenario passed.

For the current application, see [DESIGN.md](DESIGN.md) and [README.md](README.md).
Contribution guidance is in [AGENTS.md](AGENTS.md).

## Repository location

The isolated transport experiment lives in [experiments/ports-over-cloud](../../experiments/ports-over-cloud/README.md)
as an independent Go module. It contains experimental `cmd/relay` and `cmd/peer` implementations and transport
tests, with separate dependency management. See its [protocol](../../experiments/ports-over-cloud/PROTOCOL.md)
and [findings](../../experiments/ports-over-cloud/RESULTS.md). These experimental choices
do not finalize the production protocol or implement the full feature. It must not modify the running Zelport configuration.

The shared protocol and client live in sibling `zelport-sdk-go`; the server and
standalone command live in sibling `zelport-relay`. Gateway integration remains
in `internal/modules/ports_over_relay` and `internal/modules/built_in_relay`. The historical experiment remains separate. See [DESIGN.md](DESIGN.md) for the boundary.

## Purpose and architecture

Ports over Cloud lets one Zelport instance attach another instance's shared HTTP
services to its own local reverse proxy.

```text
Browser -> Zelport B -> Cloud relay -> Zelport A -> Shared service
           localhost:8080
```

The cloud relay is a separate Go binary. Both Zelport instances establish outbound
connections to it. The relay distributes a service catalog and carries tunnel
traffic. The MVP always relays traffic; direct peer-to-peer connectivity is out
of scope. No local Aux port or operating-system VPN interface is required.

The tunnel must support HTTP, WebSocket upgrades, streamed responses, and
concurrent requests. The browser continues using the receiving instance's ordinary
HTTP gateway. E2E protection applies between the Zelport instances, not between
the browser and its local Zelport or between the provider and its application.

## Operating model

This is an open-source, self-hosted private service for one developer using
multiple devices, such as a work desktop, home desktop, and laptop. It is not a
SaaS product. One Zelport instance connects to one relay and one E2E group.
Devices in that group trust one another; user accounts, roles, billing, and
per-device permissions are outside the first release.

The standalone relay CLI requires an access token by default, supplied through a
file or environment variable. Explicit `--allow-public-access` or
`ZELPORT_RELAY_ALLOW_PUBLIC_ACCESS=true` permits token-free admission and cannot
be combined with a token source. Embedded hosting defaults to
requiring one but offers an explicit **Require access token** setting. This token is independent of the E2E secret. The
relay knows the cloud access token, but never the E2E token. Protected admission must not expose a
reusable credential to observers of plain WS. A future external provisioning
service could use a dedicated API; that API is not part of this release.

Embedded hosting may explicitly accept clients with no access token. Empty tokens
select Noise NN with a distinct open-admission prologue; nonempty tokens retain
Noise NNpsk2. There is no retry or negotiation that downgrades a protected attempt.
Open admission encrypts the channel against passive observation, but does not
authenticate the relay and allows any reachable client to consume relay capacity.
An active intermediary can impersonate an open relay and observe relay-visible
metadata. E2E group authentication, provider identity pins, catalog authentication,
and peer encryption remain mandatory. Existing connection/device/handshake limits
apply in both modes. Old clients that require a token cannot use open admission.

## Publishing and importing

Publishing is disabled by default. Enabling it exports all current and future
own configured services, including sandbox, container, local, remote, and SSH
services. It must never export cloud-imported services. Administration is a separate
automatic typed capability while connected, independent of application publication.
Publication is not a health check and must not itself start a sandbox or application.

The receiving instance offers a **Ports over Cloud** tab. Users select services
to import individually. An offered `service-x.localhost` retains that suggested
domain; users may choose another local name. The receiving gateway determines
the port, normally `8080`, independently of the provider's gateway port.

A conflicting domain requires another name. Imports never overwrite existing
routes or silently take their place. Imported routes also appear on Dashboard.

Devices and services have persistent random UUIDs. The hostname is the device
name shown to the user. Each service is identified by its device UUID and service
UUID, independently of hostname changes or duplicate hostnames. Domains are labels, not identities. Local renaming changes neither
provenance nor export restrictions. A removed and recreated service must not
silently acquire the identity of an earlier import merely by reusing its name.

## Lifecycle

- An import persists across application restarts and temporary disconnections.
- An unavailable relay, provider, or usable tunnel produces 503 while preserving
  the import configuration. Display availability separately from configuration.
- After connectivity returns, reconnect and reconcile against authenticated
  provider/service identities; never silently substitute another provider.
- Disabling publication withdraws the offers and closes their active sharing
  tunnels, including long-lived streams and WebSockets.
- Provider-side service removal or loss of sharing authorization must prevent
  further access. A cached catalog cannot override current provider authorization.
- Unavailable imports remain visible until removed by the user. Removing an import
  does not delete the provider's service.
- Preserve application responses, including error statuses. Do not treat a mapped,
  published, or connected service as proof of application health.

Reconnect automatically with increasing delays. Existing requests fail when their
connection is lost; do not automatically replay application requests. Applications
are responsible for reconnecting their WebSockets. New requests work after the
tunnel recovers. A failure after response headers have been sent terminates that
response rather than attempting to replace its status with 503.

Dispatch an imported request using the source service's original domain and the
provider's Localhost port, then apply that service's existing upstream behavior.
Import-level Origin rewriting is optional and disabled by default. When enabled,
rewrite only an Origin matching the receiving service's own origin, for both HTTP
and WebSocket requests. Preserve foreign and missing Origin headers. Do not
rewrite response bodies, redirects, or cookies.

## Group membership and identity

The MVP uses one shared secret token per group of mutually trusting instances.
Generate 32 cryptographically random bytes, represented by approximately 43
Base64url characters without padding. This is a high-entropy secret, not a
user-chosen password or a public group identifier.

Every holder may publish services and access the group's shared services. The
initial model does not provide per-service access control or independent device
revocation. It does not protect members against a malicious holder of the same
E2E secret. Token compromise compromises group membership trust.

Each device UUID is also bound to a persistent cryptographic key. Initial verification
of that identity must be rooted in possession of the shared token delivered
outside the relay. An import remembers its authenticated provider identity and
service ID. Changes to that identity must not be silently accepted as a name match.

The relay must never receive the token, E2E session keys, or derived secrets
sufficient to decrypt traffic, forge authenticated catalog records, or impersonate
a member. Separate group lookup, relay admission, membership authentication, and
E2E encryption purposes. The implemented derivation and protocols are specified in
the [technical design](docs/ports-over-cloud-v1.md).

## Transport

The implemented architecture is Noise over WebSocket, supporting both:

| Transport | Role |
| --- | --- |
| `ws://` over HTTP | No outer TLS; mandatory authenticated Noise protects the E2E tunnel |
| `wss://` over HTTPS | Outer TLS additionally protects each client-to-relay connection |

```text
Zelport A ---- WS/WSS ---- Relay ---- WS/WSS ---- Zelport B
           <--------- authenticated E2E tunnel --------->
```

WSS is the recommended default for public deployment. The relay may listen on
loopback HTTP behind a reverse proxy supporting WebSocket upgrades. Terminating
HTTPS at that proxy must not expose E2E application contents.

TLS is not required for the E2E security property. Both WS and WSS must enforce
the same authenticated Noise protection, including resistance to a malicious
relay. There is no fallback to an unencrypted or unauthenticated application tunnel.

WS exposes the HTTP handshake to network observers. Admission Noise protects
subsequent control messages; catalog AEAD additionally hides catalog contents
from the relay. WSS also protects the HTTP handshake. The relay still observes
opaque stable routing identifiers, IP addresses, pairing relationships, traffic
sizes and timing. It cannot read device UUIDs, keys, hostnames, domains, ports or
service metadata. Catalog keys derive from the long-lived E2E secret; compromise
of that secret permits decryption of previously captured catalogs.

Pinned Go libraries (versions and review limitations are in the technical design):

- [coder/websocket](https://github.com/coder/websocket): WebSocket transport.
- [flynn/noise](https://github.com/flynn/noise): Noise handshakes and encrypted messages.
- [hashicorp/yamux](https://github.com/hashicorp/yamux): multiplexed streams inside
  an authenticated encrypted connection.

These libraries are used by both independently pinned Go modules. Composing them is not
itself a security proof; framing, authentication, identity binding, stream lifecycle,
and limits still require a concrete specification and tests. WireGuard is not the
selected transport. It was considered as an encrypted transport reference, but this
feature does not require a general-purpose IP VPN.

## Relay efficiency considerations (v3)

This section distinguishes implemented admission, catalog caching and catalog
compression optimizations from further recommendations. Measurements are isolated
loopback observations, not WAN or capacity guarantees. It supersedes earlier advice based on the simpler peer
transport. Optimize relay bandwidth, CPU and memory separately.

- **Measure each layer separately.** The relay terminates client-to-relay
  NNpsk2 admission channels and decrypts/re-encrypts their records while bridging
  opaque peer XXpsk3 ciphertext. It does not perform peer DH renewals or run the
  peer credit window/yamux state machines. WSS adds hop TLS processing at its
  termination point. Measure actual relay ingress/egress bytes, record/message
  counts, CPU, allocations, heap, connections and handshake rates. Existing
  peer handshake/renewal/throughput benchmarks use `net.Pipe`; they do not
  measure the complete WebSocket/admission/relay path.
- **Admission framing and buffers are optimized.** The admission data writer
  sends its four-byte length and ciphertext together, without waiting to batch
  other records. It reuses bounded, lazily grown per-connection buffers for
  encryption, incoming ciphertext and decrypted plaintext. The wire format,
  record bounds, rekey schedule, deadlines and authentication remain unchanged.
  Unread plaintext is drained before its storage is reused. At maximum record
  sizes these three buffers have a combined slice capacity of 98,340 bytes
  per admission connection until its object is released; allocator rounding
  and other connection state add to the actual memory footprint.
  Generic `protocol.WriteFrame` still writes header and body separately for
  other callers. Bridge copy buffers remain allocated once per direction
  (32 KiB each). See the [staged benchmark comparison](docs/benchmarks/relay-admission-2026-10-13.md).
- **Keep endpoint optimizations distinct.** The v3 peer facade has reader/writer
  goroutines, typed records, a 256 KiB receive-credit window per direction,
  prioritized controls and fresh-DH renewal barriers. Frame validation currently
  calls a serialization helper that creates a temporary copy; validation without
  serialization and fewer data copies are endpoint optimization candidates.
  Coalescing CREDIT messages could reduce network and relay work, but must not
  stall a sender or delay renewal controls. Do not enlarge windows or introduce
  delayed batching without measuring memory and latency under slow readers.
- **Catalog fetches omit authenticated envelopes already held by the client.**
  Each request may include up to 32 opaque route-ID/ciphertext-digest hints.
  Relay omits byte-identical records while retaining full snapshot bounds;
  it cannot inspect service fields or generate semantic deltas. Clients hint
  only authenticated, accepted, unexpired records. Omissions never advance
  expiry, pins or durable watermarks. Publication remains every 20 seconds and
  records retain their 60-second expiry. The [idle catalog measurements](docs/benchmarks/catalog-idle-2026-10-13.md)
  distinguish increased request bytes from reduced reply bytes. Deploy updated
  relay and clients together: older strict-schema relays reject the new `known`
  request field. Compact provider-authenticated freshness records remain a
  proposal requiring digest binding, expiry, late-join/reconnect recovery and
  revocation semantics. A relay heartbeat cannot replace provider authentication.
- **Catalog compression runs only on endpoints.** Independent DEFLATE records
  reduce encrypted publication size without adding compression work to relay.
  This is implemented, automatic and separate from duplicate-envelope omission.
  See [Catalog compression](#catalog-compression) for format, limits, measured
  savings and deployment requirements.
- **Keep outer WebSocket compression disabled.** Both application tunnels and
  catalog bodies reaching relay are encrypted; compressing their ciphertext is
  not a useful bandwidth optimization. Optional application compression belongs
  before peer encryption, on the endpoints. Application gzip passthrough is now covered by a gateway/relay integration
  test, including fixed-length and flushed responses. Any added compression must bound
  decompression, preserve streaming and avoid sharing compression history across
  services; even per-stream contexts can leak secrets through lengths. Do not
  enable blanket tunnel compression. See [RFC 7692 security considerations](https://www.rfc-editor.org/rfc/rfc7692.html#section-8).
- **Preserve security and topology while measuring.** Peer renewal is already
  implemented and preserves healthy yamux streams; it is not a new optimization
  proposal. Do not remove admission protection, weaken renewal limits or switch
  to QUIC simply to claim lower overhead. P2P could bypass relay payload costs,
  but remains outside the relay-based scope and requires a separate design.

A [loopback HTTP transport baseline](docs/benchmarks/relay-v3-2026-10-13.md) now
covers WS/WSS, admission, peer encryption, yamux and a TCP backend, with relay
process counters. It excludes full gateway/module dispatch and WAN effects;
measured costs do not establish optimization savings.

Idle/catalog traffic has been measured and duplicate encrypted snapshot
transfer reduced. Those idle measurements predate catalog compression; they do
not measure the combined savings of both changes. Evaluate a separate authenticated
freshness protocol only with an explicit security/lifecycle design. Separate-host/WAN benchmarking remains
pending and is excluded from the current optimization task.
Consider additional application-payload compression only if measurements justify
its security and implementation costs. Catalog compression is already implemented.
Admission measurements do not establish savings for these remaining proposals.

## Catalog compression

**Implemented and automatic.** Zelport compresses each canonical catalog record
independently using Go's standard-library `compress/flate` at `BestSpeed`. There
is no new dependency or configuration switch. This is raw DEFLATE, not a gzip
wrapper. The compressed representation is used only when smaller than the raw
representation; both include a one-byte encoding marker.

```text
Publish: canonical record -> encoding byte + raw/DEFLATE data -> catalog AEAD -> relay
Receive: relay -> authenticate/decrypt -> decode with size limit -> validate canonical record
```

The marker is inside the XChaCha20-Poly1305 authenticated ciphertext: `0` means
raw canonical bytes and `1` means DEFLATE. Authentication succeeds before any
decompression. Decoded records are limited to 128 KiB; unknown encoding markers,
malformed or truncated DEFLATE, trailing compressed data and excessive expansion
are rejected. Both encoding paths return the same canonical bytes for existing
identity, freshness and replay validation. Ciphertext-digest cache hints continue
to identify the complete encrypted envelope.

Each publication uses a fresh compression context, with no shared history between
records, devices or application streams. Compression does not hide record sizes
or eliminate length-based information leakage. The relay cannot decompress the
catalog because it does not possess the E2E secret. Publication every 20 seconds,
60-second expiry, provider pins, durable watermarks and live authorization remain
unchanged.

### Measurements and scope

Isolated fixtures with random service UUIDs and `project-N.localhost` domains
produced these complete JSON/base64 encrypted envelope sizes:

| Services in a record | Before compression | With compression | Reduction |
| --- | ---: | ---: | ---: |
| 1 | 558 bytes | 481 bytes | 13.8% |
| 25 | 3,042 bytes | 1,278 bytes | 58.0% |
| 100 | 10,842 bytes | 3,662 bytes | 66.2% |
| 256 | 27,274 bytes | 8,689 bytes | 68.1% |

These are per-record sizes, not kB/min or a full-path relay benchmark. They exclude
admission, WS/WSS and network overhead; real names and identifiers affect results.
For a few personal devices and services, the absolute bandwidth saving is small.
These measurements do not establish an application-throughput improvement.

Compression and decompression consume endpoint CPU and temporary allocations.
The current implementation creates a fresh flate writer per publication; it does
not pool compressors. The [measurement report](docs/benchmarks/catalog-compression-2026-10-13.md)
records endpoint timings, allocations, raw output and reproduction commands. These
measurements do not establish total relay CPU savings or a security review.

Application payloads are not compressed by this feature. Existing application gzip
responses pass through unchanged; outer WebSocket compression remains disabled.

### Deployment

Update every Zelport peer to the new catalog payload format. Legacy payloads
without an encoding marker are rejected; there is no compatibility decoder or
format negotiation. The outer v3 relay envelope is unchanged, so compression
alone requires no relay update. The earlier `known` catalog-cache request field
still requires a relay that supports that field.

## Hardening and transport considerations

These are considerations for future development, not a committed implementation
plan, measured performance claims, or a completed security review. WS/WSS remains
the implemented transport.

- **Fixed cryptographic suite:** retain X25519 / ChaCha20-Poly1305 / SHA-256
  with fixed handshake patterns per protocol version, no algorithm negotiation,
  and no downgrade fallback. SHA-256 is a supported Noise choice; replacing it
  with BLAKE2s alone has no demonstrated security benefit for this design.
- **Key lifecycle:** peer sessions renew with fresh XXpsk3 handshakes after
  10 minutes or 1 GiB per direction, with hard limits of 15 minutes/2 GiB and
  a 10-second renewal timeout. Successful renewal retains healthy streams;
  failures close them without replay. Symmetric Rekey every 1,048,576 records
  remains within each epoch. Outer admission only has symmetric Rekey and
  connection-bound lifetime; it does not have periodic fresh-DH renewal.
- **Relay hardening:** extend existing concurrency limits and timeouts with
  handshake flood protection, fair capacity allocation, precise protocol state
  transitions, adversarial tests, and independent security review. WireGuard's
  key lifecycle and denial-of-service defenses are useful references; adopting
  its UDP transport or kernel implementation is not a prerequisite.
- **Capacity:** Noise imposes no fixed number of concurrent tunnels. The current
  topology needs approximately `N + 2T` relay connections for `N` connected
  Zelport instances and `T` established E2E tunnels, plus transient connections
  during setup. Capacity depends on socket limits, memory, CPU, bandwidth, and
  handshake rate. Configured tunnel limits and theoretical counts are not
  throughput benchmarks; a tunnel can carry multiple application streams.
- **QUIC trade-offs:** higher throughput is not guaranteed. Independent QUIC
  streams can reduce cross-stream delivery blocking under packet loss, while
  still sharing connection congestion control. Putting the entire existing
  Noise/yamux tunnel into one QUIC stream preserves blocking between its inner
  streams. Exploiting independent streams requires a design for authenticated
  E2E framing and multiplexing that preserves protection against a malicious
  relay. QUIC performance also depends on CPU costs, UDP buffers, and platform
  optimizations; it cannot increase the relay's physical link capacity.
- **Measure before changing transport:** retain WS/WSS until comparative
  measurements justify a change. Benchmark large transfers, concurrent streams,
  and networks with latency and packet loss, recording throughput, response
  latency, CPU, and memory. Distinguish the outer tunnel transport from
  application WebSockets carried inside it: replacing the former need not
  remove support for the latter.

See the Noise, WireGuard protocol, RFC 9000, and quic-go sources in
[Primary references](#primary-references).

## Threat model and security requirements

Assume an attacker can control DNS or the network path, replace the relay, control
catalog responses, alter routing, replay recorded messages, and drop or delay
traffic. The attacker does not initially possess the E2E secret or control a
trusted endpoint. The security properties below must hold even over plain WS.

### Endpoint authentication

Encryption alone is insufficient. Complete an authenticated E2E handshake before
sending application requests. A fake relay must not be able to terminate separate
trusted sessions with each instance and read or alter their traffic.

Authenticate the expected provider identity and bind service selection to that
verified connection. The provider must open only a currently shared service by
its stable ID. Never let relay-supplied routing parameters grant arbitrary access
to an arbitrary local address or port. Administration uses only the dedicated
authenticated typed capability, never application dispatch.

### Catalog authenticity

Catalog records must be authenticated and bind the provider identity, stable
service ID, advertised data, and record version. Reject forged and modified
records. A signature is insufficient if the relay can also substitute the trusted
verification key. Establish the key's trust through token-rooted membership
verification, not solely through the catalog that supplies it.

A valid catalog record is an offer, not permanent authorization. Before opening
a service, verify the live provider and its current sharing authorization inside
the authenticated tunnel. Domain names must never override these identity checks.

### Replay, downgrade, and credentials

Bind protocol version, negotiated parameters, endpoint roles, and relevant session
context to authenticated exchanges. Require freshness for control operations and
bind them to the intended session. No fallback may bypass Noise or identity checks.

Do not place the E2E secret or E2E secrets in HTTP headers, URLs, plaintext
registration messages, diagnostics, or logs. A public group identifier must not
be confused with proof of membership. Catalog versioning and freshness must account
for restarts, replay, and newly joining clients; a version number alone is not proof
that the relay supplied the latest record.

### Limits of the guarantees

A malicious relay can suppress services, partition the group, delay responses,
or disconnect clients. Encryption cannot guarantee availability. A first-time
client cannot establish catalog completeness or absolute freshness from an
untrusted relay alone. Old authentic records must never enable unverified
redirection or access after provider-side authorization has been withdrawn.

The relay may observe operational metadata and opaque routing identifiers. E2E does not protect against endpoint compromise or a malicious
member holding the shared token. Resource limits, bounded buffers, handshake
limits, and reconnect backoff are needed to contain denial-of-service effects.

## Loop prevention

1. Never republish cloud imports, even after renaming them.
2. Reject imports of the instance's own services using stable identity rather
   than domain comparison.
3. Restrict tunnel access to explicit provider-side shared service IDs; never
   expose generic arbitrary-port forwarding or reach administration through application opens.
4. Track traversal through cooperating Zelport instances and impose a hop limit
   to detect indirect cycles introduced through Local Ports/Remote HTTP.
5. Return **508 Loop Detected** when a loop is detected.

Traversal information requires defined trust and propagation rules. Do not treat
an arbitrary browser-supplied header as authenticated traversal history. The
implemented mechanism and eight-hop limit are specified in the technical design.

An intermediary that strips traversal information can defeat distributed detection.
Bound connection establishment and resource consumption to limit the consequences;
do not break legitimate long-lived streams with an arbitrary short total lifetime.
Zelport implements these checks for cooperating routes. It does not guarantee
general cross-proxy loop detection when intermediaries remove the trace.

## Engineering choices and implementation boundary

[Compile-time modularization](docs/MODULES.md) provides the cloud integration boundary.
Ports over Cloud is selected with `ports_over_relay`; untagged Zelport builds only
the common core. The module imports no source-feature implementations and includes
its own administration API and frontend assets. Full releases enumerate all
implemented feature tags through `make build-full`.

Configuration version 6 intentionally breaks the previous format. No legacy support
or migration is required; use UUID route IDs directly as service UUIDs in the new
format. Atomic persistence and preservation on failed saves still apply.

The [technical design](docs/ports-over-cloud-v1.md) records concrete choices
for admission, Noise, key derivation, identity binding, catalog freshness,
multiplexing, lifecycle, traversal metadata, resource limits, configuration, and
API. These are engineering choices derived from the agreed requirements, not
a completed cryptographic review.

The [implementation plan](docs/superpowers/plans/2026-10-11-ports-over-cloud.md)
orders implementation and negative tests. The transport experiment has already
validated basic feasibility; integrated verification is now recorded separately.

## Validation scenarios

| Area | Required scenarios |
| --- | --- |
| Import behavior | Two instances; original and renamed domains; custom receiving port; collisions; stable IDs across restart; no name-based replacement |
| Publication | Disabled by default; all current/future own services when enabled; imports excluded; administration is a separate automatic capability; no automatic sandbox/application start |
| Traffic | HTTP methods/bodies/query, application errors, concurrent connections, WebSocket upgrade and bidirectional traffic, streaming and cancellation |
| Transport | WS and WSS; reverse-proxy deployment; Noise mandatory in both; no plaintext fallback; no local Aux listener |
| Identity | Fake relay; attacker-provided keys; wrong token; substituted provider or service; rejected identity changes |
| Catalog | Modified/forged records, stale authentic records, record replay, deletion/withdrawal, first-time client behavior, withheld records |
| Catalog compression | Canonical round trips; incompressible raw fallback; exact decoded-size limit and over-limit expansion; malformed/truncated/trailing data; unknown encoding and legacy-payload rejection |
| Protocol | Tampered ciphertext, replayed control operations, downgrade attempts, handshake failures, session-context mismatch |
| Lifecycle | Provider/relay disconnect and restart, reconnect, persistent unavailable imports, publication disabled during active streams, no blind request replay |
| Loops | Self-import, renamed import republication, indirect Local/Remote cycles, forged or removed traversal information, hop-limit handling and 508 |
| Isolation | No arbitrary backend-port access, no administration access through application opens, no secrets in URLs/headers/logs, no relay-readable application contents |
| Resource handling | Slow consumers, bounded buffers, connection limits, failed-handshake floods, cleanup and backoff without breaking legitimate streams |

Tests must distinguish inability to decrypt/forge from inability to deny service.
Traffic inspection and negative tests supplement, but do not replace, protocol
review. A malicious E2E-secret holder is outside the agreed outsider threat model.

## Primary references

- [Noise Protocol Framework](https://noiseprotocol.org/noise.html): handshake
  patterns, PSKs, authenticated context, and security considerations.
- [WireGuard conceptual overview](https://www.wireguard.com/): the IP-over-UDP
  alternative considered during design.
- [WireGuard protocol and cryptography](https://www.wireguard.com/protocol/):
  fixed suite, key renewal, key confirmation, and denial-of-service defenses.
- [RFC 9000: QUIC](https://www.rfc-editor.org/rfc/rfc9000.html): stream delivery,
  flow control, and transport behavior.
- [quic-go transport documentation](https://quic-go.net/docs/quic/transport/):
  UDP socket handling, buffers, and platform optimizations.
- [coder/websocket repository](https://github.com/coder/websocket)
- [flynn/noise repository](https://github.com/flynn/noise)
- [hashicorp/yamux repository](https://github.com/hashicorp/yamux)

## Automatic remote administration

Protocol v3 preserves full same-group administration through
`<sanitized-hostname>.zelport.localhost` and `<device-uuid>.zelport.localhost`, on the
receiving Localhost port. Membership grants full management, including configuration
changes and sandbox actions. Publishing off does not disable this capability;
cloud disconnect does. Reserved names cannot be user routes or manual imports.
Persistent pins and alias ownership prevent silent name reassignment. Collisions
and known unavailable peers return 503, unknown aliases 404. The provider validates
authenticated kind, service UUID, peer identity and external authority before calling
the trusted bridge. Exact Origin and scoped CSRF protection remain mandatory.

See the [approved administration design](docs/superpowers/specs/2026-10-12-cloud-administration-design.md)
for alias rules and lifecycle. This implementation is not an independent security audit.
