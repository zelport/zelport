# Cloud Administration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for inline execution, or superpowers:subagent-driven-development if the user selects delegated execution. Track steps with the checkboxes below.

**Goal:** Automatically expose full administration of authenticated cloud peers through hostname and UUID aliases under `snitzel.localhost`.

**Architecture:** Add a typed built-in service to the existing authenticated catalog and stream-open protocol. Route reserved aliases through an optional core administration bridge; retain the existing Noise/WebSocket/yamux transport and keep ordinary publishing separate.

**Tech Stack:** Go, existing pinned cloud libraries, embedded HTML/CSS/JavaScript.

**Spec:** [Approved design](../specs/2026-10-12-cloud-administration-design.md).

## Execution status

Implementation and isolated verification are complete; see the
[verification record](../../ports-over-cloud-verification.md) for actual coverage and
limits. Deployment steps remain pending coordinated relay and external-client upgrade.
Generic protocol registries may omit administration; real Snitzel advertises exactly
one. This is the recorded execution refinement to task 1.

## Global constraints

- English code, UI, errors, and documentation; conversation may remain Czech.
- Full remote administration is automatic only within the authenticated E2E group.
- Preserve route/device UUIDs, private keys, E2E secrets, and existing configuration.
- No arbitrary localhost proxy, extra listener, second transport, manual admin import, or self-import.
- Reserve `snitzel.localhost` and its subdomains in every build.
- Local administration retains its exact localhost authority and existing safeguards.
- A publishing toggle affects application sharing, not administration; cloud disable revokes both.
- Test with temporary configurations and controlled backends. Do not modify live services for tests.
- Run root race tests, vet, full/core builds, and the compile-time module matrix.

## Review focus

- A malicious ordinary application must not turn a forged header or CSRF token into remote administration (tasks 2, 4).
- Duplicate or renamed hosts must not silently transfer a browser origin between devices (task 3).
- Different receiving/provider ports must not create links to unrelated local applications (task 5).
- A remote settings mutation that disconnects its own tunnel must persist once without automatic retry (task 4).
- Unsupported peers and exhausted metadata limits must fail explicitly without resetting identities (tasks 1, 3, 6).

## File responsibilities

- `internal/cloud/protocol`: versioned catalog, service kinds, and stream-open context.
- `internal/cloud/client`: authenticated peer context propagation and typed service checks.
- `internal/feature/administration.go` (new): generic bridge interfaces and request context types.
- `internal/app/administration.go` (new): bridge wiring and reserved-namespace routing.
- `internal/web/remote.go` (new): trusted remote request entry point and scoped CSRF.
- `internal/modules/ports_over_relay/administration.go` (new): built-in service authorization, alias dispatch, lifecycle.
- `internal/modules/ports_over_relay/aliases.go` (new): bounded persistent identity/alias ownership.
- Existing module config/store/API/registry and embedded frontend files integrate these units.

## Task 1: Versioned built-in service protocol

**Interfaces:** Extend `protocol.Service` with `Kind` (`application` or `administration`); extend `protocol.OpenRequest` with `Kind` and optional `Admin *AdminOpen`. `AdminOpen` holds `Authority` and a bounded list of `ServiceLink{ServiceID, URL}`. Change `client.Registry.Open` to `Open(context.Context, protocol.PeerContext, protocol.OpenRequest) (net.Conn, error)`; peer context comes from the authenticated handshake, never stream JSON.

- [ ] Add failing protocol tests for both kinds, missing/unknown kinds, two admin services, malformed external authority, oversized link lists, unpublished applications, and wrong wire versions. Administration may appear when `Publish=false`; applications may not.
- [x] Set production protocol version to 2. Encode service triples `[id, domain, kind]` in the signed catalog. Exactly one administration service is required, using the provider UUID domain as its canonical domain. Reject application services in the reserved namespace.
- [x] Change admission and peer prologue/version markers and relay path to `/v2/ws`; update CLI URL validation, transport, fixtures, and tests. Keep existing credential derivation labels and group locator derivation unchanged; do not rotate secrets. Do not modify the isolated historical experiment.
- [x] Add `AdminServiceID` to cloud settings. Allocate a UUID atomically before first v2 connection when absent; keep existing identity, counters, routes, and credentials. This is an effective settings write, not MetadataOnly. Validate its uniqueness against application route IDs.
- [x] Pass the verified peer context through the provider accept loop into `Registry.Open`. Validate service kind as well as ID and provider key on both ends before the stream is acknowledged. Update client lookup/authority resolution so `Publish=false` blocks applications only; administration still requires a fresh authenticated capability record.
- [x] Bound links to 256, unique service UUIDs, and HTTP localhost-domain URLs with no credentials, query, fragment, or path except `/`. Keep the existing overall control-frame limit. No links for non-admin opens.
- [x] Run `go test -race ./internal/cloud/... ./internal/modules/ports_over_relay/...`; update existing fixtures to v2 while adding explicit v1 rejection tests.

