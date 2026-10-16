# Zelport

A local HTTP reverse proxy with optional Docker Sandbox, Container, Local Ports,
Remote HTTP, Ports over SSH and Ports over Relay modules, plus optional relay
hosting. Give each service a .localhost domain on one Localhost port, normally
8080. Administration is http://localhost:8080.

## Homebrew installation and services

Release packaging generates a Homebrew formula for macOS and Linux on ARM64 and
x86-64. The tap and release assets must be published before these commands are
available to users:

```sh
brew install zelport/tap/zelport
brew services start zelport
```

Installation alone does not start Zelport. `brew services start` starts it now
and registers automatic startup at login through launchd on macOS or systemd
on Linux. Run both commands as your regular user, without `sudo`.
Alternatively, run `zelport` in a terminal. Do not start both on the same port.
The dashboard is at http://localhost:8080.

```sh
brew services info zelport
brew services restart zelport
brew services stop zelport
```

Other Linux distributions can use the Linux binary with their own service
manager, such as OpenRC on Alpine. Linux release binaries are built without
cgo. Runtime compatibility still needs verification on each supported platform.
See [Homebrew packaging and service configuration](docs/HOMEBREW.md) for release
preparation, configuration paths, logs and service environment requirements.

## Build and run

Requires Go 1.27 or newer. macOS is the currently verified platform.
Dependencies use public version tags and are downloaded automatically by Go.
A standalone checkout can be built without the development workspace.

```sh
# Full distribution with all seven implemented features.
make build-full
./bin/zelport

# Core only: administration, Dashboard and Settings, no service sources.
make build-core
./bin/zelport-core --config /tmp/zelport-core.json --port 8081

# Explicit feature selection; the core is always included.
go build -tags 'local_ports,remote_http' -o bin/zelport-custom ./cmd/zelport
./bin/zelport-custom --config /tmp/zelport-custom.json --port 8082
```

Bare `go build ./cmd/zelport` builds only core. There are no core/minimal/custom/all
tags. Full release artifacts use the Makefile's explicit FEATURE_TAGS list.

| Feature | Build tag | Runtime requirement |
| --- | --- | --- |
| Docker Sandboxes | `docker_sandboxes` | `sbx` CLI and Docker Sandboxes |
| Containers | `containers` | Docker and/or Podman CLI, local engine connection |
| Local Ports | `local_ports` | Local HTTP backend |
| Remote HTTP | `remote_http` | Reachable HTTP/HTTPS backend |
| Ports over SSH | `ports_over_ssh` | System SSH and a preconfigured trusted host |
| Ports over Relay | `ports_over_relay` | Reachable relay and shared credentials |
| Relay hosting | `built_in_relay` | Available TCP listen port |

Runtime Dashboard lists the actual compiled modules.
Excluded modules contribute no feature implementation, API, frontend assets or
adapter dependencies to the Zelport binary. See [module design](docs/MODULES.md).

CLI flags:

- `--port`: Localhost port, default 8080; listeners bind 127.0.0.1 and ::1 only.
- `--config`: explicit configuration file path, overriding the default.
  On macOS and Linux, use `$XDG_CONFIG_HOME/zelport/config.json` when
  `XDG_CONFIG_HOME` is absolute; otherwise use `~/.config/zelport/config.json`.
  Unset, empty, and relative XDG values use this fallback. On Windows, use
  `%LOCALAPPDATA%\Zelport\config.json` (or the Windows LocalAppData known
  folder when the environment variable is unset). Existing APPDATA preview files
  are not migrated automatically; select them explicitly with `--config`.
  The previous macOS Application Support path is not searched automatically;
  move an existing configuration while Zelport is stopped, or select it with `--config`.
- `--version`: print version and exit without reading config or starting listeners.

Make builds embed `dev-<short-git-commit>` (with `-dirty` for uncommitted changes)
and a separate UTC build timestamp, e.g. `2026-10-12 11:05:40 UTC`. Both appear
in the Dashboard, `--version`, and the Zelport startup log. The Dashboard formats
the build timestamp in the browser timezone, using an English zone label (e.g. CET/CEST); CLI output
retains UTC. Without a Git commit,
the version is `dev`. Use `make build VERSION=0.1.0` for a release version.
Direct `go build` defaults to `dev` and an `unknown` build time; inject
`main.version` and `main.builtAt` with linker `-X` flags alongside selected tags.
An occupied listener port produces a startup error. No automatic login startup is
installed. HTTPS is supported for Remote upstreams, not the local gateway.

