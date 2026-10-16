# Local integration API

Zelport exposes a read-only local API automatically on its existing HTTP listener.
No startup flag, access token, runtime directory or additional listener is needed.
The API is intended for native clients such as the Omarchy plugin.

## Endpoint hint

On Linux and macOS after startup, Zelport atomically writes `{"port":8080}` using the actual listener
port to `$XDG_STATE_HOME/zelport/endpoint.json`. If XDG_STATE_HOME is unset, empty
or relative, the path is `~/.local/state/zelport/endpoint.json`.
On Windows, the endpoint is `%LOCALAPPDATA%\Zelport\State\endpoint.json` and the
default configuration is `%LOCALAPPDATA%\Zelport\config.json`. If LOCALAPPDATA is
unset, the native Windows LocalAppData known folder is used. A relative value is
rejected. APPDATA and XDG variables do not select these Windows paths. Existing
preview files under APPDATA are not moved automatically; use --config to select
an existing configuration explicitly.

Windows files are created with a protected DACL granting the current user and
Local System access before any data is written. Replacement uses a same-volume
Windows move with replacement, without deleting the previous file first. Native
Windows tests are required before runtime support can be advertised.

This location is independent of `--config`. The file is mode 0600 on Unix and remains
on shutdown as a last-used endpoint hint. It is not a liveness or identity record.
An unsuccessful write logs a warning without stopping the gateway.

Clients read this hint and call the API. If the hint is missing or invalid, use
8080. If a nondefault port fails or returns an incompatible response, try 8080
once. Do not scan ports, inspect processes or enumerate instances. The plugin
reads the hint on every poll, so changed ports are picked up automatically.
The gateway and desktop session should share the same HOME and XDG_STATE_HOME.
A single gateway per user is the supported desktop integration scenario.

## HTTP contract

`GET /api/integration/v1/state` returns `product: "zelport"`, `api_version: 1`,
capabilities, application version, administration URL, service summaries and
connection summaries. No bearer token or instance ID is required or returned.
Services include stable route IDs, domains, source IDs/names, states, URLs and
optional display-only `data_flow` text. Missing modules appear as `unsupported`.
The response does not include editable settings or the dashboard CSRF token.

Requests must use the exact `localhost` authority, including the configured port
unless it is 80. Only GET is allowed. Requests carrying Origin are rejected, CORS
is not enabled and relay administration cannot access this endpoint. HTTP responses
are JSON with no-store and nosniff headers. Application hosts retain their own
paths, including this path.

This follows the existing local dashboard trust model. Loopback does not isolate
users on the same machine. The endpoint is read-only, not an additional user
authentication boundary. Existing dashboard mutation protection remains unchanged.

Clients must validate the API version, product and response structure before
showing availability. Service launch URLs must be HTTP `.localhost` addresses
on the selected port. The administration URL must use exact localhost. Do not
follow redirects or use proxy environment variables for these requests.

## Migration from the token-based preview

Remove `--desktop-integration` from custom service definitions. The packaged
Arch user unit now invokes `/usr/bin/zelport` without this option. Do not replace
an existing custom configuration path or port while updating a unit. Old preview
plugins require an update together with the gateway. Old runtime descriptors are
no longer read or written and can be left for the login session to clean up.

The plugin does not start, stop, install or configure the gateway. Upgrade the
application and plugin, reload a changed systemd unit and restart the service:

```sh
systemctl --user daemon-reload
systemctl --user restart zelport.service
```

A manual gateway can still be started with `zelport` and optional `--port` and
`--config` arguments. No systemd or XDG_RUNTIME_DIR dependency is introduced.

## Native Windows validation

Before declaring Windows support, run on Windows as a regular user:

```powershell
go test ./internal/userpaths ./internal/privatefile ./internal/endpoint
```

These tests cover LocalAppData selection, protected file ACLs before writing,
ACL retention after replacing an existing file, failed replacement preserving
existing contents, and repeated endpoint publication. Cross-compiling these
tests does not count as execution. Also validate initial configuration creation
and subsequent edits through the running gateway on NTFS.
