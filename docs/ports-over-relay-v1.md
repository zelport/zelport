# Relay Protocol v1

This is the normative protocol contract used by Zelport App `v0.10.0`,
`zelport-sdk-go` v0.10.0 and Zelport Relay v0.10.0. Implementations accept only
the identities and formats defined here.

## Protocol identity

- Wire version: `1`
- WebSocket endpoint: `/v1/ws`
- Base cryptographic context: `zelport-relay/v1`
- Catalog context: `zelport-relay/v1/catalog`
- Client-to-relay context prefix: `zelport-relay/v1/client-relay/`
- Client-to-relay open prefix: `zelport-relay/v1/client-relay/open/`
- Consumer-to-provider prefix: `zelport-relay/v1/consumer-provider/`
- Session-renewal prefix: `zelport-relay/v1/renewal/`

Relay URLs may have an empty path, `/`, or `/v1/ws`; clients always dial
`/v1/ws`. Other paths or versions are rejected. There is no negotiation,
fallback, alias or redirect.

## Roles and identifiers

Each client has a persistent signing/handshake identity and belongs to one
group. A relay authenticates admitted clients and routes opaque protocol frames.
Providers publish route records; consumers request peer streams for records
they have accepted.

Group, device, route and session identifiers have fixed validated encodings and
bounded lengths. Hostnames are normalized before signing or comparison. Unknown
fields, invalid identifiers, duplicate identities and values outside protocol
limits fail validation.

## Cryptographic separation

Relay admission and peer protection use distinct keys and contexts. The relay
access token admits a client to a relay but does not authenticate a peer or
decrypt the group catalog. Group credentials derive catalog protection and
peer-handshake material through the exact contexts above.

Catalog envelopes authenticate protocol version, group, route identity,
provider identity, sequence and validity window. Peer handshakes bind consumer,
provider, route and session context. Key renewal uses a separate derivation
context and never reuses handshake material.

Implementations pin peer identity. A changed or conflicting identity is an
error that requires an explicit new trust decision, not automatic repair.

## Catalog

A provider emits encrypted authenticated records for explicitly published
routes. Records are monotonic per provider and expire. Consumers reject wrong
versions, groups, signatures, identities, stale sequences, expired records,
future validity beyond permitted skew and conflicting route ownership.

Accepted sequence/digest watermarks persist with digest version `1`. Missing or
different protocol markers are configuration errors. A consumer never mutates
an accepted record to make it valid.

Catalog compression is transport-only. Authentication covers the canonical
uncompressed value, and decompression is bounded before parsing. `known` hints
reduce unchanged responses but do not weaken freshness validation.

## Peer streams

The consumer opens a relay-routed stream naming the accepted provider and route.
Consumer and provider complete an authenticated peer handshake before any HTTP
bytes flow. The provider maps the authenticated route directly through the App
host contract; it does not call the public listener.

HTTP requests and responses retain streaming semantics, status codes and
WebSocket upgrades. Connection, frame, header, catalog and concurrent-session
limits are enforced before allocation or decompression. Cancellation, route
replacement, peer revocation and shutdown close all associated streams.

## Remote administration

Remote administration is available only between authenticated group members and
uses a reserved derived hostname under `*.zelport.localhost`. The consumer sets
the authenticated source authority; the provider dispatches through the typed
administration bridge. Forwarded headers never grant access.

Requests carry an authenticated `X-Zelport-Relay-Trace` envelope with a bounded
hop list and unique request identifier. Repeated identities, repeated request
identifiers, invalid ordering or the local identity already appearing in the
path are rejected to prevent loops.

One group is one mutually trusted administration boundary. The protocol does
not provide tenant isolation inside a group.

## Relay server

Zelport Relay and Built-in Relay expose only the relay endpoint on their relay
listener. Admission is either an exact configured token or an explicit public
admission mode. Public admission changes only relay access; peer credentials and
identity checks remain required.

The server applies bounded handshakes, connections, queues, frames and idle
periods. It does not expose Zelport App administration on the relay listener.

## Persistence and recovery

Configuration schema v1 stores the required relay protocol marker `1`, group
material, device identity, peer pins, imports and watermarks. Secrets are written
owner-only. Corrupt, missing or unsupported protocol state is rejected with a
precise error.

There is no automatic identity reset or credential conversion. Recovery means
creating fresh group/device material and reenrolling peers.

## Required verification

Implementations test version/path rejection, exact cryptographic contexts,
admission separation, catalog authenticity and replay rejection, peer pinning,
renewal, limits, revocation, loop prevention, streaming/WebSocket behavior and
resource cleanup. Cross-component tests run the SDK client against both
standalone Zelport Relay and Built-in Relay.