## Configuration

**Version 6 is intentionally incompatible with previous configurations.** No legacy
migration is provided. Start with a fresh config. Existing unsupported files are
rejected without being overwritten. A missing file is created with a valid empty version 6 configuration at startup.
Startup fails if the initial configuration cannot be saved. Existing files are
never replaced during initialization.

```json
{
  "version": 6,
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

Use the administration UI to edit routes. Domain names may contain multiple labels,
for example admin.project.localhost, and must be unique across all modules. UUID
identities survive edits and renaming. A configured but excluded module is a startup
error naming its required tag. Configuration saves are atomic and owner-only;
concurrent user edits conflict instead of overwriting another update. Internal
cloud sequence/watermark commits do not invalidate an open editor.

## Service behavior

**Sandbox port** is the port inside a sandbox; **Aux port** is an intermediate
host backend port; **Localhost port** is Zelport's shared public local gateway.
Ordinary containers use the term **Container port**.

Docker Sandboxes exposes applications in local sandboxes. See the
[module documentation](internal/modules/docker_sandboxes/README.md) for discovery,
service lifecycle and `sbx` CLI compatibility.

Docker Sandboxes shows the installed `sbx` client version and discovery status
in its header status button. Click **Docker Sandboxes** in the header for the
executable path and diagnostics. The **Containers** header button similarly groups
CLI, runtime and discovery details by engine.
CLI metadata is checked at startup and on manual Refresh; automatic discovery
polls reuse it. A readable version does not establish daemon or application health.

Containers discovers already-published local TCP mappings from Docker/Podman
and experimental native-Linux nerdctl ([details](internal/modules/containers/README.md));
it does not create mappings or manage container lifecycle. Engine/connection/full
container identity are retained across refresh. Remote engine connections are
not supported. CLI outages retain last-known usable inventory.

Local Ports connects a domain to a localhost Aux port, supporting IPv4 and IPv6.
Remote HTTP accepts http(s)://host:port and an optional base path; incoming paths
and query strings are forwarded. Target credentials, query and fragments are not
accepted. TLS verification defaults on; disabling it is an explicit per-route
choice. Rewrite Origin defaults off; enabling it translates this proxy's origin
for HTTP and WebSocket while preserving foreign or missing origins. Redirects,
cookies, HTML and applications' own public-URL settings are not rewritten.
The Remote HTTP dialog reminds users to proxy only trusted services and avoid
sensitive websites. HTTP targets and disabled TLS verification show explicit
upstream security warnings in the dialog and accessible warning tooltips beside
target URLs in the service list and Dashboard. A localhost
secure context does not establish trust or encryption for the upstream connection.
Warnings belong to administration; proxied application responses are unchanged.

Ports over SSH uses the host's SSH config and system SSH `-W` over stdio. It creates
no Aux port. Verify host keys and unlock credentials using SSH first; Zelport is
noninteractive and does not accept new host keys automatically. Connections are
request-driven and bounded, and processes close on route removal/shutdown. SSH
process tests are isolated; a real remote target is not yet live-validated.

All proxy modules preserve WebSocket upgrades, streaming and application statuses
(including 403). Changing/removing a route closes its active requests and upgraded
connections. A configured mapping is not a health check. Unavailable infrastructure
returns 503, unreachable HTTP backends 502, unknown domains 404. Project /api paths
belong to the application; local administration uses exact localhost authority; authenticated cloud administration
uses the reserved namespace described below.

## Settings and relay hosting

The **Settings** tab (`#settings`) contains sections contributed by compiled modules.
Core-only builds show an empty settings page. The `relay` server module and
`ports_over_relay` client module are independent; enabling one never enables the other.
The client is displayed as **Ports over Relay**. Its module ID, build tag and
configuration key are `ports_over_relay`; the `#cloud` UI tab anchor remains unchanged.

