# Web Configuration Editor Implementation Plan

**Goal:** Edit saved CA and SSH configuration files locally and through remote administration.
**Architecture:** Shared bounded atomic file helper, module-owned GET/PUT endpoints and one native textarea dialog.
**Tech Stack:** Go standard library, embedded JavaScript/CSS, sbx tests.
**Spec:** docs/superpowers/specs/2026-10-24-web-config-editor-design.md (approved).

## Constraints and review focus

No new dependency or filesystem browsing. Paths come exclusively from saved settings. Keep module boundaries, Origin/CSRF and authenticated relay context. File contents never enter polling/storage/logs. Preserve drafts, permissions and originals on failed validation/write. Reject stale file/target/path versions. Missing files are created only on Save. Limit UTF-8 content to 1 MiB. Check Windows replacement semantics explicitly.

## Tasks

- [x] 1. Write failing file-helper and module API tests. Implement `internal/configfile` with bounded Read/Write, opaque versions, symlink target checks, permissions and atomic replacement. Add module `/ca-file` and `/config-file` endpoints, strict CA validation and SSH pool retirement.
- [x] 2. Write failing frontend tests. Add `ui-file-editor.js` exported by `ui.js`, compact responsive dialog styles, both module buttons, draft/pending/close/keyboard behavior, target identification and save refresh.
- [x] 3. Run targeted and full checks, build/module matrix, fresh review and browser verification on 7070 with temporary files. Update configuration/design-system documentation.

## Commands and expected results

Use a fresh sandbox snapshot `/tmp/zelport-web-editor` in `zelport-tests`, with `GOFLAGS=-buildvcs=false`.
Targeted: `go test -race ./internal/configfile ./internal/modules/remote_http ./internal/modules/ports_over_ssh ./internal/web`; `node --test scripts/file-editor.test.mjs scripts/remote-http-settings.test.mjs scripts/ssh-configuration.test.mjs`.
Full: `make check`, `make build`, `make check-modules`. Expected: all pass.

## Execution ledger

Ruling: Continue natively in the existing feature checkout because the approved feature builds on its uncommitted changes. Do not create a checkout from HEAD, disturb `.superpowers`, or commit unrelated work. Plan approval is covered by the user's approved design and instruction to implement; no second approval loop.
Pre-flight: task 2 consumes the task 1 file response `{path,content,version}` and PUT `{revision,version,content}`. Modules refresh after save, not the shared helper. No signature conflict.


Verification complete: new backend and UI tests failed before implementation.
Full `make check` passes with 157 frontend tests, Go race tests, vet and all 42
packaging checks; native Linux build and macOS preview build pass. Remote typed
administration integration covers read/write and invalid Origin/CSRF rejection.
Independent review found CRLF normalization, canceled writes and failed initial
loads; all were reproduced, fixed and rechecked by the reviewer.
Browser on 7070 verified CA validation and save, missing SSH file creation only
on Save, Cmd+S, dirty-close guard, external-file conflict with draft retention,
explicit reload, 320px width and 800x400 short layout. Original default paths
restored afterward; real user SSH and CA files were not edited.
Windows composition containing both changed modules builds. The standard
Windows App build is blocked by the pre-existing missing Local Ports
`newListenerScanner`, recorded in TODO.md. This is not Windows runtime validation.

Final `make check-modules` passed all 12 compositions after the review fixes and asset registration change. Final `git diff --check` passed. Preview remains on port 7070 with default paths restored.
