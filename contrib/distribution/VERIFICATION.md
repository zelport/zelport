# Source package verification

Verified on 2026-10-16 for 0.1.0-preview.4.

- Workspace check, check-modules and build passed with publicly resolvable SDK
  and relay versions. No sibling source checkout is required for package builds.
- Source recipe generator tests passed, including checksum generation,
  invalid archives, local dependency rejection and overwrite protection.
- makepkg built the full gateway in an official x86_64 Arch Linux container
  under emulation on an arm64 macOS host. Dependencies downloaded from public
  Go infrastructure. This is not a native Omarchy desktop acceptance test.
- The final recipe downloaded the public GitHub v0.1.0-preview.4 source archive,
  passed SHA256 verification and rebuilt successfully. .SRCINFO was generated
  by makepkg.
- The package was installed with pacman inside that disposable container.
  The installed executable served the dashboard, created configuration version 6
  with mode 0600 and exited successfully on SIGTERM.
  The packaged binary also passed the new local API/helper test, including port
  changes, missing/stale endpoint fallback and nonfatal state-file write failure.
- systemd-analyze verified the installed user unit. Service enablement and login
  startup still require a real user session on the target device.
- namcap reported no errors. It warned about missing FULL RELRO in the Go-linked
  executable and a possibly unnecessary ca-certificates dependency. Certificates
  are needed for runtime HTTPS connections. The executable is stripped and PIE.
- Preview version ordering was checked with vercmp against preview.2 and 0.1.0.

The recipe declares aarch64, but that target has not been tested here.
AUR publication remains blocked by closed account registration. The GitHub
recipe provides a local makepkg installation path until publication is possible.

No host gateway, user configuration or runtime identity was changed. All package
installation and process checks used a disposable container and temporary config.
