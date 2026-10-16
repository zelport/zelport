# Ports over Relay v3 technical design

**Status: implemented v3 protocol; not an independent cryptographic audit.**
Product requirements are in [PORTS_OVER_CLOUD.md](../PORTS_OVER_CLOUD.md).
See [verification evidence and limits](ports-over-cloud-verification.md) before
interpreting the security requirements as assurance claims. The isolated
experiment is historical feasibility evidence and uses a different protocol.

## Scope and code boundary

The implementation uses the [module architecture](MODULES.md) and its
[normative contracts](MODULE_CONTRACTS.md). Zelport's cloud adapter
lives in `internal/modules/ports_over_relay`, selected by `ports_over_relay`.
Untagged Zelport contains core only. Full distribution uses `make build-full`.
Cloud depends on generic service contracts, never on individual source modules.
Core storage treats cloud settings and import-specific fields as opaque module
JSON; typed fields described below belong to the cloud module. Core must not
import cloud protocol or cryptographic packages. Cloud UI/API/assets likewise
belong to the module. Relay is a separate command unaffected by Zelport tags.

Ship two binaries in the root Go module: `cmd/zelport` and `cmd/zelport-relay`.
Keep the experiment as an independent module without cross-module imports.
Add `internal/cloud/protocol`, `secure`, `relay`, and `client` packages. These must
not import `internal/app` or `internal/web`; application integration supplies a
service registry and HTTP handler. Reuse reviewed concepts, not the experiment's
entire peer implementation.

One developer owns all devices. Each Zelport has one relay/group configuration.
The relay has one shared admission token, no user database, and no provisioning
API. It may route multiple opaque group locators, without treating them as
separate accounts or an authorization boundary. Admission-token holders can see
opaque catalog envelopes and routing identifiers; E2E group membership is
required for catalog decryption and service use.

## Identity, storage, and recovery

Generate UUIDv4 device and service identities using `crypto/rand`. The device
hostname is a label read from the OS; it is not unique or trusted authentication.
Persist an X25519 private key with the device UUID. Bind UUID and public key in
both authenticated catalog records and the live E2E handshake. Pin the public key
when creating an import. An unexpected key for a pinned UUID fails closed.

Use each own route's UUID as its service UUID; do not introduce a second ID for
compatibility with the current opaque route IDs. New routes receive UUIDv4 IDs.
Editing or renaming preserves the UUID; deletion and recreation produces a new
one. Imports store group locator, device UUID, service UUID, and pinned provider
key. Two devices with the same hostname remain distinct.

Introduce configuration version 6 as a clean break. Do not load, migrate, or
maintain compatibility with versions 1–5. Return an explicit unsupported-version
error for older files; do not silently overwrite them. Users can start with a
fresh configuration. The user explicitly accepts existing-configuration
incompatibility; there is no legacy migration work in this implementation.

Keep routes, optional cloud settings, device key, and catalog sequence in the same
owner-only JSON document. Cloud settings contain `enabled`, `relay_url`,
`access_token`, `group_token`, and `publish`; both booleans default false. Imported
routes have module `ports_over_relay`; their settings contain `group`,
`device_id`, `service_id`, `provider_key`, and `rewrite_origin`. Domain and route
UUID belong to the common envelope. Other route-type fields are rejected. Preserve valid
new-format configuration when cloud is disabled or a save fails.

Generate tokens as 32 random bytes encoded as unpadded Base64url. Generate the
E2E secret locally when first setting up a cloud; join another device by copying
only relay URL and the two tokens, never the device private identity. Explicit
local UI actions may reveal/copy a E2E secret; ordinary state responses never
return either token or private keys. Do not log request bodies or credentials.

Do not silently regenerate a missing/corrupt established identity. Restore its
configuration backup or explicitly reset to a new UUID/key and re-import. Cloning
the entire configuration to another active device causes a duplicate-identity
error; it is not a supported enrollment method. No automatic key replacement.

For a lost device, replace the cloud access token on the relay, restart the relay to
close old connections, and replace both tokens on remaining devices. Group-token
changes close all old sessions and clear authenticated catalog caches. Preserve
imports as belonging to their old group until the user removes and re-imports each one after verifying the provider in the
new group. There is no automatic rebind/reset API. Moving only the
relay URL retains group bindings and imports but reconnects all channels.

