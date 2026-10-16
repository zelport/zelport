# Zelport MVP implementation ledger

Authority: user-approved plan in this chat, 2026-10-11.

Deliverables: standard-library Go CLI; sbx JSON adapter; atomic persisted routes;
serialized reconciliation; loopback HTTP proxy and protected administration;
embedded English UI; lifecycle/error pages; tests and README.

Defaults: port 8080, poll 5 seconds, macOS, manual startup, HTTP only.
Saved configuration never starts a stopped sandbox. Explicit Start may publish
its configured port. Running sandboxes regain missing mappings automatically.
Removing a route optionally removes its exact mapping; shared mappings are guarded.

Ruling: work in the empty current checkout on a feature branch; there is no
existing code or commit to isolate into another worktree.
Ruling: fixtures retain real sbx v0.47.0 wire format but anonymize UUIDs/names.
Identity is UUID, never sandbox name. CLI commands address UUIDs.
Pre-flight: CLI models feed reconciliation; persisted routes feed proxy and UI.
One manager serializes changes, while immutable snapshots keep proxy requests
independent of slow CLI calls. No conflicting interfaces.

Progress:
- Verified actual ls envelope and ports array via read-only sbx calls.
- Task 1 complete: real-wire CLI fixtures, timeouts, error propagation, exact
  command arguments; adapter tests pass.
- Task 2 complete: persisted routes, stopped save, explicit start, automatic
  reconcile, UUID replacement guard, stale-state retention, removal and validation;
  lifecycle tests pass.
- Environment: Go tests need toolchain/cache access outside the restricted
  execution environment. Elevated read-only sbx listing works.
Ruling: actual sbx ports rejects UUID arguments. Resolve saved UUID to the current
name immediately before each CLI operation, without saved-name fallback. This
prevents ordinary same-name replacement attachment; the CLI does not provide an
atomic identity-conditional mutation, so an external delete/recreate racing the
command remains a Docker CLI limitation.
- Task 3 complete: HTTP routing, app error passthrough, immediate streaming,
  protocol upgrades, local admin authority, Origin+CSRF checks, JSON/HTML status
  responses tested successfully.
- UI and CLI implemented: embedded assets, no build dependencies, loopback IPv4
  and IPv6 listeners, background reconciliation and graceful shutdown.
- Independent review: three findings reproduced RED and fixed GREEN:
  automatic publication now rechecks fresh running status; explicit Start uses a
  separate backend operation; default port 80 uses canonical Origin; deletion
  intent is durable before unpublish and survives a final persistence failure.
- Live verification: inventory and per-sandbox ports read through the real CLI;
  existing port connected through UI; stopped config saved without starting;
  503 stopped page rendered; restart retained both routes; new temporary sandbox
  port 39999 published to host 49152; 502 page rendered for unavailable backend;
  exact temporary mapping removed through UI, verified absent via sbx ports.
- Existing running backend returned HTTP 404 both directly and through proxy;
  browser declined that navigation, so passthrough was verified over HTTP.
- Explicit start/stop behavior is tested with controlled backends. The user's
  stopped sandbox was not started or stopped during verification.
- Final verification: go test -race -count=1 ./..., go vet ./..., and standalone
  go build all passed. Live read-only adapter test passed. No browser console
  errors in tested UI flows. Temporary preview server stopped; original Docker
  mappings and sandbox running/stopped states preserved.
- Work remains locally on codex/zelport-mvp; no merge, push, or publication.
