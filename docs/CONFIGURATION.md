# Configuration

Use the administration UI to create and edit routes. The default file is
`$XDG_CONFIG_HOME/zelport/config.json` or `~/.config/zelport/config.json` on
macOS and Linux, and the native per-user configuration directory on Windows.
Pass `--config` to select another path.

The schema is exactly version `1`. A missing file is created as a valid empty
configuration. Other versions, unknown fields and invalid module settings are
rejected; Zelport does not import or rewrite earlier schemas.

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

Writes are atomic and owner-only. Route UUIDs survive edits, and concurrent
changes are rejected instead of silently overwriting one another.

Service sources are visible in the Services menu and Add service by default,
including experimental sources. Settings → Service sources controls visibility only.
Explicit choices in `module_availability` survive restarts. Showing a source does
not enable its connections, sharing or listeners.


## Remote file access

`remote_file_access` is `false` unless set. While false, remote administration
cannot change the Ports over SSH configuration path or the Remote HTTP CA path and
cannot read or edit those files; it still manages services and other settings.
Only local administration can change it, on the Relay page under Configuration → Allow remote file access.

## Built-in Relay

The default relay port is `8070`. Set `modules.built_in_relay.port` to use a
different port. An explicitly saved port is preserved, including the previous
default `8090`.

## Ports over SSH

The Ports over SSH source page holds its configuration. The configuration file path
is stored under `modules.ports_over_ssh.ssh_config_path`. Leave it blank for
`~/.ssh/config`, or enter an absolute path or a path beginning with `~/`.
Paths containing `%` are unsupported because OpenSSH versions interpret them differently.
The saved path controls both host discovery and SSH connections. Path changes
close the module's existing SSH connections so later requests use the new file.
User configuration remains referenced by the hardened SSH wrapper, including for
ProxyJump connections. File contents and identity keys are not copied into Zelport's configuration.
The selected configuration file is returned only when **Edit file** is opened.
Referenced identity files are not exposed by the editor.

Save the selected path before choosing **Edit file**. The integrated editor is
available locally and through authenticated remote administration. Saving reloads
host discovery and closes existing SSH pools so later connections use the saved
contents. **Refresh hosts** also reloads aliases after external file changes.


## What services receive

Zelport is a transparent reverse proxy. Toward a service it:

- keeps `Host` for Local, SSH and remote administration, and sets the target host
  for Remote HTTP and relay imports;
- sets `X-Forwarded-For` (the real client, never a client-supplied value),
  `X-Forwarded-Host` and `X-Forwarded-Proto`;
- removes `Proxy-Authorization` and hop-by-hop headers, and passes `Authorization`,
  `Cookie` and `Origin` (rewritten only with Use target origin);
- adds `X-Zelport-Loop` for loop detection (below). Remote HTTP targets beyond
  loopback (`localhost`, `*.localhost`, `127.0.0.0/8`, `::1`) never receive it,
  because they cannot lead back to a Zelport listener;
- adds itself to `Via` as `<version> zelport` (RFC 9110), after any earlier hops,
  where the version is the HTTP version the request arrived with (`1.1`, `2`);
- sets `X-Request-Id` (below);
- passes `traceparent` and `tracestate` (W3C Trace Context) unchanged. Zelport is
  not a span in the trace;
- never sends the signed relay trace (`X-Zelport-Relay-Trace`, device ID and group
  fingerprint) to a service. It only travels inside the relay tunnel, for older
  Zelports in the group.

Loop detection is stateless. `X-Zelport-Loop` is `<nonce>.<mark>.<mark>…`: the
first Zelport picks a random 16-byte nonce for the request, and every route the
request passes adds an 8-byte mark, SHA-256 of the nonce, route ID, hostname and
port. A request that comes back to a route it passed answers 508 Loop detected,
and so does a ninth route. The header names no device, group or route, and marks
of two requests cannot be linked. It works without relay (a local proxy that leads
back to Zelport) and with older Zelports in the path. A proxy that drops every
Zelport header hides a loop, which then ends only by timeouts.

`X-Request-Id` names one request in Zelport and service logs. Zelport keeps an ID
the request already carries (visible ASCII, at most 128 characters), so it stays the
same across several Zelports and other proxies. Without one it takes the trace ID
of a valid `traceparent`, so the ID matches the trace in OpenTelemetry tools, and
otherwise picks a random 32-digit hex ID. Nothing is stored.

Responses stream immediately (SSE and WebSocket work). Zelport adds itself to their
`Via` and adds `X-Request-Id` when the service does not answer with its own. Zelport's
own pages (unknown address, unavailable service, loop detected) carry `X-Request-Id`
too. Otherwise responses pass unchanged. A service
has 5 seconds to accept the connection and 5 minutes to start its answer.

## Remote HTTP outbound connections

Remote HTTP Configuration applies to all Remote HTTP services. The module stores
`ca_cert_path`, `proxy_url`, `proxy_username` and `proxy_password` under
`modules.remote_http`. Defaults use system certificate trust, an optional
`additional-ca.pem` bundle, and direct connections.
The module does not inherit proxy environment variables.

`ca_cert_path` is an absolute or `~/` path to a PEM certificate bundle. Leave
it blank to select `additional-ca.pem` in the platform default Zelport configuration
directory (typically `~/.config/zelport/additional-ca.pem`, respecting XDG).
A missing default file means system trust only. An existing invalid default file
is an error, as is a missing explicitly selected file.
Its certificates supplement system roots. It must be a regular file no larger
than 1 MiB, containing certificates without private keys. Certificate hostname
checks remain enabled. Invalid files and missing explicitly selected files fail
closed for outbound traffic and appear as a recoverable Configuration error. Updating the file applies to
subsequent connections. File contents are returned only when **Edit file**
is opened, and are never included in polling state.

