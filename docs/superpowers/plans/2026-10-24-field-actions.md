# Field action adornments

User-approved scope: move E2E Change/Generate into InputAdornment and apply the same placement to analogous existing field actions. Execute inline; existing dirty checkout retained, no commits requested. Baseline /tmp/zelport-before-field-actions.tgz.

- E2E field: Reveal, Copy, Change (edit icon) when stored; Reveal, Copy, Generate (shuffle icon) while editing. Preserve draft/save/discard and secret lifecycle.
- SSH configuration path: browser editor, terminal editor, refresh hosts inside the path field.
- Remote HTTP CA path: browser editor, terminal editor, reload certificates inside the path field.
- Built-in Relay already uses adornments; use the same edit icon for Change access token. Keep its explicit disconnection confirmation.
- Local Ports and service dialogs: existing picker refresh is scoped to its results, not a single path/credential. No detached per-field actions to migrate. Form Save/Discard and dialog confirmations remain text buttons.

Files: module-owned JS/CSS above; shared icon registry; existing action-locator tests; UI_DESIGN_SYSTEM.md. No API or credential policy change.

Verification: existing frontend behavior suites and full repository checks/build; browser exercise each field action, disabled/draft states, keyboard tooltip/focus, 1280/720/390/320 widths and short viewport; use real styles in a theme preview where native browser theme control is unavailable. Fresh scoped review before delivery. No new implementation-mirroring tests for this reversible presentation change.

## Evidence
- Focused suites:84 passed. Full make check:214 frontend tests passed, Go race suite and vet passed,39+3 packaging tests passed. One existing focused mock lacked adornmentButton after the move; updated that mock interface and reran all checks successfully.
- Fresh scoped review: no critical/important/minor findings. Unrelated pre-existing changes and backend semantics excluded; actual rendering and test results verified separately here.
- Real macOS browser instance7274 used isolated SSH config and synthetic CA file. E2E Generate/Save/Change/Reveal/Discard work via adornments; generated value remains40 characters. SSH and CA web editors opened their correct saved fixture paths and were cancelled without writes. Refresh hosts returned1; certificate reload succeeded. Both editor actions disabled for unsaved path drafts; Discard restored saved state. Terminal-launch behavior unchanged and covered by existing tests; no terminal editor was launched in this UI run.
- Built-in Relay pencil adornment inspected; absent test-instance token correctly left all credential actions disabled. Existing rotation/confirmation tests passed.
- Keyboard Tab focused CA browser-edit action and displayed shared tooltip; Escape dismissed it. SSH320px and CA720/390/320px layouts had no horizontal overflow, all three field actions measured44x44. Short480px viewport remained scrollable. Gallery using real shared components/styles verified light/dark at desktop and390px.
- Screenshots in the local task artifacts directory (not published): field-actions-relay.png, field-actions-ssh.png, field-actions-remote.png, field-actions-light.png, field-actions-dark.png.
- Darwin arm64 binary updated in bin/zelport (build2026-10-24 18:49:51 UTC). Preview7070 restarted with unchanged config path; no live settings mutations. Own test7274 and gallery18884 stopped; temporary viewport reset and browser tabs closed. Original user tab preserved.
