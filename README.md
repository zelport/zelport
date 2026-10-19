# Zelport

[![Verify release binaries](https://github.com/zelport/zelport/actions/workflows/release.yml/badge.svg?branch=main&event=push)](https://github.com/zelport/zelport/actions/workflows/release.yml?query=branch%3Amain)

Zelport App is a local HTTP reverse proxy with browser-based administration.
It gives local services stable `*.localhost` names through one loopback
listener, normally `http://localhost:8080`.

The standard Zelport App contains the complete registered service-capability
set:

- Docker Sandboxes;
- Containers;
- Local Ports;
- Remote HTTP;
- Ports over SSH;
- Ports over Relay;
- Built-in Relay.

Docker, Podman, `sbx`, system SSH, Homebrew and systemd are supported external
tools. They are not bundled with Zelport App and are required only for the
capability that uses them. Modules are internal code boundaries, not product
editions that users select when installing Zelport.

## Install

Install released builds through the [Zelport website](https://zelport.com/) or
Homebrew:

```sh
brew install zelport/tap/zelport
brew services start zelport
```

Installation does not start Zelport unless the selected installation flow says
so. Run Zelport and service-manager commands as the regular desktop user, never
with `sudo`. The independently versioned
[Zelport Installer](https://github.com/zelport/zelport-installer) owns the shell
installer; this repository contains no editable installer copy.

## Build and run

Use the Go version declared by [`go.mod`](go.mod) or newer.

```sh
make check
make build
./bin/zelport
```

`make build` and ordinary `go build ./cmd/zelport` produce the complete Zelport
App. `make build-core` produces the diagnostic `zelport-core` executable for
module-boundary checks; it is not a separately distributed product.

## Release verification

The [release workflow](.github/workflows/release.yml) builds the archives for
every platform in its release matrix once. It then executes each packaged
binary on a matching standard GitHub-hosted runner and verifies startup, the
standard module composition, Local Ports proxying, Local Integration, Built-in
Relay, Ports over Relay and graceful shutdown. A stable tag is published only
after every native job passes, and publication uses the exact tested archives
without rebuilding them.

Homebrew and Zelport Installer automation consume these immutable App release
assets. They verify their own installation paths but never rebuild Zelport App.

Useful flags:

- `--port`: Localhost port, default `8080`; listeners bind only to loopback.
- `--config`: explicit configuration path. The default is
  `$XDG_CONFIG_HOME/zelport/config.json` or `~/.config/zelport/config.json` on
  macOS/Linux, and the native per-user configuration directory on Windows.
- `--version`: print the version and exit.

The configuration schema is exactly version `1`. A missing file is created as a
valid empty configuration. Any other version, unknown field or invalid module
setting is rejected; Zelport does not import or rewrite an earlier schema.

```json
{
  "version": 1,
  "revision": 1,
  "modules": {},
  "routes": [
    {
      "id": "82958650-fb63-43c1-903b-f3b6f89b1667",
      "module": "local_ports",
      "domain": "example.localhost",
      "settings": {"aux_port": 3000}
    }
  ]
}
```

Use the administration UI to create and edit routes. Configuration writes are
atomic and owner-only. Route UUIDs survive edits, and concurrent changes are
rejected instead of silently overwriting one another.

## Service behavior

**Localhost port** is Zelport App's public loopback listener. **Aux port** is a
host-side backend port. **Container port** and **Sandbox port** identify ports
inside those environments.

- Docker Sandboxes discovers applications through `sbx`.
- Containers discovers already published TCP mappings through Docker or Podman;
  native Linux `nerdctl` support is experimental. Zelport does not create
  mappings or manage container lifecycle.
- Local Ports connects a hostname to a local HTTP backend.
- Remote HTTP forwards to an HTTP or HTTPS origin with explicit TLS and Origin
  rewriting controls.
- Ports over SSH uses the system SSH client and existing SSH configuration.
- Ports over Relay connects mutually trusted Zelport App instances through
  Relay Protocol v1.
- Built-in Relay exposes the same relay server implementation as standalone
  Zelport Relay.

HTTP streaming, WebSocket upgrades and application status codes are preserved.
Removing or changing a route revokes its active requests and upgraded
connections. An unknown hostname returns 404, unavailable source infrastructure
returns 503, and an unreachable HTTP backend returns 502.

## Relay

[Zelport Relay](https://github.com/zelport/zelport-relay) is the standalone
relay server. Built-in Relay is the same server embedded in Zelport App. Ports
over Relay is a service source that acts as a relay client. These roles are
independent even when they run in one executable.

Relay Protocol v1 uses `/v1/ws` and the `zelport-relay/v1` cryptographic
namespace. A group is one mutually trusted administration boundary: members can
administer one another through authenticated relay sessions. Use a separate
group when that trust is not appropriate. Start with fresh identities,
credentials and relay configuration; no earlier schema or protocol is imported.

See [Ports over Relay](PORTS_OVER_RELAY.md) for the operating model and
[Relay Protocol v1](docs/ports-over-relay-v1.md) for the normative technical
contract.

## Architecture and operations

- [Architecture](DESIGN.md)
- [Modules](docs/MODULES.md)
- [Module contracts](docs/MODULE_CONTRACTS.md)
- [UI design system](docs/UI_DESIGN_SYSTEM.md)
- [Read-only local integration API](docs/LOCAL_INTEGRATION_API.md)
- [Homebrew packaging and services](docs/HOMEBREW.md)
- [Docker Sandboxes](internal/modules/docker_sandboxes/README.md)
- [Containers](internal/modules/containers/README.md)
- [Deferred work](TODO.md)

Zelport refuses to run as root by default on macOS and Linux. `--allow-root`
exists for deliberate controlled environments and always emits a warning. The
restriction prevents accidental privileged operation; it is not a security
boundary against an administrator.

Licensed under the [MIT License](LICENSE). Third-party notices are listed in
[THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt).
