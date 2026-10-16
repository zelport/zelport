# Ports over Cloud v1 verification

Verified on macOS with Go 1.27.1 on October 11, 2026, using temporary configurations,
controlled applications and owned loopback processes. The user's running Zelport
and real configuration were not changed. This is implementation evidence, not an
independent cryptographic audit or proof of security under all attacks.

## Delivered scope

The root module now builds `zelport-relay` and the optional `ports_over_relay`
Zelport module. Full builds include all six modules. Cloud owns its settings,
identity, credentials, API, assets, catalog, transport and imports. The historical
experiment remains an independent module. See [README](../README.md) for operation,
[technical design](ports-over-cloud-v1.md) for protocol/bounds and
[requirements](../PORTS_OVER_CLOUD.md) for the threat model.

Implemented: separate admission/group secrets; outbound WS/WSS; Noise NNpsk2
admission and XXpsk3 E2E; UUID/static-key binding; exact-byte authenticated catalog;
persisted sequences/watermarks; pinned manual imports; live provider authorization;
in-memory dispatch; HTTP/WebSocket/streaming; optional Origin rewrite; reconnect;
revocation; protected administration; loop detection and bounded tunnel resources.
No P2P, VPN, Aux listener, accounts/provisioning API, legacy migration, automatic
identity recovery, or implicit import rebinding was added.

## Commands and outcomes

All completed successfully:

```sh
GOWORK=off go mod tidy
make check-modules
make check
make build
python3 scripts/cloud-demo.py
GOWORK=off go test -race ./internal/cloud/... ./internal/modules/ports_over_relay ./internal/app
GOWORK=off go test ./internal/cloud/protocol -run '^$' -fuzz '^FuzzStrictWire$' -fuzztime 3s -parallel 2
GOWORK=off go test ./internal/cloud/protocol -run '^$' -fuzz '^FuzzAuthenticatedCatalog$' -fuzztime 3s -parallel 2
```

`make check` runs core/full race tests and vet plus the independent experiment.
`make check-modules` checks core, each of six modules, Local+Remote, cloud with
each source, and full builds: race tests, vet, builds and binary dependency graphs.
Cloud implementation/libraries are absent when its tag is excluded. `make build`
produces full Zelport, integrated relay and the two historical experiment commands.
The short fuzz runs completed 220,826 and 211,492 executions respectively without
failures; these are smoke checks, not exhaustive fuzzing.

## Evidence by behavior

| Area | Evidence |
| --- | --- |
| Credentials and storage | Secret-state redaction; explicit protected generate/reveal; corrupt identity rejected; failed writes preserve identity; distinct admission/group tokens; token file exclusive creation and 0600 permissions |
| Concurrency | Counter/watermark CAS; a user edit survives intervening metadata commits without losing counters; concurrent user edits still conflict; existing core lease/upgrade/revocation race tests |
| Admission and E2E | Wrong admission/group tokens; provider static-key substitution; changed group/UUID/session/version contexts; recorded hello/ready replay; handshake cancellation; tampered/replayed ciphertext; rekey in both directions; plaintext/credential capture assertions |
| Protocol | Independent HKDF golden vectors; strict duplicate/trailing/null/framing rejection; maximum 256-service catalog fits its control message; malformed-input fuzz smoke |
| Catalog | MAC forgery, expiry/future skew, duplicate IDs, durable lower/equal-conflicting sequence rejection; one rejected unrelated record does not interrupt a healthy active tunnel |
| Relay | Admission required; duplicate active registration denied; paired connection releases pending capacity; provider control loss closes active pair; WS and a controlled TLS reverse proxy |
| Routing | Two actual managers and relay; source UUID dispatch; application 403/path/query/Host; own Origin rewrite and foreign/missing Origin preservation; provider rename retains import identity; unknown/unpublished service denied |
| Long-lived traffic | First streaming chunk arrives without buffering; bidirectional upgraded traffic; publication disable closes an active WebSocket; per-request ownership survives HTTP response serialization |
| Loops and replay | Actual indirect two-instance cycle returns 508; stripped trace reaches bounded stream rejection (503) and unwinds; lost backend response does not repeat a side-effecting POST; forged/repeated/expired/eighth/ninth-hop traces |
| API | Exact authority, Origin and CSRF gate credentials; normal state exposes neither token nor private key; import binding immutable; offline import rename preserved |
| Process lifecycle | Two compiled Zelport processes and relay, original/renamed imports, custom gateway ports, app 403, disable/re-enable, relay termination/restart, retained imports |
| Browser | Isolated cloud-only and Local+Cloud builds; empty/connected catalog, saved imports, settings dialog, default sharing off, domain collision, successful delayed editor save across
  catalog refreshes, URL-fragment reload; no console errors in checked flows |

The self-cleaning Python demo owns its subprocesses, ports, token file and configs.
WSS, upgraded traffic and indirect loops are covered by Go integration tests;
the process demo itself uses WS and a simple HTTP application.

