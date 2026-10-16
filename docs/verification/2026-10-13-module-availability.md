# Module availability, maturity and Settings accordions

Implemented required stable/experimental descriptor maturity, persisted navigation
availability and native Settings accordions. The five original source modules are
stable; the relay client and hosting module are experimental. Only standalone tabs
have availability switches. Relay hosting retains its own functional enable setting.

Core guards hiding with persisted route counts and a pure optional module guard.
Ports over Relay reads desired connection configuration, not transient connection
status. Used interfaces are kept available, including across upgrade/restart and
after removal of the last route. Availability never stops a backend or resets its
resources. Origin/CSRF and revision protection apply to its core mutation endpoint.

Verification completed on macOS:

- `make check`: core/full root race tests and vet plus the independent experiment.
- `make check-modules`: feature/dependency matrix, including relay-only/client-only.
- `make build`, plus `go build -o bin/snitzel-core ./cmd/zelport`.
- Targeted app/web race tests after final adjustments: defaults, explicit choices
  surviving restart and maturity changes, configured routes, desired connection
  guard, failed persistence, owner-only permissions, stale revisions, concurrent
  route creation versus hiding, required maturity, absent-module and mutation API
  protection.
- Thirteen frontend regression tests: existing relay draft/secret protections,
  error ownership, unavailable deep links and remotely hidden active tabs.
- Browser tests against an isolated full instance: default maturity states, enabling
  and hiding tabs, disabled switches/reasons for an unavailable local service and
  a disconnected but enabled relay client, state retained after process restart,
  native accordion toggling, draft retention, Space activation with retained switch
  focus, ArrowRight skipping hidden tabs and unavailable hash fallback to Settings.
- Core-only browser: Module availability contains no uncompiled modules or Relay
  hosting accordion.

Tests used temporary configurations on ports 18080/18081. Existing user services,
credentials and functional module enable settings were not changed. Linux/Windows
browser/runtime behavior was not exercised by this change.