When upgrading a configuration from before the module rename, stop the process
and make a private backup first. Rename the previous client entry to
`ports_over_relay` in both `modules` and `module_availability`, and update that
client's route `module` values. Preserve every settings value and route UUID,
including credentials, provider pins and catalog watermarks. Do not merge entries
if both names exist. There is no automatic legacy alias or configuration migration.
The relay protocol and relay server configuration are unaffected.


```sh
# Gateway plus an optional embedded relay server, without a relay client.
go build -tags built_in_relay -o bin/zelport-relay-host ./cmd/zelport
```

In **Settings → Relay**, select **Enable relay**, choose a **Port** (default `8090`),
and save. Hosting is disabled by default. **Bind address** accepts a
local IPv4 or IPv6 address; blank means **All interfaces**, including VPN and public
interfaces. All-interfaces mode opens separate IPv4 and IPv6 listeners where supported.
The Localhost gateway and administration remain on loopback, normally port `8080`.
The relay port exposes only the relay protocol, never the administration UI.

First enable with protection required generates a separate random **Relay access token**, persisted with the
configuration. Use the reveal/copy actions in **Relay access token** to configure clients; normal state
responses never contain the token. Disabling and restarting preserve it. The relay
never receives the E2E secret. Group members still have full mutual administration
access, independently of application sharing, as described below.

A client on the host can use `ws://127.0.0.1:8090`; other clients use the host's
reachable DNS name or IP, for example `ws://192.168.1.20:8090`. No multicast or mDNS
is needed. VLAN routing and firewalls must allow clients to reach that endpoint.
**Listening** confirms local binding, not remote reachability. WS retains mandatory
Noise admission and peer E2E encryption; use a TLS reverse proxy for WSS.

**Require access token** is enabled by default, including for existing
configurations. Disabling it explicitly allows clients with an empty access token.
Anyone who can reach the relay can then connect and consume capacity. The visible
**Access token not required** badge appears beside the relay status. The channel uses
unauthenticated Noise NN encryption; it does not authenticate the relay. Peer E2E
identity/group authentication remains mandatory, with no fallback on a wrong token.
In **Ports over Relay → Settings**, turn off **Use access token** to clear a saved
client token. A blank token field with that option enabled keeps the saved value.

Changing protection persists first, then disconnects current clients on the same
listeners. Turning protection back on reuses the saved token, or generates one if
none exists. Disabling protection preserves the saved token for later use. First
enable without protection does not generate an unused token. Standalone CLI hosting
continues to require an access-token file.

The access-token field in Built-in Relay settings has inline actions to reveal/hide, copy, or change the token.
Changing it requires confirmation: generate a fresh 32-byte token or enter an
existing base64url-encoded 32-byte token. A successful change is persisted first,
then closes all existing relay connections without releasing the listeners.
Clients must update their relay access token; the E2E secret is unchanged.
A failed write preserves the old token and connections. Revealed tokens are cleared
when leaving Settings or when the configuration revision changes.

Settings show listener addresses, connected clients, and errors.
A failed port reservation or configuration write leaves the previous server running.
Use **Reload settings** to discard a draft and recover from a concurrent-edit conflict.
For overlapping bind addresses on the same port, disable before changing the bind.
A startup bind failure leaves administration available; fix the conflict and save
again. Disabling closes all relay connections before the save returns. A remote
administration request using that relay can lose its response; inspect the resulting
state before manually retrying. Stopping the hosting Zelport stops its relay too.

The standalone `zelport-relay` uses the same server and listener lifecycle but keeps
its default `127.0.0.1:8090` bind. Use an explicit address for LAN hosting:

```sh
../zelport-relay/bin/zelport-relay --listen :8090 --access-token-file ./relay-access.token
```

Run `../zelport-relay/bin/zelport-relay --help` for options, environment variables and examples.
Explicit CLI options override their corresponding environment variable, then defaults.

| CLI option | Environment variable | Default |
| --- | --- | --- |
| `--listen` | `ZELPORT_RELAY_LISTEN` | `127.0.0.1:8090` |
| `--access-token-file` | `ZELPORT_RELAY_ACCESS_TOKEN_FILE` | unset |
| — | `ZELPORT_RELAY_ACCESS_TOKEN` | unset |
| `--allow-public-access` | `ZELPORT_RELAY_ALLOW_PUBLIC_ACCESS` | `false` |
| `--max-connections` | `ZELPORT_RELAY_MAX_CONNECTIONS` | `128` |
| `--log-level` | `ZELPORT_RELAY_LOG_LEVEL` | `info` |

