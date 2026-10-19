# Containers

The `containers` build tag includes independent Docker, Podman and experimental
nerdctl discovery. Only existing published loopback TCP mappings are offered as
upstreams. Discovery does not start engines, create containers, alter connections,
or publish ports.

## CLI and runtime status

The **Containers** information panel reports CLI and runtime status per backend.
Missing optional executables are neutral; runtime failures become errors when a
configured service depends on that engine.

Executables are resolved from the Zelport process PATH. Backends scan
independently, verify runtime access before discovery, and retain last-known
containers only for the same connection identity.

Docker uses the configured local Unix socket/context. Podman supports native Linux,
local sockets and locally managed Podman machines. CLI commands use structured JSON.
Remote container hosts are not automatically interpreted as localhost upstreams.

### Docker socket permissions on Linux

A rejected runtime probe reports **Permission denied**, preserving the original
CLI output under **Technical details**. For the standard Docker socket, **How to
fix** explains the Linux rootful Docker setup: add the account running Zelport to
the existing `docker` group. Run this on the Docker host as that account:

```sh
sudo usermod -aG docker "$USER"
```

Sign out completely, sign in again and restart Zelport. Existing processes retain
their previous group membership. Zelport does not change system permissions.
With a rootful Docker daemon, the `docker` group grants root-level control of the
host; this statement does not apply to rootless Docker. See the
[Docker post-installation guide](https://docs.docker.com/engine/install/linux-postinstall/).
The group-specific hint is not shown for other engines or nonstandard/rootless
socket paths; check that connection's access policy instead.

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

Healthy inventories with the same canonical local socket and full container ID
share one displayed group without changing route identity. This is presentation
deduplication, not failover or identity migration.

## Verification

Run the opt-in live Podman scan with:

```sh
ZELPORT_CONTAINER_LIVE=1 go test ./internal/containers -run TestLiveReadOnly -v
```

The remaining adapter and frontend coverage uses controlled fixtures; nerdctl
fixtures are not a live containerd or VM integration test.
