# Docker Sandboxes

This module exposes applications running in local Docker Sandboxes through
Zelport domains. It requires the `docker_sandboxes` build tag and an installed
`sbx` CLI. See the [project README](../../../README.md) for build and startup
commands and [module contracts](../../../docs/MODULE_CONTRACTS.md) for integration
with the core.

## Discovery and service lifecycle

Discovery runs periodically, after configuration changes, and on explicit
refresh. The adapter invokes `sbx` directly with a bounded timeout; see the
[module descriptor](module.go) and [adapter](../../sbx/client.go).

A **Sandbox port** is the application port inside the sandbox. An **Aux port** is
its intermediate host mapping. Applications must listen on an interface reachable
by port publishing, usually `0.0.0.0` inside the sandbox rather than only loopback.
Only usable loopback TCP mappings are used as proxy upstreams.

Configuring a stopped sandbox does not start it. Publication happens when the
sandbox is running, or through the explicit **Start sandbox** action. Visiting a
service never starts its sandbox. Saved sandbox IDs remain authoritative: the
adapter resolves the current name immediately before port operations and never
reconnects a missing identity merely because another sandbox has the same name.

Removal offers **Remove from proxy** or **Remove proxy and port mapping**. A
mapping cannot be removed while another route uses it. Removal intent is persisted
before the CLI operation, preventing refresh from recreating the mapping after a
failed removal. **Open terminal** is a macOS-only explicit action using
`sbx exec -it <name> bash` after resolving the sandbox identity.

## CLI compatibility

Discovery uses machine-readable output:

```sh
sbx ls --json
sbx ports <name> --json
```

JSON does not guarantee a stable schema across CLI releases. The adapter accepts
both an object containing `sandboxes` and a top-level array. Port listings use
`host_ip`, `host_port`, `sandbox_port` and `protocol` fields. Additional JSON
fields are ignored. The checked-in fixtures and tests define the supported
formats; future `sbx` releases are not implicitly compatible.

Empty arrays are valid inventories. Missing or null inventories, malformed JSON,
missing sandbox IDs/names/statuses and duplicate IDs are rejected. Unrecognized
output is a discovery error, never evidence that all sandboxes were removed.

## Discovery failures in the UI

CLI failures preserve configuration, last-known mappings and active proxies.
The module tab replaces its lists with an error notice and raw CLI output until
discovery succeeds; the Dashboard retains configured services.

## Verification

Run from the repository root:

```sh
go test -race ./internal/sbx ./internal/modules/docker_sandboxes
ZELPORT_LIVE_TEST=1 go test ./internal/sbx -run TestLiveReadOnly -v
```

The [adapter tests](../../sbx/client_test.go) and sanitized
[JSON fixtures](../../sbx/testdata/) define supported formats and identity
behavior. The live test is read-only; mutations use controlled backends.