## Task 2: Narrow administration bridge and scoped CSRF

**Interfaces:** Add `feature.RemoteAdminContext{ConsumerID, ProviderID, Authority string; ServiceLinks map[string]string}` and `feature.AdministrationProvider` with `ServeAdministration(http.ResponseWriter, *http.Request) bool`. Add optional `feature.AdministrationHost` with `DispatchAdministration(http.ResponseWriter, *http.Request, feature.RemoteAdminContext) error`. Core registers the handler through `Manager.SetAdministrationHandler(func(http.ResponseWriter, *http.Request, feature.RemoteAdminContext))`; CLI registers `web.Server.ServeRemoteAdministration` before module startup.

- [ ] Add failing web/core tests for reserved-domain saves and loads, unchanged local Host/Origin checks, forged HTTP headers, ordinary-route bypass attempts, and token reuse across remote/local origins and consumers.
- [x] Implement reserved namespace validation in generic route configuration. Existing conflicting configuration fails with a domain-specific error and remains untouched.
- [x] In manager gateway dispatch, consult the optional administration provider only for the reserved namespace. Unknown reserved aliases return 404; ordinary routes never reach the admin bridge. Cloud-excluded builds have no provider.
- [x] Refactor the internal web admin handler to accept explicit expected origin/token/context. Keep local entry unchanged. Remote entry accepts only the typed bridge call; ignore forwarded context headers.
- [x] Derive remote CSRF tokens as HMAC-SHA256 keyed by the process CSRF secret over JSON `["remote-admin/v1", consumerID, providerID, origin]`. Use constant-time comparison and exactly one matching Origin for mutations. Retain CSP, no-store, body limits, and existing module API dispatch.
- [x] Include an optional remote administration context in `/api/state`, without replacing provider runtime facts. No remote context is accepted from an HTTP header or ordinary request body.
- [x] Run `go test -race ./internal/web ./internal/app ./internal/feature ./cmd/zelport` and confirm the core build has no cloud dependency.

## Task 3: Persistent peer ownership and automatic aliases

**Interfaces:** `HostnameLabel(string) string` returns a valid label or empty. `Module.resolveAdministration(host string) (AdminTarget, error)` returns provider UUID/key, admin service UUID, and availability from verified records and persisted ownership. `AdminTarget` never selects a provider by hostname alone.

- [ ] Add failing alias tests for dotted/mixed-case names, non-ASCII characters, empty/overlong labels, UUID-shaped names, duplicate names, rename, disconnect, restart, key substitution, and group changes.
- [x] Persist peer key pins and alias-owner sets keyed by group, with at most 1024 peers and 4096 total alias-owner bindings. Count UUID and hostname bindings; never silently evict. Invalid records/capacity failures leave earlier state usable and produce visible warnings.
- [x] Lowercase hostnames, replace each character outside `[a-z0-9-]` with `-`, trim edge hyphens, and omit labels longer than 63 or matching canonical UUID syntax. UUID aliases use the authenticated device UUID.
- [x] On first verified v2 record, persist its pin and aliases before activating them. Retain all previous hostname ownership claims; conflicting names resolve to 503 for every claimant. A renamed old alias remains unavailable, never redirected. Use current authenticated records only for availability.
- [x] Known peers without a usable live record resolve to 503, unknown aliases to 404. Self aliases are not registered. Disabled cloud retains current-group unavailable aliases; group changes use only the new group's alias space.
- [x] Validate external Host with the exact receiving port, normalizing DNS case only and treating default HTTP port 80 consistently. Do not accept arbitrary ports or suffix lookalikes.
- [x] Run `go test -race ./internal/modules/ports_over_relay/...` including persistence failure and metadata-bound tests.

## Task 4: Authenticated admin HTTP dispatch and revocation

**Interfaces:** `Module.ServeAdministration(w, r) bool` implements the optional provider. Provider `Open` branches on authenticated kind/ID; admin dispatch uses `feature.AdministrationHost`, ordinary dispatch retains existing route leases.

