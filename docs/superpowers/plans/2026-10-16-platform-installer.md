# Platform Installer Implementation Plan

**Goal:** Implement the approved platform-aware installer in the gateway repository.

**Architecture:** A standalone Bash entry point delegates Arch installation to
makepkg and installs verified release binaries on other Linux distributions.
Service registration is independent of binary installation. Platform detection,
verification and service operations are separately testable shell functions.

**Spec:** The approved conversation design, documented in
`contrib/install/README.md`. macOS prints Homebrew instructions; Windows prints
an upcoming-support notice. No desktop plugin installation. Non-systemd hosts
receive manual startup instructions pending explicit service adapters.

**Constraints:** No root execution, no gateway configuration writes, no unrelated
process termination, no overwriting custom service units, no duplicate install
when another Zelport command is on PATH. Pin releases and verify downloads.
Use synthetic fixtures and isolated homes in tests.

## Implementation steps

- [x] Write isolated platform, checksum, preservation and service tests; observe
  failures before implementing the entry point.
- [x] Implement platform dispatch, verified binary and Arch installation,
  user-service setup and manual-start fallback.
- [x] Add license-notice installation and failure-path tests.
- [x] Add installer tests to packaging checks and document usage and limitations.
- [x] Run packaging and workspace checks, a real-download smoke check in an
  isolated home, and a final diff/privacy review.

## Review focus

- Checksum mismatch must leave the installed binary unchanged.
- A missing user session must never trigger system-level service registration.
- Existing custom units, symlinks and gateway configuration must be preserved.
- Pipe invocation must not consume package-manager prompts as script input.
- Release pins and the Arch recipe checksum must match published artifacts.

## Verification results

- 20 isolated installer tests passed, including direct and pipe invocation.
- Packaging checks passed (release packaging, Arch recipe and installer).
- Workspace `check`, `check-modules` and `build` passed.
- The published Linux amd64 archive was downloaded, checksum-verified and
  installed with its notices in a disposable home without starting a service.
- The pinned public Arch recipe checksum was verified.
- Direct macOS execution printed Homebrew guidance without installation.
- Final diff and personal-information scan passed. Native Linux package-manager
  execution and actual systemd startup were not exercised by this change.
