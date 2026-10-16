# Relay hosting verification

The optional `relay` server shares its listener lifecycle with `snitzel-relay`.
It is independent of the `ports_over_relay` client, now displayed as Ports over Relay.

## Automated coverage

- Runtime tests exercise IPv4/IPv6 wildcard listeners, rollback after one family
  fails to bind, and synchronous closure of upgraded connections.
- Module tests use temporary configurations and protected web APIs to exercise
  enable/disable, restart/token persistence, port changes, conflicting edits,
  occupied ports, atomic write failure, startup recovery and secret-free state.
- Integration tests run HTTP, streaming, WebSocket, administration, wrong-token
  rejection and reconnect against both the shared standalone runtime and embedded
  hosting. These are controlled local tests, not real routed-LAN deployment tests.
- The build matrix checks core, every feature, relay/client combinations, and
  excluded dependency graphs. Server-only builds exclude client and yamux.

The frontend lifecycle regression tests use Node's built-in test runner, without
third-party packages or a frontend build:

```sh
node --test scripts/*-ui.test.mjs
```

They verify recovery from revision conflicts, late token responses after navigation
or disposal, edits made while a save is pending, and tab ownership of errors that arrive after
navigation. Browser checks cover Settings
navigation/history, keyboard navigation, enable/disable, port changes, occupied-port
errors, token reveal/copy, draft preservation and the core-only empty Settings page.

During the expanded matrix run, two existing startup-readiness assumptions surfaced:
the client test observed catalog entries before Connected, and administration began
before the provider had the consumer's catalog. Tests now wait for the relevant
readiness conditions; no production client authorization behavior was changed.

## Platform checks

Run these on each supported operating system with local TCP binding permitted:

```sh
GOWORK=off go test -race ./internal/cloud/relay ./internal/modules/built_in_relay ./internal/cloud/integration
make check
make check-modules
make build
```

macOS local runtime tests and browser checks are recorded for this implementation.
Linux amd64 and Windows amd64 cross-builds check compilation only. Actual runtime,
firewall, multi-interface and routed-VLAN checks on those platforms remain unverified.
No test modifies the user's live configuration or service routes.