## Review findings and resolutions

A separate code reviewer found two important defects and two bound inconsistencies:

- Rejecting one invalid record disconnected every provider. Records are now rejected
  individually with a visible warning; a healthy active stream is regression-tested.
- Routine catalog persistence invalidated open editors. Metadata-only commits retain
  strict internal CAS while user mutations can cross metadata revisions. A later
  user mutation still conflicts; settings callbacks preserve the latest metadata.
- Active paired tunnels consumed the pending-pairing quota. Pending allocation now
  ends at pairing, while active ownership/control-loss cleanup is retained.
- A maximum catalog exceeded its control frame after double Base64 encoding. Signed
  records now embed as raw JSON; maximum-size round-trip coverage guards the bound.

A separate integration failure exposed premature closure of a borrowed dispatch
lease before net/http serialized the response. Host.Dispatch now borrows the lease;
the caller holds it until connection teardown. Public gateway dispatch still owns
and closes its lease. This contract is documented and covered by HTTP integration.
Relay response queues allow a full 32-device reply plus event headroom (64 entries).

## Assurance and deployment limits

- No independent cryptographic audit, full formal protocol proof, exhaustive
  adversarial scheduler, long-running soak test or Internet deployment was performed.
  The design's scenario list remains a review/coverage roadmap; not every named
  hypothetical test in the original plan has a dedicated test function.
- Physical laptop sleep, real network partitions across machines, production WSS
  termination, Linux/Windows runtime operation and an actual remote SSH service
  remain unverified. Local relay restart and controlled TLS cover narrower cases.
- Browser evidence covers desktop light-mode flows. Dark-mode/narrow-layout and
  every modal/revocation interaction combination were not separately audited here.
- A malicious relay can disconnect, suppress catalogs and expose traffic metadata.
  E2E does not establish complete/fresh catalogs or protect a compromised endpoint
  or malicious group member. Existing imports pin keys; first enrollment relies on
  safe out-of-band group-token distribution.
- Correct clocks matter: records last 60 seconds with 5 seconds future skew.
  Backups can roll back sequence watermarks; identity/sequence recovery is explicit.
  Accepted-device history is bounded to 1,024 entries and requires administrative
  cleanup/new configuration if exhausted; history is not silently discarded.
- Stripped traversal metadata prevents guaranteed loop detection. Tested resource
  bounds terminate the controlled stripped-header cycle, not every arbitrary proxy
  topology. Streams/WebSockets have no short total lifetime imposed to mask loops.

Pinned dependencies were checked against primary release/security sources:
[coder/websocket security](https://github.com/coder/websocket/security),
[coder/websocket releases](https://github.com/coder/websocket/releases),
[flynn/noise advisory](https://github.com/flynn/noise/security/advisories/GHSA-g9mp-8g3h-3c5c)
(the affected range precedes pinned v1.1.0), and
[yamux releases](https://github.com/hashicorp/yamux/releases).
This check is not a comprehensive dependency-security assessment.

## Cloud administration v2 — 2026-10-12

Isolated relay/client integration covers hostname and UUID aliases, publishing off,
remote route creation on the provider only, different gateway ports, receiver-side
import links, wrong/duplicate Origin, unknown aliases, wrong ports and remote cloud
disable with durable configuration despite possible response loss. The disable
scenario passed 30 consecutive race-enabled runs after fixing registration conflicts
with concurrent catalog persistence. Authorization registration can be retried before
HTTP begins; an HTTP request itself is never automatically retried.

Unit tests cover scoped CSRF tokens, forged-context headers, reserved route names,
typed opens and invalid link metadata, peer pin/alias persistence, failed writes,
restart/rename/collision retention, effective versus metadata commits, and active
administration streams surviving publication withdrawal and closing on client reset.
A final independent code review identified peer changes incorrectly marked metadata;
the fix uses effective commits with strict revision comparison and regression tests.

Chrome verification used temporary configurations and two Zelport instances on
18180/18181 with an isolated relay. Both aliases loaded; a remote route edit changed
the provider, a renamed receiver import supplied its clickable URL, an unmapped
service had no link, and remote cloud UI omitted chained administration links.
The final remote banner and endpoint wrapping were visually checked. The temporary
fixture and listeners were removed afterwards.

This is in-process multi-instance integration plus a separate browser fixture process,
not a new multi-machine rollout or independent cryptographic audit. Live iMac/MacBook
and relay processes were not upgraded. Coordinate all three for protocol v2; verify
preserved device IDs/routes and both remote aliases after deployment. The older
process-demo evidence remains historical and does not prove the new admin feature.

Final commands passed: `make check check-modules build build-core`. This includes
root core/full race tests and vet, the isolated historical module, feature exclusion
checks, full/core Zelport and relay builds. No new dependencies were added.
