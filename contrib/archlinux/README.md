# Arch Linux package

This directory owns the source package for Zelport App. The package compiles the
complete standard `zelport` executable and installs its systemd user unit.
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

Set `VERSION` to the numeric SemVer being prepared, without a `v` prefix.
`make release-ready` verifies and generates the immutable source archive,
`PKGBUILD`, and `.SRCINFO` together with the binary release assets:

```sh
: "${VERSION:?set VERSION to numeric SemVer without v}"
make release-ready VERSION="$VERSION"
```

To reproduce the packaging inputs separately, also set `REVIEWED_COMMIT` to the
exact reviewed commit, generate the source archive with the matching versioned
prefix, then generate the recipe from those bytes:

```sh
: "${VERSION:?set VERSION to numeric SemVer without v}"
: "${REVIEWED_COMMIT:?set REVIEWED_COMMIT to the reviewed commit}"
git archive --format=tar.gz --prefix="zelport-${VERSION}/" \
  --output="bin/arch/zelport_${VERSION}_source.tar.gz" "$REVIEWED_COMMIT"
python3 contrib/archlinux/prepare.py \
  --version "$VERSION" \
  --source-archive "bin/arch/zelport_${VERSION}_source.tar.gz" \
  --output "bin/arch/${VERSION}"
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
