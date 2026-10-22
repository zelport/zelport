# Zelport

[![Verify release binaries](https://github.com/zelport/zelport/actions/workflows/release.yml/badge.svg?branch=main&event=push)](https://github.com/zelport/zelport/actions/workflows/release.yml?query=branch%3Amain)

Open what your AI agents are building. Give every web service a stable
localhost address whether it runs on your computer or a development server.
Use browser features that require a secure context without managing TLS
certificates.

Zelport App is a local HTTP reverse proxy with browser-based administration.
It gives web services stable `*.localhost` names through one loopback listener,
normally `http://localhost:8080`.

The standard build contains every capability described under
[Service behavior](#service-behavior). Docker, Podman, `sbx` and system SSH are
optional external tools for the capabilities that use them. Modules are internal
code boundaries, not product editions.

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

The [release workflow](.github/workflows/release.yml) exercises every packaged
binary on its native platform and publishes the exact tested archives. Homebrew
and Zelport Installer consume those immutable assets without rebuilding them.

Useful flags:

- `--port`: Localhost port, default `8080`; listeners bind only to loopback.
- `--config`: explicit configuration path. The default is
  `$XDG_CONFIG_HOME/zelport/config.json` or `~/.config/zelport/config.json` on
  macOS/Linux, and the native per-user configuration directory on Windows.
- `--version`: print the version and exit.

See [Configuration](docs/CONFIGURATION.md) for the schema, example and
persistence guarantees.

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
