# Homebrew distribution

The rolling `zelport/homebrew-tap` repository packages Zelport App releases. It
has no independent release tag or product version. The formula version follows
the App (`0.10.0` for this release); packaging-only corrections use Homebrew's
numeric `revision`.

## Release inputs

`make release VERSION=0.10.0` creates four archives and `SHA256SUMS`:

- `zelport_0.10.0_darwin_arm64.tar.gz`
- `zelport_0.10.0_darwin_amd64.tar.gz`
- `zelport_0.10.0_linux_arm64.tar.gz`
- `zelport_0.10.0_linux_amd64.tar.gz`

Each archive contains executable `zelport`, `LICENSE` and
`THIRD_PARTY_NOTICES.txt`. The generated formula declares `version "0.10.0"`,
uses release tag `v0.10.0`, records the measured archive hash for each platform
and omits `revision` for the first package.

The release workflow builds this bundle once and runs the executable from each
archive on its matching native macOS or Linux, ARM64 or x86-64 GitHub-hosted
runner. Only a bundle that passes all four native acceptance jobs can enter the
draft publication step. The workflow downloads that draft again, verifies
`SHA256SUMS` and compares every asset byte-for-byte before making it public.
It never rebuilds an archive after native testing.

Publish the tap formula only after every referenced release URL is immutable and
its downloaded bytes match the candidate checksum. Run `brew style`, strict
online audit, install and formula tests in a clean Homebrew environment.
The tap and Zelport Installer consume the immutable App release assets; their
automation must not compile replacement binaries.

## Install and service

```sh
brew install zelport/tap/zelport
brew services start zelport
```

Run both commands as the regular user. Installation alone does not start
Zelport. The service uses launchd on macOS and systemd user services on Linux.
Do not run a manual instance and a managed service on the same Localhost port.

Useful commands:

```sh
brew services info zelport
brew services restart zelport
brew services stop zelport
```

Service processes do not inherit the complete interactive shell environment.
Configure `PATH`, container engine context and SSH agent availability explicitly
when their corresponding service sources are used. Docker, Podman, `sbx` and
system SSH are supported external tools and are not bundled by the formula.

Logs are written to Homebrew's `var/log/zelport.log`. Zelport configuration
remains in the regular per-user configuration directory and is not removed by
formula uninstall.

## Local formula fixture

Local formula development uses neutral version `0.0.0`; it is not a Zelport
product release. Generate fixture archives locally, verify their hashes and
rewrite only the installed test tap copy. Never commit local `file://` URLs or
fixture checksums to the source tap.
