# Containers

The `containers` build tag includes independent Docker, Podman and experimental
nerdctl discovery. Only existing published loopback TCP mappings are offered as
upstreams. Discovery does not start engines, create containers, alter connections,
or publish ports.

## CLI and runtime status

The UI has one compact row per backend, with separate CLI and runtime results.
Normal status rows are static, with no disclosure controls. Only failures with a
detected CLI offer expandable diagnostics, in the document flow so multiple open
details never overlap. Missing optional executables are neutral. Runtime failures
are warnings, or errors when configured services depend on that engine.

Executables are resolved from the Zelport process PATH on each scan. Each engine
has its own bounded scan; an absent or slow backend does not block the others.
An `info` response verifies runtime access before listing containers. A successful
runtime probe does not prove container application health. A later inventory
failure leaves runtime status available and reports the discovery error separately.
Failed scans retain last-known containers only for the same connection identity.

Docker uses the configured local Unix socket/context. Podman supports native Linux,
local sockets and locally managed Podman machines. CLI commands use structured JSON.
Remote container hosts are not automatically interpreted as localhost upstreams.

## Experimental nerdctl

Native Linux `nerdctl` uses `info --mode native --format '{{json .}}'` to identify
the configured containerd instance and effective namespace. Only that namespace
is queried, not every containerd or Kubernetes namespace. Namespace and daemon
UUID form the stable connection identity. The namespace is pinned for subsequent
list/inspect calls and runtime identity is checked again before publishing results.
An identity change discards the scan; it never reconnects existing routes by name.

The CLI must support native info with a containerd UUID and Docker-compatible JSON
inspect output. Missing identity fields or malformed output fail discovery rather
than falling back to human-readable output. Only reported loopback TCP bindings
are used; UDP and non-loopback bindings are excluded (wildcard bindings are reached
through host loopback, as with Docker and Podman).

macOS/Windows VM wrappers, including unverified Lima/Rancher Desktop forwarding,
are not supported in this first implementation. Their CLI can be found, but the
runtime remains unchecked with an explanation. Do not infer that a guest port is
reachable at the same port on the host. No nerdctl/containerd SDK dependency is
added, and this backend is excluded from builds without `containers`.

## Shared engines and UI stability

Two healthy inventories with the same canonical local socket and full container ID
share one displayed group. Both route references still match that group; adding a
new service uses its first backend. This is presentation deduplication, not failover
or identity migration. Unknown socket relationships, names alone and stale sources
are not deduplicated.

Status rows and resource lists render independently. Unchanged polls and revision
updates preserve DOM nodes, open details, selection and form drafts. Updated state
is still supplied to editors even when no render is needed.

## Verification

Adapter tests cover missing CLIs, runtime versus discovery failures, namespace and
UUID isolation, changed-runtime rejection, port parsing, and last-known inventory
recovery. Frontend tests cover the three status rows, severity, proven duplicate
sources and unchanged-poll retention. A live Podman scan is opt-in:

```sh
ZELPORT_CONTAINER_LIVE=1 go test ./internal/containers -run TestLiveReadOnly -v
```

Synthetic nerdctl fixtures do not constitute a live containerd/VM integration test.
