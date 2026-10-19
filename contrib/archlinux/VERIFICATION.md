# Arch package verification

Verified on 2026-10-17 for Zelport App `0.10.0`.

- Source snapshot: tag `v0.10.0`, App commit `040a82b`, with archive prefix
  `zelport-0.10.0/`.
- Source asset: `zelport_0.10.0_source.tar.gz`.
- Source SHA-256:
  `4d72e6e1e668dde3d08f64193b11a085a44a12b8aad43e7a81ec4a3087533484`.
- `prepare.py` unit tests: passed.
- Native target: Omarchy `4.0.4-1`, Linux x86_64.
- Build toolchain: Omarchy Go `1.27.0-X:nodwarf5` and makepkg/pacman `7.1.0`.
- `makepkg --verifysource`: passed against the exact source asset.
- `makepkg --cleanbuild --nodeps --noconfirm`: passed in an isolated temporary
  directory without installing the package.
- Public SDK and Relay dependencies resolved at `v0.10.0` with the checksums in
  `go.sum`.
- Package check: `zelport --version` reported `0.10.0`; `zelport --help` passed.
- Package contents: executable, systemd user unit, license, third-party notices,
  and dependency license files only.

Not yet verified:

- a native aarch64 Arch build or runtime;
- installation into a persistent Arch host; and
- external package-catalog submission.

## Reproducing recipe inputs

Set `VERSION` to numeric SemVer without `v` and `REVIEWED_COMMIT` to the exact
reviewed commit:

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

The generator accepts stable numeric SemVer, verifies required source files,
rejects local module versions and `replace` directives, records the archive
SHA-256, and refuses to overwrite its output directory.