## Cryptographic boundaries

Use the evaluated library versions as an initial baseline: coder/websocket
v1.8.15, flynn/noise v1.1.0, and hashicorp/yamux v0.1.2. Check advisories and Go
compatibility at implementation time; pin reviewed versions in root go.mod and
go.sum. AGENTS.md documents the added dependencies and exclusion checks. Keep the experimental module independently pinned.

Derive independent 32-byte keys with HKDF-SHA256: input is the relevant 32-byte
secret, salt is the UTF-8 string `snitzel-cloud/v1`, and info is one of the exact
ASCII labels `group-locator`, `peer-psk`, `trace-mac`, or
`relay-admission`. Only the last uses the cloud access token; all others use the group
token. Encode the locator as lowercase hex. Never give relay any other group
derivative. The labels and salt are protocol constants, not configuration.

### Client-to-relay admission

Use one WebSocket endpoint `/v3/ws`; reject browser Origin headers. Accept only
binary frames, disable compression, verify WSS certificates normally, ignore
HTTP_PROXY, and never follow redirects. No token in URLs or HTTP Authorization.

Every WS, including control, consumer-connect, and provider-join sockets, runs
`Noise_NNpsk2_25519_ChaChaPoly_SHA256` with the admission PSK. The bounded initial
hello specifies version, channel purpose, and a fresh 32-byte client nonce. Its
exact bytes and fixed client/relay roles form the prologue. Empty handshake
payloads; after the two handshake messages, the client sends encrypted `ready`
and the relay sends encrypted `ready`. No catalog, registration, allocation of
provider sessions, or other authenticated operation precedes both confirmations.
A captured first handshake message cannot authorize a replayed operation.

All subsequent client-to-relay control and tunnel bytes remain inside this
admission channel. This avoids inventing a bearer-token challenge protocol for
plain WS. Relay knows this layer's keys. The independent peer layer below is what
protects service contents from relay. An access-token holder can impersonate
relay at this layer; it still cannot pass peer authentication without the group
secret and expected device key. WSS remains recommended for public deployment.

### Peer-to-peer E2E

Use `Noise_XXpsk3_25519_ChaChaPoly_SHA256`, persistent device static keys, and the
peer PSK. Endpoints reconstruct the full UUID/key context from decrypted,
validated catalogs. The relay receives only the fixed routing array
`[3, group, consumer_route, provider_route, session]`. Bind both routing and
logical contexts into the peer prologue; do not send a plaintext identity preface.

Each peer verifies its counterpart's actual Noise static key against the
AEAD-authenticated UUID/key binding; the consumer additionally checks its import
pin. Consumers register authenticated device records even when not publishing.
Exchange typed encrypted FINISH confirmations binding epoch 0, role and handshake
hash before opening yamux. Bind each selected
service UUID to an acknowledged stream-open operation inside this channel.
No unauthenticated early application data or version fallback is permitted.

Use ordered length-prefixed Noise records: uint32 big-endian ciphertext length,
at most 32,768 plaintext bytes including a type byte, plus a 16-byte tag.
The peer net.Conn facade keeps a 256 KiB receive-credit window per direction and
at most 16 queued controls, each at most 4 KiB. CREDIT is returned only after an
application read; the wire reader never waits for application consumption.
Only the writer encrypts outgoing records and only the reader decrypts incoming
records. Shared transition state is serialized; socket writes occur outside the
state lock. Controls have priority at record boundaries.

Renewal uses fresh XXpsk3 ephemeral keys, the original session binding, previous
handshake hash, full contexts and next uint64 epoch. The original consumer alone
initiates; providers may request renewal. HS1/HS2/HS3 use old keys with explicit
DATA/CREDIT barriers. Each direction's first new-key record is FINISH, binding
its epoch, role and new handshake hash. There is no old-key retry. The stable
facade preserves yamux streams across successful renewal. Timeout, identity or
protocol errors close the session without application replay.

Soft limits are 10 minutes or 1 GiB per direction; hard limits are 15 minutes or
2 GiB. Control plaintext counts toward byte budgets. Renewal timeout is 10 seconds,
bounded by remaining hard lifetime. Symmetric Rekey remains before each
1,048,576th record in each direction/epoch. It does not reset age/byte budgets.
Fresh-DH renewal applies to peers, not outer admission. The E2E implementation
uses net.Conn without carrier-specific imports; QUIC/direct paths are not included.

