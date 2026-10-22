# Homebrew distribution

The rolling `zelport/homebrew-tap` repository packages Zelport App releases. It
has no independent release tag or product version. The formula follows the
canonical App release (`v0.10.0` here); its required prefix-free `version` field
is derived automatically. Packaging-only corrections use Homebrew's numeric
`revision`.

## Release inputs

`make release-ready VERSION=v0.10.0` runs the complete source and module checks,
then creates and verifies four binary archives, the source and Arch packaging
assets, and `SHA256SUMS` in `dist/0.10.0`:

- `zelport_0.10.0_darwin_arm64.tar.gz`
- `zelport_0.10.0_darwin_amd64.tar.gz`
- `zelport_0.10.0_linux_arm64.tar.gz`
- `zelport_0.10.0_linux_amd64.tar.gz`
- `zelport_0.10.0_source.tar.gz`
- `PKGBUILD`
- `zelport-0.10.0.SRCINFO`

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
After publication, `make verify-published-release VERSION=v0.10.0` independently
downloads the public release and verifies its exact asset set and checksums.

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
Configure `PATH` and SSH agent availability explicitly when Ports over SSH is
used. System SSH is an external tool and is not bundled by the formula.

Logs are written to Homebrew's `var/log/zelport.log`. Zelport configuration
remains in the regular per-user configuration directory and is not removed by
formula uninstall.

## Local formula fixture

Local formula development uses neutral version `0.0.0`; it is not a Zelport
product release. Generate fixture archives locally, verify their hashes and
rewrite only the installed test tap copy. Never commit local `file://` URLs or
fixture checksums to the source tap.
