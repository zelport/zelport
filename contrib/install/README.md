# Platform-aware installer

Run the root `install.sh` with Bash as a regular user. Never run the whole
installer with sudo. The script is self-contained and can also be supplied on
Bash's standard input. Prefer downloading and inspecting it before execution.

```sh
bash install.sh
# Install without configuring, enabling, starting or restarting a service:
bash install.sh --no-start
```

After this script is published, the download form is:

```sh
curl -fsSL https://raw.githubusercontent.com/zelport/zelport/main/install.sh -o install.sh
less install.sh
bash install.sh
```

The optional pipe form is `curl -fsSL
https://raw.githubusercontent.com/zelport/zelport/main/install.sh | bash`.
It executes the downloaded script directly. The script's download checksums
verify release assets, not the installer itself and not an independent signature.

## Platform behavior

| Platform | Action |
| --- | --- |
| macOS | Print `brew install zelport/tap/zelport` and `brew services start zelport`; no installation |
| Windows shells (MSYS, MinGW, Cygwin) | Print that support is coming soon; no installation |
| Arch and Arch-derived distributions, including Omarchy | Fetch a commit-pinned, SHA-256-verified PKGBUILD and run `makepkg --syncdeps --install` |
| Other Linux distributions | Install the release binary for x86_64 or aarch64 into `~/.local/bin/zelport` |
| Linux without a reachable systemd user manager | Install normally and print manual startup instructions |

WSL is detected as Linux. Native PowerShell cannot execute this Bash script.
Unsupported operating systems and architectures fail before downloading files.
Bash, curl, tar and either sha256sum or shasum are required for Linux installation,
with ordinary shell tools such as awk, sed, install and mktemp. Minimal images
may need these tools installed first. Arch requires makepkg from base-devel and
pacman. Missing build dependencies are installed through makepkg's normal
interactive package-manager workflow, which may request sudo. A Go compiler is
needed only for the Arch source build.

The Arch recipe is pinned to a reviewed commit outside the gateway release tag:
the final recipe records the checksum of that tag's source archive. This avoids
trying to embed an archive's checksum inside the archive itself. The recipe
verifies that source archive and Go verifies module dependencies normally.
Other Linux installations verify the selected archive against the release's
SHA256SUMS. Downloads require HTTPS, including redirects. Only the selected
binary and license notices are streamed out of the archive, never arbitrary
archive paths. Notices are stored under `$XDG_DATA_HOME/zelport`, defaulting to
`~/.local/share/zelport`.

## Services, updates and existing installations

The installer checks `systemctl --user show-environment`, not merely whether the
systemctl executable exists. A reachable user manager enables startup for the
user session. It does not enable lingering or configure a root system service.
A non-systemd host receives manual instructions; OpenRC and other service-manager
adapters are deferred.

On the binary path, the generated unit is stored under
`$XDG_CONFIG_HOME/systemd/user/zelport.service`, defaulting to `~/.config`. It runs
`%h/.local/bin/zelport` with the application's normal configuration defaults.
Existing custom units and service symlinks are not overwritten. Use `--no-start`
when maintaining a custom service. Arch uses the package-provided user unit.

Installation preserves gateway configuration, endpoint hints and runtime
identities. An active managed user service is restarted only when the binary,
package or managed service unit changes.
If ss is available, an occupied default port prevents startup before installation.
When ss is unavailable, a failed new service is stopped to avoid a restart loop
and the installer reports the journal command. No unrelated process is killed.
The status check confirms systemd activity, not individual backend health.

A Zelport command found at another path blocks installation rather than creating
a second copy. A symlink at the binary destination also blocks replacement.
Use the original package manager for Homebrew or other managed installations.
`--no-start` leaves service state untouched, even when a service is already active.
The Arch package still includes its standard unit file, but it is not enabled
or started by this option.
Repeated runs skip installation when the user-local executable reports the pinned
release via `--version`. On Arch, the verified recipe's package version and
revision are compared with pacman's installed package, and `pacman -Qk` checks
for missing package files before skipping the build. Arch still downloads the
small pinned recipe, but does not rebuild or reinstall a matching complete package.
An unchanged active service is left running. Missing managed units, stopped
services and disabled startup are configured as needed unless `--no-start` is set.

A different installed version is replaced with the pinned version. Inspect the
pin before using an older installer against a newer installation. With
`--no-start`, an updated binary takes effect only when the process is restarted.

The Omarchy plugin is deliberately separate:

```sh
omarchy plugin add https://github.com/zelport/zelport-omarchy.git --enable
```

## Installer version

Every execution prints `Zelport installer <version>` before argument and platform
handling, including help and failed invocations. The installer version is
independent of the application version it installs. Increase
`ZELPORT_INSTALLER_VERSION` for every published script change, including release
pins. Use patch increments for fixes and pin updates, minor increments for
compatible features, and major increments for breaking behavior.

## Maintenance and verification

Update `ZELPORT_RELEASE`, `ZELPORT_ARCH_REF` and `ZELPORT_ARCH_SHA256` together
only after the corresponding binaries and final Arch recipe are published.
Never move an existing release tag or silently replace its assets.
Update the release fixtures in the installer tests with the version pins.

```sh
bash -n install.sh
python3 -m unittest discover -s contrib/install -v
make check-packaging
```

Tests use temporary home directories, generated archives and fake platform,
network, package-manager and service commands. They exercise failure and update
paths without installing packages or changing a running gateway. They do not
establish native compatibility with every distribution. Native Arch compilation
and real systemd startup require separate disposable-host verification.