These compositions and protocol choices require review and vectors; selecting
Noise is not evidence that this application protocol is secure. See the
[Noise specification](https://noiseprotocol.org/noise.html) and
[HKDF specification](https://www.rfc-editor.org/rfc/rfc5869.html).

## Catalog and relay routing

Each device publishes an encrypted catalog envelope:
`[3, group_locator, device_route_id, nonce_base64url, ciphertext_base64url]`.
Locators are 64 lowercase hex characters; nonce is 24 random bytes. Catalog keys
use HKDF-SHA256 with salt `snitzel-cloud/v3`. Routing uses info `catalog-routing`
and HMAC-SHA256 of the canonical lowercase UUID string's UTF-8 bytes. Encryption
uses info equal to canonical JSON `["catalog-encryption", group, route]`.
XChaCha20-Poly1305 authenticates canonical JSON
`["snitzel-cloud/v3/catalog", 3, group, route]` as associated data.

Plaintext retains the canonical record array of version, group, device UUID,
hostname, public key, sequence, issue/expiry times, gateway port, publication flag
and sorted service triples. Reject malformed/noncanonical fields, wrong routing
binding, expired records and invalid authentication. Complete serialized control
messages must fit 128 KiB, and snapshots 2 MiB. The relay only parses envelopes;
it cannot verify plaintext expiry, membership or service claims. Long-lived group
keys mean recorded catalogs are decryptable after later E2E-secret compromise.

Persist a monotonically increasing sequence before each publication, including
refresh and withdrawal; a persistence error blocks the new record. Refresh every
20 seconds, expire after 60 seconds, tolerate at most 5 seconds future clock
skew. Persist highest accepted sequence per accepted provider (bounded to 1,024
identities), including providers not imported yet; allow the exact same
canonical plaintext at an equal sequence, never conflicting content or lower
sequence. DigestVersion 3 hashes authenticated canonical plaintext, independent
of nonce. Legacy watermarks retain their sequence floor and require a strictly
higher sequence before conversion. Identical acceptance causes no configuration
write; failed persistence changes neither durable nor in-memory trust.
Expired entries may remain displayed as stale but cannot authorize a new tunnel.
Clock-skew and rejected-record errors are visible. Reject individual bad records
without closing unrelated healthy sessions; keep previously accepted records
until their own expiry prevents new authorization. Metadata-only sequence and
watermark commits use strict CAS without invalidating open user editors. This does not prove completeness or prevent
suppression/replay of a still-valid latest-known record by a malicious relay.

Authenticated control operations: `register`, `catalog`, `changed`, `connect`,
`offer`, and `join`. Publication withdrawal is a newer `register` record with
`publish=false` and only the administration service; there is no separate withdraw opcode. Requests carry IDs, responses correlate them;
limit outstanding operations. No public catalog endpoint. Relay is an in-memory
registry and cannot decrypt catalogs. Duplicate active device registrations
are rejected instead of replacing the first connection. Each pending tunnel binds
the requesting control connection, group, both devices, session ID, and a random
one-use join capability. Deliver the capability only over the provider's admitted
control connection. Expire it after 10 seconds; reject duplicates and mismatches.

After pairing, relay bridges the peer Noise bytes inside the two admission
channels. It never runs the peer handshake. A malicious relay or admission-token
holder can disrupt routing; endpoint authentication must make that denial of
service rather than impersonation. Disconnect removes live registration; clients
keep saved imports. Catalog events are hints; clients fetch and verify snapshots
on connect, on events, and every 20 seconds to recover from missed events.

## HTTP dispatch and connection ownership

One peer Noise session contains yamux, with a pool keyed by group and provider
UUID/key. Serialize establishment per provider, never under a global network
lock. Each HTTP request opens a fresh yamux stream, sends a bounded authenticated
service-open envelope, waits for provider authorization, then sends HTTP/1.1.
Do not reuse HTTP streams or automatically retry requests, including Go transport
retries. WebSocket upgrade retains that stream. Stream-open acknowledgments carry
structured errors; unavailable or unpublished service becomes 503 before HTTP
starts. Existing application statuses pass through unchanged. The module registry adapter
tracks in-memory connections through core leases; session resources use the
module-scoped Host.Track contract.

Provider lookup resolves current own route by service UUID on every stream and
again at dispatch. Dispatch directly through Host.Acquire/Host.Dispatch with a tracked Lease and
module-owned sharing scope, never by calling the public gateway or dialing an
address supplied by a peer.
That handler preserves existing sandbox/container/local/remote/SSH behavior.
Administration and cloud imports are ineligible for this application path, including after a concurrent
edit. Hold a short authorization gate while registering stream ownership; do not
hold it through the request. Publication disable takes the same gate, forbids
new streams, cancels and closes all owned streams (including hijacked WebSockets),
then advertises withdrawal. Own route deletion/edit closes its old streams.

At the consumer, set Host to the authenticated source domain and provider gateway
port (omit port 80). Strip untrusted forwarded headers before reconstructing them.
Source route behavior then determines backend Host/TLS settings. Optional Origin
rewrite maps only the exact import origin to this provider origin, including
nondefault ports; it applies to HTTP and WebSocket. Response Location, cookies,
HTML, and application public-URL settings are unchanged. Provider rename with the
same service UUID updates source authority after authenticated catalog refresh;
a stale source authority must produce refresh-required 503, not misroute.

## Lifecycle and bounds

Automatic reconnect uses exponential backoff of 1, 2, 4, 8, 16, then 30 seconds
with up to 20% jitter and a 30-second absolute cap. Reset after 60 seconds of
healthy connectivity. Cancellation interrupts dialing and waiting. UI/manual
refresh must not create parallel reconnect loops. Never replay an HTTP request.
Before headers, transport failure returns 503; after headers, abort the response.
Application WebSocket clients reconnect themselves.

Default bounds (engineering defaults, not product requirements):

| Resource | Bound |
| --- | --- |
| Relay WS connections / simultaneous unauthenticated handshakes | 128 / 16 |
| Registered devices / services per device | 32 / 256 |
| Pending pairings / outstanding control requests per connection | 32 / 16 |
| Peer sessions per client / streams per session / streams per client | 16 / 32 / 128 |
| Handshake / pairing / stream-open | 5 s / 10 s / 5 s |
| Control message / complete catalog snapshot | 128 KiB / 2 MiB, chunked |
| yamux stream receive window / accept backlog | 256 KiB / 32 |
| HTTP request headers / initial response headers | 32 KiB / 30 s |
| Control heartbeat / missing heartbeat deadline | 20 s / 60 s |
| Idle session without active streams | 60 s |
| Blocked individual frame write | 30 s |

Use bounded relay control queues of 64 messages (one complete 32-device response
plus event headroom) and client response queues of 34 messages and close a stalled control connection.
Reject excess work promptly without unbounded goroutines or pending dial queues.
The header timeout is for initial headers only; do not impose a total HTTP body,
stream, SSE, or WebSocket lifetime. Relay copying uses bounded chunks/backpressure.
No unlimited whole-body buffering. Log lifecycle/error codes and UUIDs, never
secrets, payloads, full request URLs, or HTTP headers.

## Loop prevention

Do not export cloud routes or allow self-imports. Carry an authenticated traversal
envelope in reserved `X-Zelport-Cloud-Trace`: group, random request UUID, ordered
(device UUID, service UUID) visits, issued/expiry times, and HMAC with trace key.
Maximum 8 service visits and 4 KiB encoded size. Append at each own-route dispatch;
return 508 on a repeated pair or an attempted ninth visit. Refresh a two-minute
expiry at each authenticated hop; expiry governs arrival, not stream lifetime.

Local browser requests start a trace; a supplied reserved header must verify or
fail with 508, never be silently reset. Carry verified envelopes through cooperating
Local/Remote/SSH and cloud proxy paths and encrypted stream-open metadata. Reject
conflicting envelope/header copies. Identity and trace state must be usable by
own-route handlers even when publishing is disabled, if cloud is configured.

A proxy that strips metadata or crosses a group that cannot authenticate it can
prevent full detection. Do not promise universal loop prevention. Bound streams,
dials, and buffers, and test actual two-instance cycles plus stripped-header cases.

## Relay CLI and administration

`zelport-relay` flags: `--listen` (127.0.0.1:8090), `--access-token-file`,
`--allow-public-access` (false), `--max-connections` (128), `--log-level` (info),
`--version`, and `--help`. Configuration also supports the corresponding
`ZELPORT_RELAY_*` environment variables and a direct `ZELPORT_RELAY_ACCESS_TOKEN`.
Explicit CLI values override their matching environment setting. Token files and
direct tokens are mutually exclusive; any token source conflicts with public
admission. Missing/empty credentials never implicitly enable public admission.
See the [operation guide](../README.md) or `zelport-relay --help` for the complete reference.
`zelport-relay token --out FILE` creates a 0600 token file exclusively; it never
prints the token. HTTP listener may bind a public interface explicitly. No native
TLS in the relay; use WSS via an external reverse proxy. No unprotected administrative
endpoint. Token replacement takes effect on relay restart and closes old sessions.

Zelport API additions, all under existing exact localhost/Origin/CSRF protection:

- `GET /api/modules/ports_over_relay/settings`: sanitized configuration, connection state, device UUID,
  authenticated catalog, and freshness; never trigger network work on a GET.
- `PUT /api/modules/ports_over_relay/settings`: atomic settings update; absent secret fields preserve secrets,
  nonempty replacements must decode to 32 bytes. There is no in-place credential-clearing/reset API.
- `POST /api/modules/ports_over_relay/credentials`: explicit local generate or reveal E2E-secret action;
  protect by CSRF, no-store, and never include cloud access token in ordinary responses.
- `POST /api/modules/ports_over_relay/refresh`: enqueue one bounded catalog refresh.
- Common route save/delete endpoints accept module `ports_over_relay` with strict validation;
  creation verifies current authenticated offer and pins its key. Renaming an
  existing offline import preserves the pin and UUIDs. Collisions return 409.

Ports over Cloud UI has setup, publish switch, catalog grouped by hostname (UUID
suffix distinguishes duplicates), manual import/rename, optional Origin rewrite,
and saved unavailable imports. Dashboard includes cloud routes. Persist `#cloud`
using existing URL tab behavior; poll sanitized state every 15 seconds only when
Dashboard/cloud is visible. Backend connections and reconciliation are independent
of the browser. Editing tokens is explicit, with blank fields meaning unchanged.

## Release evidence

The [verification record](ports-over-cloud-verification.md) tracks evidence and
remaining deployment/assurance limits. Release verification requires race tests, vet, both production builds, protocol negative tests, and an
isolated process demo. Validate WS and WSS behind a controlled reverse proxy,
relay/provider restart, laptop sleep-like disconnect, lost acknowledgments, token
rotation, revocation under load, and actual indirect cycles. Check plaintext
captures for credentials and application contents. Review exact wire vectors,
framing and auth order before claiming security properties. State untested real
Internet/platform conditions explicitly; passing tests are not a security audit.

References: [coder/websocket](https://pkg.go.dev/github.com/coder/websocket),
[flynn/noise](https://github.com/flynn/noise),
[yamux configuration](https://pkg.go.dev/github.com/hashicorp/yamux#Config).

## Administration extension retained from version 2

The filename is retained for existing links. Version 3 changes admission and peer
handshake contexts and uses `/v3/ws`; old clients must upgrade with the relay.
Saved endpoint paths ending in `/v2/ws` remain loadable but always dial `/v3/ws`;
there is no network fallback.
Credential HKDF labels (including the `snitzel-cloud/v1` salt) remain unchanged.
Catalog services are encrypted and authenticated `[id, domain, kind]` triples with `application` or
`administration` kind. Zelport advertises exactly one administration capability,
with its own persisted UUID and canonical `<device-uuid>.zelport.localhost` domain.
Generic protocol registries may omit it, but cannot advertise duplicates.
The 256-service cap includes this capability, leaving 255 application entries.

Administration opens contain a validated external authority and at most 256 unique
service UUID/receiver URL pairs. Only HTTP `.localhost` URLs on the receiving port,
without credentials, query or fragment, and with no path beyond `/`, are accepted.
Application opens reject administration context. The provider receives peer identity
from the Noise session, not the open payload. Client-scope ownership revokes active
administration on disconnect/reconfiguration; ordinary sharing scope does not.

See the [administration design](superpowers/specs/2026-10-12-cloud-administration-design.md)
and [architecture](../DESIGN.md) for persistent aliases, scoped CSRF and UI behavior.
The same protected APIs are available through this trusted remote entry point.
