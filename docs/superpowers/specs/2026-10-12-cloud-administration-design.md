# Automatic administration over Ports over Cloud

Status: proposed implementation design. This is not implemented functionality or a
completed security review. It supersedes the blanket administration exclusion only
for the authenticated built-in administration service described here.

## Agreed behavior

Every cloud-enabled Snitzel automatically offers its full administration UI to
other authenticated members of the same E2E group. This uses the existing relay,
Noise sessions, and multiplexed HTTP streams. It requires no manually created
service or import and is independent of publishing ordinary services.

The receiving instance exposes two aliases on its own gateway port:

- `<sanitized-hostname>.snitzel.localhost:<receiving-port>`
- `<device-UUID>.snitzel.localhost:<receiving-port>`

For example, `desktop.example.test` becomes `desktop-example-test.snitzel.localhost:8080`.
Local administration remains at `localhost:<local-port>`. Actions execute on the
managed machine; opening a terminal opens it there. Full administration includes
existing credential-management actions, subject to the same explicit UI actions
and CSRF protection as local administration.

## Address ownership and lifecycle

Reserve `snitzel.localhost` and all its subdomains for administration in all
builds. Reject ordinary routes in that namespace on save and configuration load
with a clear English error; do not overwrite or silently remove existing routes.
The bare reserved domain is not an additional administration endpoint.

Lowercase the full hostname, replace dots and other characters outside ASCII
letters, digits, and hyphens with hyphens, and trim leading/trailing hyphens.
If the result is empty or longer than 63 characters, omit the hostname alias
and retain the UUID alias. UUID-shaped labels are reserved for device identities.

Bind aliases to verified device UUIDs and pinned provider keys, not names alone.
Persist bounded peer/alias ownership metadata alongside the existing catalog
watermarks. A hostname alias that collides between identities is unavailable for
all conflicting identities; expose the UUID links and show the conflict. Do not
silently reassign an old alias to a different device when its owner disconnects.
An authenticated hostname change withdraws the old alias from active routing but
retains its ownership record. Exclude the receiving instance itself.

Known unavailable aliases return 503 without opening a tunnel. Unknown aliases
return 404. Disabling cloud disconnects remote administration and closes its active
streams; remembered aliases remain unavailable. Changing the E2E group discards
that group's active resolution context. Ordinary publishing being disabled must
not revoke administration streams. All metadata remains bounded; exhaustion
rejects additional aliases visibly rather than evicting ownership silently.

## Built-in service and module boundary

Represent administration as a typed built-in cloud service with its own persistent
random UUID and a protocol discriminator distinguishing it from application
services. Generate that UUID with initial cloud identity setup, or atomically when
an existing identity first enables the capability. It cannot be edited, deleted,
republished, manually imported, or used to select arbitrary host ports.

Authenticate the capability and service identity in catalog records. Extend the
strict catalog encoding and live stream-open authorization to distinguish the
built-in service from ordinary services. Version the changed wire protocol; reject
incompatible peers explicitly without fallback or interpreting ordinary service
records as administration. Ordinary `publish` rules still apply to applications.

Expose a narrow optional administration bridge through the generic module/host
boundary. The CLI wires the web administration handler into that bridge before
module startup. Core does not import cloud packages. Cloud-disabled builds gain
no remote administration handler, catalog discovery, or transport dependency.

Provider dispatch calls the administration handler through this authenticated
bridge, never by proxying arbitrary traffic to `localhost`. The provider validates
its own enabled state, requested service UUID, service kind, and peer authorization
before acknowledging a stream. Track administration connections under the cloud
connection lifecycle, separately from the ordinary sharing scope. Existing
session and stream resource limits apply to both kinds.

## Host, Origin, and CSRF

The receiver resolves only exact registered aliases at its configured gateway
port. Before forwarding a mutation, require exactly one Origin matching that
browser-facing authority; reject absent, foreign, duplicate, or malformed values.
Do not accept arbitrary browser-supplied forwarded-authority headers.

Bind the external authority and authenticated consumer identity to the encrypted
stream-open context. Provider dispatch validates the authority against its UUID
or current sanitized hostname and enters the dedicated administration handler with
trusted request context. Such context must not be constructible from HTTP headers
on the public gateway or through an ordinary cloud service.

The administration handler uses that bound origin for its exact Origin check and
issues a CSRF token scoped to remote administration, consumer identity, and origin.
Derive it with HMAC from the process CSRF secret and an unambiguous encoded context;
never reuse the local administration token remotely. Check tokens in constant time.
Both aliases therefore work independently without relaxing local Host checks.
Preserve no-store, CSP, framing restrictions, request limits, and English errors.

The tunnel remains protected against an unauthenticated relay or forged catalog.
Authenticated group members have full administrative trust; this feature does not
introduce per-device roles or a second login system.

## UI context and links

Show the managed hostname and UUID and a visible remote-administration indicator.
Keep API calls and assets relative to the current UI origin. Keep the provider's
configuration and runtime port truthful; represent the receiver's browser-facing
port separately rather than rewriting the provider's state.

On the receiving instance, Ports over Cloud lists each known peer with its hostname
and UUID administration URLs even when it publishes no ordinary services. Automatic
administration endpoints do not appear as editable user routes or application
imports. Sharing text must explain that administration remains available while
cloud is connected even when ordinary publication is off.

When viewing remote administration, application names link only to a verified
matching import on the receiving instance, using that import's chosen alias and
receiving port. Otherwise render the name as text with an explanation that it is
not connected locally. Never construct a link that accidentally opens an unrelated
local service. Keep edits directed at the provider. Suppress navigation links to
third-party peer administrations inside the remote UI; peer navigation belongs to
the receiving instance's cloud tab and must not chain administration tunnels.

## Verification and rollout

Use isolated configurations and two or more instances. Verify:

- Both aliases reach the correct remote UI and support protected configuration
  changes, relative assets, module APIs, and mismatched provider/receiver ports.
- Administration works with ordinary publishing disabled; ordinary services stay
  inaccessible. Publishing changes do not terminate administration streams.
- Wrong group secrets, forged catalogs, provider-key changes, ordinary service
  substitution, untrusted headers, foreign/missing Origin, and tokens copied
  between aliases or local/remote UI are rejected.
- UUID/name collisions, invalid/long hostnames, hostname changes, stale catalogs,
  disconnects, restarts, and disabled cloud never route to a different identity.
- No self-import, administration republication, arbitrary-port access, or chained
  remote administration; unknown aliases return 404 and known unavailable ones 503.
- Remote UI service links use matching local imports only. All mutations affect
  the provider; opening a terminal remains an action on the provider machine.
- Core and cloud-excluded builds retain their dependency isolation. Run race tests,
  vet, the module build matrix, and full builds; verify the browser flows manually.

Update AGENTS.md, DESIGN.md, PORTS_OVER_CLOUD.md, the protocol documentation, and
module contracts to describe the narrow administration exception. Deploy compatible
relay and clients together after isolated verification. Preserve live configuration,
route UUIDs, device keys, and E2E credentials; do not reset identities to adopt the
new built-in service.