A token file and a direct environment token cannot be combined. A token source
cannot be combined with public access either. Empty tokens are errors; they never
implicitly enable public access. The public-access environment variable accepts
only `true` or `false`; `--allow-public-access=false` overrides an environment
value of `true`. Ports must be in 1–65535 and connection limits must be positive.

To explicitly permit admission without an access token:

```sh
../zelport-relay/bin/zelport-relay --listen :8090 --allow-public-access
```

Public access does not change the default bind address or publish applications.
Clients still need their shared E2E secret, which the relay never receives.
At info level, startup prints the version/build time, access mode and actual listeners.
Public mode prints `zelport-relay access: public (no access token required)` at startup,
including with `--log-level error`. Token-file changes take effect after restart.

Standalone and embedded relays reserve each port for one cooperating relay instance
per OS user account, even on different bind addresses. Reservations use OS-held file
locks under `<account-home>/.zelport/relay-locks/`, independent of the configuration
path and cache environment. Lock files stay in place; their presence does not mean
a relay is running. Closing or crashing the process releases the lock. Do not delete
these files while relays are running. A missing system account record or non-writable home/lock directory prevents
startup rather than bypassing the reservation.

This coordinates updated Zelport processes, not unrelated applications, older
binaries or other user accounts. All cooperating instances must use the new build.
Normal socket reuse remains enabled for prompt restarts. `:8090` opens separate
IPv4 and IPv6 listeners where supported; `0.0.0.0:8090` selects only IPv4 and
`[::]:8090` only IPv6. No alternate port is chosen on a bind or reservation failure.
Changing the bind address on the same port requires disabling hosting first.

Both hosting modes use one configured relay endpoint per client, without automatic
failover or direct peer connections. The relay is a shared point of failure.

## Module availability and maturity

Settings contains expandable **Relay** and **Module UI availability** sections.
The latter lists only compiled standalone module interfaces, with switches labeled by module name and an Experimental badge where applicable. Stable modules have no badge;
enabled switches are green. Experimental navigation tabs also show an outlined flask
with an “Experimental module” tooltip on hover or keyboard focus. Their accessible
names include “Experimental”. Changes save immediately and survive restart.

**Stable** modules are available by default: Docker Sandboxes, Local Ports and
Remote HTTP. **Containers**, **Ports over SSH**, **Ports over Relay** and
**Relay hosting** are **Experimental**. The Containers, Ports over SSH and
Ports over Relay tabs start unavailable until enabled.
Relay hosting remains accessible in its Settings accordion and uses its own
**Enable relay** setting (off by default); it has no availability switch.

Module availability controls navigation only. It does not stop discovery,
connections, APIs or services. A module with any configured service, including an
unavailable service, cannot be disabled. Ports over Relay additionally requires
**Connect to relay** to be turned off, even if its status says Disconnected.
Blocked switches explain the reason, and the API enforces the same conditions.

Existing modules with configured services or an enabled relay connection remain
available. Once a module is used, it stays available after removing its last service
until explicitly disabled. Explicit choices survive maturity changes. Direct links
to unavailable module tabs lead to Settings. Re-enabling restores the original tab
and module identity.

## Ports over Relay

Build the relay and a Zelport containing the cloud feature:

```sh
make build-full
make -C ../zelport-relay build
../zelport-relay/bin/zelport-relay token --out ./relay-access.token
../zelport-relay/bin/zelport-relay --access-token-file ./relay-access.token
```

The relay defaults to `127.0.0.1:8090`. Use `--listen` for an explicit alternative,
`--max-connections` to change its 128-connection limit, `--log-level info|error`,
and `--version`. The token command creates a new owner-only file and refuses to
overwrite it. Read/copy that file locally into Zelport's **Relay access token** field;
do not put its contents in URLs or shell arguments.

