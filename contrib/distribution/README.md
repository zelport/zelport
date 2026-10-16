# Arch / Omarchy source package

The package name is `zelport`. It compiles the full gateway on the user's device
using Go. No precompiled binary variant is currently maintained for Arch.
Homebrew distribution is separate. The Omarchy plugin is also a separate project.

AUR account registration is currently paused. Until an AUR package is published,
`yay -S zelport` cannot work. Use the local package procedure below.

## Install the prepared package

On Omarchy, with base-devel and git installed:

```sh
git clone https://github.com/zelport/zelport.git
cd zelport/contrib/archlinux
makepkg -si
```

Review PKGBUILD before running it. `makepkg -si` installs missing build dependencies
through pacman, then compiles and installs the package. Run it as a regular user,
not root. It asks for privilege escalation when installing system packages.
The package needs Go 1.27 or newer. The installed binary does not need Go.

The recipe declares x86_64 and aarch64 targets and builds for the native host.
Do not use `arch=('any')`: the resulting executable is architecture specific.
Additional architectures require Go support and validation before being added.
See VERIFICATION.md for the exact scope of completed testing.

## Start the gateway

After stopping any existing gateway using the same port and configuration:

```sh
systemctl --user daemon-reload
systemctl --user enable --now zelport.service
```

The unit runs `/usr/bin/zelport` as the current user.
It preserves the usual configuration path and loopback port 8080. Installation
does not enable the service automatically or install the Omarchy plugin.
Use `systemctl --user restart zelport` after upgrading. Stop and disable the
service before removing the package. User configuration is not packaged or deleted.

## Prepare a new source package version

First publish SDK and relay tags and update go.mod/go.sum. Verify a standalone
build without a local module proxy, workspace, or private credentials. Tag the
reviewed gateway source. Do not replace published tags.

Download the exact GitHub source archive for the release and generate its recipe:

```sh
curl -fL https://github.com/zelport/zelport/archive/refs/tags/v0.1.0-preview.5.tar.gz \
  -o /tmp/zelport-source.tar.gz
python3 contrib/distribution/prepare.py --version 0.1.0-preview.5 \
  --source-archive /tmp/zelport-source.tar.gz --output bin/arch/0.1.0-preview.5
```

The generator verifies required source files, rejects workspace-local dependency
pins and replacements, and embeds the source archive checksum. It refuses to
overwrite a previous output. Preview versions use an Arch-compatible pkgver
without the upstream hyphen, while source URLs retain the exact upstream tag.

On Arch, in the output directory, run:

```sh
makepkg --printsrcinfo > .SRCINFO
makepkg --verifysource
makepkg --cleanbuild --syncdeps
namcap PKGBUILD
namcap zelport-*.pkg.tar.zst
```

After successful checks, copy PKGBUILD and .SRCINFO to `contrib/archlinux/` for
GitHub installation. When AUR registration reopens, publish those two reviewed
files to the `zelport` AUR repository. No binary or source archive is uploaded
to AUR. Keep the source archive checksum pinned, never use SKIP.
