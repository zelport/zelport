# Arch package verification

Verified on 2026-10-17 for Zelport App `0.10.0`.

- Source snapshot: App commit `ee8556c` with archive prefix
  `zelport-0.10.0/`.
- Source asset: `zelport_0.10.0_source.tar.gz`.
- Source SHA-256:
  `16f3bfb12fb41c56fc1e6090133d9da27bf63b15a40fd100d5e1884fef9cd420`.
- `prepare.py` unit tests: passed.
- `makepkg --printsrcinfo`: passed in `archlinux:latest`.
- `makepkg --verifysource`: passed against the exact local source asset.
- `makepkg --cleanbuild --nodeps`: passed in a disposable x86_64 Arch container
  emulated on an arm64 macOS host.
- Build toolchain: Arch Go 1.27.2 and GNU Make 4.4.1.
- Unpublished SDK and Relay `v0.10.0` dependencies were supplied through the
  read-only local Go proxy with the checksums recorded by the runtime plan.
- Package check: `zelport --version` reported `0.10.0`; `zelport --help` passed.
- Package contents: executable, systemd user unit, license, third-party notices
  and dependency license files only.

Not yet verified:

- a native x86_64 Arch host;
- an aarch64 Arch build or runtime;
- dependency resolution from public `v0.10.0` module tags;
- installation into a persistent Arch host; and
- external package-catalog submission.