For an isolated local trial, start two Zelport instances with separate `--config`
files and `--port` values. On the first instance, open **Ports over Relay → Settings**,
set `ws://127.0.0.1:8090` and the relay access token, and enable **Connect to relay**.
Leaving E2E secret blank on first setup generates a separate 32-byte secret.
Use **Reveal E2E secret** to copy it to the second instance, along with the relay
URL and relay access token. Do not copy the complete configuration/device identity.

Enable **Share all own services** on the provider. It publishes current and future
own routes; cloud imports are excluded. Administration is a separate automatic
capability, independent of this switch. On the other instance,
select an authenticated catalog service and import its original domain or another
unique `.localhost` name. Its URL uses the receiving instance's Localhost port.
No Aux listener is created. Sharing is off by default and never starts a sandbox.
Unavailable imports remain saved and return 503; reconnect is automatic. Disabling
sharing closes active requests and WebSockets before the settings save succeeds.

Connected peers with the same E2E secret automatically offer **full administration**
in the Ports over Relay tab, even when ordinary sharing is off. Open either
`http://<hostname-with-dots-replaced-by-hyphens>.zelport.localhost:8080` or
`http://<device-uuid>.zelport.localhost:8080`, using the receiving instance's port.
These names cannot be assigned to application routes. Conflicting hostnames are
unavailable; use the UUID alias. Old hostname ownership is retained after rename.
Known offline peers return 503; unknown aliases return 404.

The remote banner identifies the managed device. Service links point only to
matching imports on the receiving device; other services are shown without links.
Disabling the relay client closes remote administration too, so a remote save can succeed
while its response is lost. Check the resulting state before manually repeating it;
the client never retries mutations automatically.

**Upgrade together:** this feature uses protocol v3 and `/v3/ws`. Upgrade the relay
and every client before reconnecting; v1/v2 peers are incompatible. Existing v6
configuration, identities and secrets are preserved; an administration UUID is
allocated on startup. The historical experiment retains its separate protocol.

For a public deployment, keep the relay on loopback and put a WebSocket-capable
HTTPS reverse proxy in front of it. For example, a Caddy site can use:

```caddyfile
relay.example.com {
    reverse_proxy 127.0.0.1:8090
}
```

Point clients at `wss://relay.example.com`; certificate verification is mandatory.
WS and WSS both carry authenticated Noise admission plus independent peer E2E
channels. The relay knows its access token and opaque routing identifiers, but cannot read
the encrypted catalog or application contents and never receives the E2E secret. Group members trust one another; this is not a
multi-tenant account system. WSS remains recommended on public networks.

Peer sessions perform a fresh Noise handshake after 10 minutes or 1 GiB per
direction. Successful renewal preserves streams, including WebSockets; failed
renewal closes the session without replaying application requests. Catalog
encryption uses the long-lived E2E secret, so later compromise of that secret
exposes recorded catalogs. Outer relay admission does not receive fresh-DH renewal.
See [protocol v3](docs/ports-over-cloud-v1.md) and its
[verification record](docs/verification/2026-10-13-protocol-v3.md).

Imports pin provider keys and stable UUIDs. A source rename retains its identity;
a recreated route requires a new import. Rewrite Origin is optional, defaults off,
and changes only the import's own origin to the provider origin. Redirects,
cookies, HTML and application public-URL settings are unchanged.

Back up each instance's configuration securely: it contains its private identity,
both credentials and replay watermarks. Do not run copies of one device identity
simultaneously. A rolled-back provider sequence or changed key fails closed.
There is no automatic identity repair/reset. Use a fresh configuration/new identity
and recreate imports when recovery cannot preserve the original identity/sequence.
For a lost device, create a new relay access-token file, restart relay, and replace
both tokens on remaining devices. Old-group imports stay saved but unavailable;
remove/re-import them explicitly in the new group. The relay never receives the
E2E secret. Changing only the relay URL preserves group bindings.

See the [protocol](docs/ports-over-cloud-v1.md) and
[verification record](docs/ports-over-cloud-verification.md) for tested behavior,
resource bounds, clock requirements and remaining deployment/security-review limits.
This implementation has not received an independent cryptographic audit.

## Repository and verification

