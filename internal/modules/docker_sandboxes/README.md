# Docker Sandboxes

This module exposes applications running in local Docker Sandboxes through
Zelport domains. It requires the `docker_sandboxes` build tag and an installed
`sbx` CLI. See the [project README](../../../README.md) for build and startup
commands and [module contracts](../../../docs/MODULE_CONTRACTS.md) for integration
with the core.

## Discovery and service lifecycle

Discovery runs periodically and after configuration changes. The refresh icon
on the module tab requests discovery immediately. The adapter invokes `sbx`
with direct arguments, without a shell, and enforces a bounded command timeout.
The [module descriptor](module.go) and
[adapter defaults](../../sbx/client.go) are authoritative for those timings.

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

JSON does not guarantee a stable schema across CLI releases. The
[adapter](../../sbx/client.go) accepts both observed list representations:

```json
{"sandboxes": [{"id": "sandbox-id", "name": "project", "status": "running"}]}
```

```json
[{"id": "sandbox-id", "name": "project", "status": "running"}]
```

The checked-in fixtures and adapter tests are authoritative for the accepted
list and port representations. Port listings are arrays of objects with
`host_ip`, `host_port`, `sandbox_port` and `protocol` fields. Additional JSON
fields are ignored. This is support for tested formats, not a promise of
compatibility with every future `sbx` release.

Empty arrays are valid inventories. Missing or null inventories, malformed JSON,
missing sandbox IDs/names/statuses and duplicate IDs are rejected. Unrecognized
output is a discovery error, never evidence that all sandboxes were removed.

## Discovery failures in the UI

CLI failures preserve configuration and last-known usable mappings. The module
tab hides its sandbox and service lists while list discovery fails, showing a
bordered error notice with a collapsed **CLI output** disclosure. The raw error
is displayed as text with preserved line breaks; the UI does not parse its wording
or infer remedies. An unchanged notice is not recreated during polling, preserving
text selection and the disclosure state. Successful discovery restores the list.

The Dashboard retains configured services and their reported state. State cells show only the state label, without an additional diagnostic line.
Hiding the module list does not stop existing proxies.

## Verification

Run from the repository root:

```sh
go test -race ./internal/sbx ./internal/modules/docker_sandboxes
ZELPORT_LIVE_TEST=1 go test ./internal/sbx -run TestLiveReadOnly -v
```

The [adapter tests](../../sbx/client_test.go) cover both list shapes, empty and
invalid inventories, identity preservation, command arguments and timeouts.
Sanitized [JSON fixtures](../../sbx/testdata/) describe the supported wire formats.
The opt-in live test reads the installed CLI's inventory and ports; it does not
publish ports or start sandboxes. Mutation behavior is tested with controlled
backends rather than the user's services.