`proxy_url` accepts HTTP, HTTPS, SOCKS5 and SOCKS5H endpoints. Use separate
authentication fields, without credentials, query parameters or fragments in the
URL. SOCKS5 and SOCKS5H resolve target names through the proxy. HTTP proxying uses
CONNECT for HTTPS origins and supports streaming and WebSockets. HTTPS proxy
certificates remain verified even when a service disables origin TLS verification.

Proxy passwords are persisted with the configuration's owner-only permissions
and never returned in module state or diagnostics. Leave the password blank to
keep it for the unchanged proxy endpoint and username. Changing either requires
re-entering or explicitly clearing the stored password. Clearing the proxy URL
returns to direct connections and removes stored proxy credentials. Saving new
network settings closes active Remote HTTP requests and upgraded connections.

Use **Edit file** to edit the saved CA path. Unsaved settings must be saved or
discarded first. Saving validates the certificate bundle and refreshes its status.
**Reload certificates** refreshes validation after external file changes without
discarding configuration drafts. File changes affect subsequent connections.


## Browser configuration editor

**Edit file** opens the saved CA or SSH configuration file on the
administered instance, including through the authenticated relay administration
tunnel. It is a plain text editor with **Cancel**, **Save**, and Ctrl/Cmd+S.
Save or discard pending settings changes before opening it. The modal shows the
file path and identifies the remote host. File text stays in the open dialog
only and is cleared on close; it is not stored in browser storage or logs.

Opening a missing file creates nothing. Saving creates missing directories and
a private file. Existing permission bits are retained; writes replace a complete
file atomically. Symlinks to regular files retain the link and edit its resolved
target. Dangling links, nonregular files, invalid UTF-8, NUL and text above 1 MiB
are rejected. CA saves also require a valid certificate-only PEM bundle. Invalid
existing PEM remains readable for repair. SSH text is not executed for validation.

If the file, resolved target or settings changed after opening, Save refuses to
overwrite it and keeps the draft. Copy the draft or explicitly reload the file.
Closing modified text requires an explicit discard choice. External writers do
not share Zelport's locks; a filesystem change after the last comparison can
still race the atomic replacement. General filesystem access and editing SSH
Include or identity files are not provided.


## Local Ports discovery

Local Ports Configuration controls port suggestions through
`modules.local_ports.discovery_enabled` (default true), `ignored_ports` and
`ignored_processes` (both default empty). For example:

```json
{"discovery_enabled": true, "ignored_ports": ["22", "8000-8099"], "ignored_processes": ["sshd", "postgres"]}
```

Use comma-separated lists in the UI. Double-quote process names containing commas
and escape a quote by doubling it. Ports must be 1–65535; ranges are inclusive
and ascending. Each list supports at most 128 entries. Processes match exact
reported names case-insensitively, with at most 256 UTF-8 bytes per name and no
control characters. There are no wildcards. A process filter hides a port only
when all its reported owners are ignored; unknown or mixed owners remain visible.

These settings affect suggestions, never configured services or manual ports.
Ignored listeners still supply status/exposure metadata for configured services.

Sandbox and container runtimes publish many ports from one proxy process. When a
listener belongs to such a process, Zelport asks that runtime once and shows the
port as `<process> · <workload>`:

| Process | Runtime call |
|---|---|
| `sbx` | `sbx ls --json` (Docker Sandboxes) |
| `gvproxy`, `rootlessport`, `pasta`, `slirp4netns` | `podman ps` |
| `com.docker.*`, `docker-proxy`, `vpnkit-bridge`, `OrbStack Helper`, `limactl` | `docker ps` |

Each call has a 1.5 s timeout and is cached for 10 s, and Rescan clears the cache.
No call is made when no listener belongs to these processes. A new service picked
from such a port takes the workload name. Ports that no workload publishes keep
the process name, and `ignored_processes` still matches the process name.
Containers with host networking or Docker on Linux without the userland proxy
show their own process or no listener at all.

When two processes hold one port, for example a sandbox on `127.0.0.1` and a
container proxy on all interfaces, connections to `localhost` reach the process
bound to the loopback address itself. Zelport labels the port with that process,
and the non-loopback warning names the other one.
Disabling discovery stops OS scanning entirely and withholds listener indicators.
Filters stay saved while disabled. Open port pickers update after settings change
without replacing the manual port, search text or accepting stale scan results.
Settings use the same revision and Origin/CSRF protection locally and through
remote administration. Concurrent edits retain the draft for review.

### Relay E2E secrets

Ports over Relay accepts a new E2E secret of 32–256 Unicode code points, including
spaces and special characters. Input is preserved exactly without trimming or
normalization. Previously saved shorter secrets remain valid when loading
configuration or saving unrelated settings. Generate creates 40 uniformly random
alphanumeric characters (`a–z`, `A–Z`, `0–9`). The same
exact secret must be used on each device; it is distinct from the relay access
token. Generated base64url keys keep their existing group identity.

For other input, Zelport derives the protocol group key with Argon2id (3 passes,
64 MiB, 4 lanes, 32-byte output), using the versioned domain salt
`zelport/e2e-secret/argon2id/v1`. Fixed parameters and salt allow independently
configured devices to derive the same key. Choose an unpredictable secret: a
minimum length cannot establish entropy, and the common salt permits shared
precomputation. See the [Go Argon2 documentation](https://pkg.go.dev/golang.org/x/crypto/argon2).

The module stores the original input as optional `group_secret` alongside the
derived `group_token` in the owner-only configuration file. They are validated
as a pair. State APIs expose neither; explicit Reveal/Copy returns the original
input. Entering an existing raw key clears any previous `group_secret`. Newly
generated alphanumeric secrets use the same derivation and storage as
user-entered text.