- `cmd/zelport`: common startup and tagged module factory registration.
- `internal/cloud/integration`: gateway/SDK/relay integration tests.
- Sibling `zelport-sdk-go`: shared protocol, encrypted channels and client.
- Sibling `zelport-relay`: public server and standalone command.
- `internal/feature`, `internal/app`, `internal/web`: contracts, registry/storage,
  connection leases, protected administration and frontend shell.
- `internal/modulekit`: generic module lifecycle/proxy helpers.
- `internal/modules/*`: independently selected implementation/API/UI.
- `internal/sbx`, `internal/containers`, `internal/sshpipe`: source adapters.
- `../../experiments/ports-over-cloud`: independent historical transport module;
  no imports or replace directives couple it to production.

```sh
make check          # Gateway core/full race tests and vet
make check-modules  # Individual/mixed/full builds and excluded-dependency checks
make build          # Full gateway executable
python3 scripts/cloud-demo.py # Isolated two-process cloud lifecycle demo
```

Root `go test ./...` includes package tests but does not traverse nested modules.
Binary inclusion is checked separately with `go list -deps ./cmd/zelport` and
selected tags. Use isolated configs/backends for verification; do not test against
personal saved routes. No frontend build or CDN is required.

See [DESIGN.md](DESIGN.md), [module contracts](docs/MODULE_CONTRACTS.md), and
[Ports over Relay requirements](PORTS_OVER_CLOUD.md). The cloud experiment's
[results](../../experiments/ports-over-cloud/RESULTS.md) are historical feasibility evidence. Integrated cloud verification is recorded
separately; neither is a security audit.

The header shows **Built-in Relay** only while hosting is enabled, with a green
indicator when listening and red on error. Disabled hosting has no indicator. **Ports over Relay**
remains visible while its module UI is available, with a gray indicator when disconnected and a red
indicator when an enabled connection fails. Select an indicator to see its runtime state, listener or
endpoint details, and a link to its settings. Hosting details include the connected
client count and whether access-token protection is disabled. `Listening` does not
confirm firewall reachability. With no connection indicators available, the header shows **Local**. Remote administration retains its separate device-identity banner.

### Remote HTTP module rename

The module ID and build tag are now `remote_http`. Before deploying this rename
with an existing configuration, stop the application and back up its configuration.
Rename `remote_ports` keys under `modules` and `module_availability` to
`remote_http`, and update route `module` values from `remote_ports` to
`remote_http`. Preserve all settings, route UUIDs, and other values. There is no
automatic legacy alias. The browser tab anchor remains `#remote`.

## Running as root

`zelport` refuses to start with effective UID 0 by default on macOS and
Linux. Run it as a regular user. To deliberately override this guard, pass
`--allow-root` or set `ZELPORT_ALLOW_ROOT=1`. Only the exact environment value `1`
enables the exception; other values do not. An explicit `--allow-root=false`
overrides the environment. Every allowed root invocation prints a warning to
stderr, regardless of the log level. `--help` and `--version` remain available
without an exception.

This prevents accidental privileged execution; it is not a security boundary
against an administrator. The check uses the process effective UID, not sudo
environment variables, and does not detect Windows administrator privileges.

Root execution can select a different configuration directory, SSH credentials,
and container environment. Built-in Relay follows the gateway process policy.

## Final product rename

The product and CLI output now use Zelport. The default configuration is
`$XDG_CONFIG_HOME/zelport/config.json` or `~/.config/zelport/config.json` on
macOS/Linux. Remote administration uses `*.zelport.localhost`; old administration
URLs are no longer generated. See [rename notes](docs/UPGRADING.md) before updating
an existing installation. Explicit `--config` paths continue to work.

## Native desktop clients

Native integrations such as Zelport Integration use the automatic read-only local API.
On Linux and macOS, the last-used port is recorded under `$XDG_STATE_HOME/zelport/endpoint.json`
or `~/.local/state/zelport/endpoint.json`. The Windows implementation uses
`%LOCALAPPDATA%\Zelport\config.json` and `State\endpoint.json`; native runtime
validation is pending. No integration flag or token is needed.
See [local API and migration](docs/DESKTOP_INTEGRATION.md).
Existing administration guards and relay boundaries remain unchanged.

Linux and Omarchy packaging: [distribution guide](contrib/distribution/README.md).

Licensed under the [MIT License](LICENSE).
