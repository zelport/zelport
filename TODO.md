# Ideas and deferred work

This file records ideas discussed during development. Entries are not commitments
or instructions to implement them. Keep the context and intended benefit so they
can be revisited without reconstructing the original conversation.

## Gateway port in configuration

Recorded: 2026-10-16. Status: deferred.

Currently, the gateway port defaults to `8080` and can only be changed with the
`--port` CLI flag. Homebrew users must supply a custom service definition to
change it, which is cumbersome for a basic setting.

Consider storing the gateway port in `config.json`, with this precedence:

1. An explicitly supplied `--port` flag.
2. The configured port.
3. The default port `8080`.

Before implementing, decide how this fits the configuration schema and whether
the dashboard should expose the setting. Document restart requirements and how
users find the dashboard at its new address. Preserve loopback-only binding and
existing configurations that do not specify a port.

For now, retain the current CLI and Homebrew service behavior.

## Omarchy packaging and future binary packages

Recorded: 2026-10-16. Status: source package selected, binary packages deferred.

The primary installation path for Omarchy will be `yay -S zelport`. This package
will build from source on the user's device. Users should not need to clone the
repositories or invoke the compiler manually. The package manager will handle
build dependencies and compilation through the package recipe.

SDK and relay dependencies now use publicly resolvable version tags. The source
package recipe is available for local installation from GitHub. AUR
publication remains deferred while new account registration is closed. Verify
installation on each supported target architecture. Source compilation does not by
itself guarantee support for every architecture.

Later, consider an optional `zelport-bin` package with precompiled binaries for
selected architectures to reduce installation time and build dependencies.
Keep `zelport` as the documented general installation path. A binary package
does not automatically fall back to source compilation when no compatible
binary exists. Clearly document supported architectures and the source option.

## Non-systemd installer service integration

Recorded: 2026-10-16. Status: deferred.

The shell installer installs the Linux release binary without a reachable
systemd user manager and prints manual startup instructions. Add explicit
service adapters for systems such as Alpine with OpenRC after validating their
user-service and privilege requirements. Preserve the regular-user gateway
model. Do not infer a working user manager merely from the presence of a CLI.
Test native Alpine operation, login startup and upgrades before advertising
automatic service support. The Omarchy plugin remains independently installed.

## Windows compatibility

Recorded: 2026-10-16. Status: planned, not advertised as supported.

Develop shared gateway functionality with Windows in mind. Keep native paths,
service management, file permissions and process lifecycle outside platform-neutral
logic. Full Windows cross-compilation should remain possible, but runtime testing
is required before release support can be claimed.

Local integration now has platform-specific paths: Windows uses LocalAppData
for config.json and State/endpoint.json. Protected file DACLs and same-volume
replacement are implemented, with native Windows tests added but not yet run.
The endpoint is published automatically on Windows, Linux and macOS. Validate
Windows ACLs, replacement behavior and existing APPDATA configuration migration
before advertising runtime support.

Validate installation, automatic startup, shutdown, configuration persistence,
local HTTP access and the availability of each external backend on Windows.