- [x] Add integration tests that GET state and mutate a controlled local-port route remotely, using both aliases and different gateway ports. Assert only the provider configuration changes.
- [x] Advertise the built-in admin service on every enabled connection, regardless of `Publish`; retain ordinary application publication rules. Do not include it in own-service exports, editable routes, or manual-import APIs.
- [x] Before proxying a browser mutation, check exact receiving Origin. Bind external authority and matched local service URLs into the encrypted admin open. Preserve browser Host/Origin through the tunnel; provider validates the authority against its own UUID/current hostname and calls the trusted bridge.
- [x] Construct provider request context from the verified peer UUID and validated encrypted open metadata. Ordinary service opens carrying admin context, wrong service kinds, or wrong IDs are rejected.
- [x] Track both pipe ends and upgraded connections under the existing client scope; retain one HTTP request per stream and current stream limits. Do not tie admin streams to the ordinary sharing scope. Cloud disable/reconfigure closes admin streams synchronously.
- [ ] Add tests for publication off, publishing toggled during active admin traffic, cloud disable, forged Origin/Host/headers, unsupported kinds, failed handshakes, and canceled connections. For a mutation disabling its own cloud, allow response loss but assert exactly one durable commit and no automatic request retry.
- [x] Exclude chained admin access: a remotely dispatched admin UI cannot ask its provider to serve as a receiver for another peer. Reserved alias routing is not invoked by the provider's internal admin handler.
- [x] Run the cloud integration tests with race detection and confirm ordinary HTTP errors, streaming, WebSockets, and application sharing revocation still pass.

## Task 5: Remote UI identity and correct local links

**Interfaces:** Extend `/api/state` with `remote_admin` containing managed hostname/UUID, external authority, and service URL map from the typed context. A shared frontend `serviceURL(route)` returns the local normal URL or a verified remote-context mapping; absent remote mappings return null.

- [x] Update `internal/web/assets/app.js`, `ui.js`, and `index.html` to show the managed identity and remote-administration indicator. Keep all API and asset requests same-origin and relative.
- [x] Preserve provider `port` and runtime/config facts; label provider port as such in remote mode. Use a separate external authority for the browser endpoint and never rewrite configuration ports for presentation.
- [x] Use `serviceURL` for dashboard and generic service-list links. The receiver constructs its map from imports matching the pinned provider, group, and service UUID; if several imports match, choose the lexicographically first domain. Unmapped services are plain text with `Not connected on this device`.
- [x] Add a peer administration list to the local cloud tab with both available aliases and explicit unavailable/conflict states, even when a peer publishes no applications. Do not offer Edit, Remove, or Import actions for built-in admin services.
- [x] In remote mode hide peer administration links; avoid links that would chain through the provider. Keep configuration controls and existing provider-side terminal actions available. Explain the execution location in remote mode.
- [x] Replace the current claim that administration is excluded with accurate cloud-connection versus ordinary-sharing copy.
- [ ] Browser-test two isolated instances: both aliases, remote mutation, locally renamed application import, unmapped service, different ports, peer offline, error display, refresh, and tab retention. Verify a false local same-name application is never linked.

## Task 6: Verification, documentation, and controlled rollout

- [x] Update AGENTS.md, DESIGN.md, PORTS_OVER_CLOUD.md, README.md, `docs/ports-over-cloud-v1.md`, module contracts, and verification notes. Clearly distinguish the built-in admin exception from forbidden arbitrary administration forwarding; mark the technical protocol document as v2 despite its historical filename.
- [x] Run `make check`, `make check-modules`, `make build-full`, and `make build-relay`. Confirm no new third-party dependencies and no cloud imports in core/excluded builds.
- [ ] Run isolated multi-process relay/client verification, including v1 rejection, relay restart, ordinary publication off, all CSRF negative cases, collision persistence, and active-stream revocation. Record actual results and limitations.
- [x] Review the complete change against the approved spec and the five Review Focus cases before deploying. Preserve existing uncommitted work; never stage the repository wholesale.
- [ ] Back up live configuration with owner-only permissions before adding the admin UUID. Check for reserved-domain conflicts without printing secrets. Stop and report conflicts rather than rewriting routes.
- [ ] Coordinate the incompatible protocol rollout: do not replace a live relay while an external v1 client still needs it without informing the user and arranging the client update. Provide the new full Snitzel binary and exact restart command for the second machine.
- [ ] After the coordinated update, verify runtime config path, unchanged device identity and own routes, cloud connection, and remote UI aliases. Do not execute test mutations against the user's live routes.

## Self-review

Coverage checked against all approved design sections: address ownership (task 3),
built-in service/module boundary (tasks 1–2), Host/Origin/CSRF (tasks 2 and 4),
UI context and links (task 5), verification and rollout (task 6). Every Review Focus
case has an owning test step. No implementation code or live configuration has
been changed by preparing this plan.

## Execution choice

Recommended: inline implementation in this chat. These tasks share protocol,
identity, lifecycle, and administration interfaces; one implementation sequence
avoids competing changes at those boundaries. A separate final review should
focus on origin handling, peer binding, and revocation after the complete feature
passes its isolated integration tests.
