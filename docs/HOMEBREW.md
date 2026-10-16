# Homebrew distribution

The repository prepares binary archives and a formula for the intended
`zelport/tap` tap. It does not publish repositories, upload releases, register
services or modify an existing Homebrew installation.

## Prepare a release

From a standalone checkout:

```sh
make release VERSION=0.1.0
```

Dependencies use public Go module versions. Python 3 and the Go version required
by `go.mod` are needed. The full
feature list comes from `FEATURE_TAGS` in the Makefile. Builds embed the release
version and UTC build time. They use `CGO_ENABLED=0` and target macOS and Linux
on ARM64 and x86-64. No Go toolchain is required on the installing machine.

The output directory `bin/homebrew/0.1.0/` contains four archives, `SHA256SUMS`
and `Formula/zelport.rb`. Each archive contains the executable, MIT license and third-party notices.
The formula uses SHA-256 hashes computed from those exact archives. Existing
output directories are rejected rather than overwritten.

This is separate from the existing [Arch/Omarchy packaging](../contrib/distribution/README.md).
When publishing both sets of assets, merge their checksum entries into one
release manifest rather than replacing one `SHA256SUMS` file with the other.

Before publication, test each archive on its target platform. Cross-compilation
alone does not verify runtime support. Retain the exact generated archives when
publishing, as rebuilding changes their checksums.
Review third-party dependency licenses and required notices before distribution.

Publication is a separate maintainer operation:

1. Upload the four archives and `SHA256SUMS` to the `v0.1.0` release in
   `zelport/zelport`.
2. Put the generated `Formula/zelport.rb` in `zelport/homebrew-tap`.
3. Verify `brew install zelport/tap/zelport` and `brew test zelport` on each
   supported platform before announcing availability.

Neither destination is created automatically. A formula pointing to unpublished
release assets cannot be installed. The tap is separate because Homebrew maps
`zelport/tap` to the `zelport/homebrew-tap` repository.

## Run as a user service

```sh
brew install zelport/tap/zelport
brew services start zelport
brew services info zelport
```

`brew install` only installs files. `brew services start` starts the application
and enables startup at login. Homebrew uses launchd on macOS and systemd on
Linux. A Linux user service requires an available systemd user session. Operation
before login or after logout needs separate administrator configuration.

Do not use `sudo`. The service does not pass `--allow-root` and Zelport rejects
root execution by default. It keeps the default loopback listeners on port
8080. Stop any foreground instance on that port before starting the service.
The formula uses the stable Homebrew `opt` path so upgrades do not pin the
service to an old Cellar version. Restart the service after upgrading.

The service supervisor restarts failed processes and allows 15 seconds for shutdown.
Use `brew services stop zelport` to stop it and disable startup at login.
Use `brew services restart zelport` to restart it.

## Configuration, environment and logs

Installation and uninstallation do not create, replace or delete user
configuration. The service uses Zelport's usual configuration path:
`~/.config/zelport/config.json`, or `$XDG_CONFIG_HOME/zelport/config.json` when
an absolute `XDG_CONFIG_HOME` is present in the service environment.

Service managers do not automatically inherit an interactive shell's settings.
Before switching an existing setup to a service, confirm its configuration path
in the startup log. If you use a custom XDG directory or `--config`, provide that
setting in a custom service definition and start it using
`brew services start zelport --file=/absolute/path/to/service`. Use a launchd
plist on macOS or a systemd unit on Linux. Keep the same user identity and
configuration path to preserve routes and credentials.

The supplied PATH includes Homebrew's bin and sbin directories, standard system
paths and `/usr/local/bin`. Docker, Podman and sbx remain optional runtime tools.
Commands installed elsewhere need an explicit service PATH. SSH agent sockets,
custom Docker contexts and other shell-only settings may also need service
environment configuration. Do not copy secrets into the formula.

Both output streams go to `$(brew --prefix)/var/log/zelport.log`. The formula does
not configure log rotation. The dashboard remains http://localhost:8080.

On Linux without systemd, use the binary with the distribution's service manager
under the intended user account. Homebrew services does not provide OpenRC
integration.

## Local Homebrew installation test

The sibling `homebrew-tap` repository contains a local test helper. It validates
release checksums and changes only the installed tap's formula to local archive
URLs. See its README for the full sequence. The source formula keeps its intended
GitHub release URLs. This allows testing `brew install zelport` before publishing.

Homebrew versions with tap trust enforcement require explicitly trusting the
formula when using the short install command after adding the tap:

```sh
brew trust --formula zelport/tap/zelport
brew install zelport
```

Trust only the reviewed formula, not the entire tap. Local test installations
use `0.0.0-homebrew-test` and are not public releases.
