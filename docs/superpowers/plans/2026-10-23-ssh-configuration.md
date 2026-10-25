# SSH Configuration Implementation Plan

**Goal:** Add module Configuration and a terminal editing action.
**Spec:** docs/superpowers/specs/2026-10-23-ssh-configuration-design.md
**Architecture:** Module-owned settings/UI, hardened sshpipe wrappers, OS-specific terminal launcher.
**Tech Stack:** Go, native JavaScript/CSS, sbx test runtime.

## Global constraints

English text; no dependencies; preserve Origin/CSRF, SSH restrictions, polling
drafts and existing uncommitted UX changes. Work on the existing feature branch.
Never edit the user's actual SSH config during verification.

## Review focus

- Config file paths with spaces, quotes, Include directives and control characters.
- Generated wrapper cleanup must not remove the selected user config.
- Remote administration cannot launch an OS process.
- Stale settings/editor requests cannot apply to an unexpected file.
- Editor launch return/error handling and keyboard/polling drafts.

## Task 1: SSH config selection (independent agent)

Files: internal/sshpipe only.
Interfaces: ResolveConfigPath(string) (string,error), NewWithConfig(host string,port int,configFile string) *Pool.
- [x] Add failing tests for default/custom path resolution and wrapper include/cleanup.
- [x] Implement selection while preserving hardening and existing New callers.
- [x] Verify isolated sshpipe tests in sbx.

## Task 2: Terminal editor launcher (independent agent)

Files: new internal/terminal package only.
Interfaces: Availability() error, OpenEditor(path string) error.
- [x] Add failing command construction/escaping and launcher lifecycle tests.
- [x] Implement platform-specific terminal launch and temporary script cleanup.
- [x] Verify tests in sbx and cross-compile supported OS variants.

## Task 3: SSH settings frontend (independent agent)

Files: ports_over_ssh assets and dedicated frontend tests only.
Consumes: state fields and /settings, /edit-config endpoints described in the spec.
Produces: openSettings() and existing serviceModule lifecycle/edit contracts.
- [x] Add failing navigation/settings/draft/edit tests.
- [x] Implement Overview/Configuration, local editing action, custom source help.
- [x] Verify targeted frontend suite in sbx; root owns shared CSS changes if needed.

## Task 4: Module backend and integration (root)

Files: ports_over_ssh Go settings/module/API tests and documentation.
- [x] Add tests for settings persistence, validation, custom discovery, local-only launch and stale revisions.
- [x] Implement settings commits, path-aware pools/discovery and editor endpoint.
- [x] Integrate helpers; verify pending errors and full-width responsive UI.
- [x] Run all tests, race, vet, build, module matrix and native preview.
- [x] Obtain fresh review, resolve issues, refresh 7070 and report evidence.

## Verification record

- Full sandbox checks passed: 129 frontend tests, Go race tests, vet, 42 packaging tests, application build and all 12 module combinations.
- macOS arm64 preview built and served on localhost:7070 using the existing temporary application configuration.
- Browser verified saving a temporary custom path, reading its synthetic alias, disabling editing for unsaved drafts, restoring the original path, and 320/390 px layouts without page overflow. Desktop configuration uses the full module width.
- Edit in terminal was accepted by the native macOS launcher with a temporary fixture. The computer-use tool denies access to Terminal, so interactive editor behavior could not be inspected. The user's real SSH configuration was never edited.
- Fresh independent review identified Include token escaping, quoted Windows editor paths and inaccurate IPv6 help. All findings were resolved and independently re-reviewed. The final clean snapshot passed make check and make build again.
- Windows/Linux terminal behavior has automated launcher-boundary checks. The terminal package cross-compiles for Windows and macOS, without desktop runtime validation on Windows/Linux. A full Windows App cross-build is blocked by the existing Local Ports scanner lacking a Windows implementation (`newListenerScanner`).

- Final macOS arm64 application and Windows amd64 SSH-only application cross-builds passed.
