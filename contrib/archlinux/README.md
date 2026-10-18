# Arch Linux package

This directory owns the source package for Zelport App. The package compiles the
standard seven-module `zelport` executable and installs its systemd user unit.
Docker, Podman, nerdctl, OpenSSH and `sbx` remain third-party tools that are only
needed when their corresponding integrations are used.

## Build and install

Review `PKGBUILD`, then run the following as a regular user on Arch Linux:

```sh
makepkg --syncdeps --install
```

The package is architecture-specific and currently declares `x86_64` and
`aarch64`. Installation does not start Zelport or alter its configuration.

```sh
systemctl --user daemon-reload
systemctl --user enable --now zelport.service
```

Use `systemctl --user restart zelport.service` after an upgrade. Stop and disable
the service before removing the package.

## Prepare a release recipe

`make release VERSION=0.10.0` generates the immutable source archive,
`PKGBUILD`, and `.SRCINFO` together with the binary release assets. To reproduce
the packaging inputs separately, generate the source archive from the reviewed
commit with the exact `zelport-0.10.0/` prefix, then generate the recipe from
those bytes:

```sh
git archive --format=tar.gz --prefix=zelport-0.10.0/ \
  --output=bin/arch/zelport_0.10.0_source.tar.gz <reviewed-commit>
python3 contrib/archlinux/prepare.py \
  --version 0.10.0 \
  --source-archive bin/arch/zelport_0.10.0_source.tar.gz \
  --output bin/arch/0.10.0
```

The generator accepts stable numeric SemVer only, verifies required source files,
rejects local module versions and `replace` directives, and records the measured
archive SHA-256. It refuses to overwrite an existing output directory.

In the generated directory, run:

```sh
makepkg --printsrcinfo
makepkg --verifysource
makepkg --cleanbuild --syncdeps
```

Copy only the generated `PKGBUILD` and `.SRCINFO` back to this directory after
validation. Do not use `SKIP`, replace a published source asset, or submit the
package to an external catalog without a separate decision. See
[`VERIFICATION.md`](VERIFICATION.md) for the current validation scope.
