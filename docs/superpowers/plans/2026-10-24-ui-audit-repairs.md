# UI audit repairs implementation plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all eleven findings approved in the 2026-10-24 browser audit, then repeat the UI verification on port 7070.
**Architecture:** Repair shared controls first; keep module-specific state and diagnostics owned by their modules. Reuse existing discovery results without adding health probes.
**Tech Stack:** Native JavaScript/CSS, Go embedded assets, Node tests in sbx, integrated browser via CUA.
**Spec:** The eleven findings and proposed remedies in the preceding audit, explicitly approved by the user.

## Constraints and rulings
- English UI copy. Preserve configuration, remote administration protections, secrets, drafts and polling stability.
- Continue the existing codex/ux-audit-fixes checkout with its uncommitted prior work; do not create a clean worktree that omits that work. No commits requested.
- User approved implementation of the concrete audit recommendations; no additional design approval round is needed.
- Leave preexisting .superpowers scratch untouched. This file is the implementation ledger.

## Tasks
- [x] 1. Shared interactions: conditional Escape handling, safe popover focus lifecycle, explicit field errors and focus recovery, meaningful domain validation, editor error recovery. Files: ui-dom.js, ui-forms.js, ui-services.js, ui-dialogs.js, ui-file-editor.js, module configuration forms. Regression tests: hidden-tooltip Escape propagation, nested popover focus, duplicate domain field error and recovery, error focus after asynchronous failure.
- [x] 2. Responsive presentation: full service identity and target, compact module heading retaining right-aligned view switch, 44px controls, consistent removal footer and mapping-only explanation. Files: app.css, module styles, ui-services.js. Verify 1280x800, 1024x600, 390x844, 320x568.
- [x] 3. Dashboard diagnostics: display module-provided cached local listener notices and route summary warnings, retain nodes across unchanged polls. Files: dashboard.js, local_ports module.js, app.js. Test known missing listener, discovery disabled/unavailable and metadata-only updates.
- [x] 4. Configuration consistency: no path field tooltip, explicit Remote HTTP CA scope, Relay Save/Discard dirty-state parity and restored baseline. Files: SSH/Remote/Relay configuration code. Test edit/revert, unchanged state, conflicts and successful saves.
- [x] 5. Full checks, fresh review, build preview, UI re-audit with isolated fixtures; restore original preview config and viewport.

## Review focus
- Errors inside short dialogs must become visible without losing entered data.
- Escape must reach its dialog when no tooltip is open, including initial close-button focus.
- Automatic focus on a popover trigger must not open a tooltip while another popover transitions.
- Mobile rows must retain full names/targets without horizontal overflow or undersized actions.
- Cached listener diagnostics must never claim current availability after discovery fails or is disabled.

## Verification ledger
Baseline: previous audit reproduced failures in the live 7070 preview; archive /tmp/zelport-audit-before.tgz preserves pre-change source for scoped review.

Implementation pass: shared tooltip lifecycle and field feedback, responsive tables/headings, Dashboard notices, and Relay dirty-state changes are in place. Three new shared regressions were observed RED then GREEN. The full JS suite passed 179/179 before adding the final cache/dirty-state regression scenarios. Old assertions for intentionally removed behavior were updated (hidden diagnostics, path tooltip, stacked removal actions, always-active save).
Ruling: cached local notices are explicitly described as last detected observations; Dashboard does not trigger discovery. They are invalidated by changed settings/routes or a failed discovery.
Ruling: layout changes are verified in the integrated browser, rather than adding tests that merely repeat CSS declarations.


## Final verification — 2026-10-24

All eleven approved findings are implemented. A fresh independent review found
two additional edge cases in those repairs (reverted edits during a pending
Built-in Relay save and the wording of cached non-loopback observations). Both
were reproduced with failing tests and fixed. Browser verification additionally
found and fixed proxy errors incorrectly associated with the CA field and an
early-navigation initialization race. Both received failing-then-passing tests.

| Approved finding | Verification evidence |
| --- | --- |
| Hidden errors and lost focus | Remote duplicate-domain failure at 1024×600 places the message beside Domain name, focuses it, and preserves the target URL. Local invalid port and SSH relative path focus the associated field. Invalid proxy scheme focuses Proxy URL. |
| Tooltip Escape and popover lifecycle | Native keyboard Escape first dismisses the visible Close tooltip, then closes the removal dialog. Header Open settings reaches Relay Configuration without InvalidStateError. |
| Mobile service identity and target | Local at 390×844 and Remote HTTP at 320×568 show full long names and target addresses without document overflow. SSH target is also visible. |
| Path-field tooltip overlap | SSH Configuration at 320×568 has zero open tooltips after its path receives focus. Remote HTTP path also has no tooltip. |
| Dashboard diagnostics | Local missing-listener observation remains visible after navigating to Dashboard. Existing HTTP security warnings appear beneath targets. Tests verify invalidation on lookup failure and disabled discovery. |
| CA trust copy | Remote HTTP explicitly says certificates are for Remote HTTP connections. No OS trust claim remains. |
| Domain validation | A domain containing a space produces an inline format explanation, aria-invalid and focused Domain name instead of a generic browser validation bubble. |
| Relay dirty state | Both Relay forms start with disabled Save/Discard, enable after an edit, disable after reverting, and restore baseline on Discard. Built-in Relay successfully saves a test port while disabled, then returns to clean state. Concurrent edit/revert during save is covered by regression test. |
| Mobile heading | Plus/refresh sits beside the title; Overview/Configuration stays right-aligned below it. Measured right edge: 300 at width 320 and 370 at width 390. Desktop modules all use right edge 1180 at width 1280. |
| Touch targets | Visible Remote HTTP actions and Local menu triggers measure 44×44 at narrow widths. SSH editor Close measures 44×44 even with a wrapped title. |
| Removal confirmation | Shared Cancel / Remove footer and mapping-only explanation verified in dialog. URL uses left alignment in the final CSS. |

Additional checks: the web editor preserves a draft after an externally induced
file-version conflict and focuses the visible error at 320×568. Successful Local
Ports service creation was followed through its public URL to the controlled
backend (“Audit backend works”). Empty Local, SSH and Relay overviews use full
1080px content width at a 1280px viewport and centered action buttons.

Final automated verification, inside `zelport-tests` sandbox:
- `GOFLAGS=-buildvcs=false make check`: 185/185 JavaScript tests, Go race tests,
  Go vet, 39 release tests and 3 Arch preparation tests passed.
- `GOFLAGS=-buildvcs=false GOOS=darwin GOARCH=arm64 make build`: passed.
- Host `git diff --check`: passed.

The final browser run after the 09:45:22 UTC restart has no new warning/error
console entries, including immediate reload/hash navigation and header popover
Open settings. Earlier console records are retained by the browser and were
filtered by timestamp rather than mistaken for fresh failures.

Scope limits: live visual verification used the current light theme. The
integrated browser exposes viewport control but no color-scheme emulation, so
dark-theme variables were inspected in source rather than claiming a dark-mode
screenshot test. No live remote Relay peer or successful SSH server connection
was provisioned. Their UI states and existing backend tests remain separate
from an end-to-end network test. Mouse-hover timing was covered by automated
tooltip tests; live tooltip checks used focus, click, Escape and viewport changes.

Preview restored to the original `/tmp/zelport-host-7070.2eBVBX/config.json` on
port 7070 with `/private/tmp/zelport-ui-audit-verified`. Only the isolated audit
configuration and audit SSH file were changed by scenario testing. The audit
backend was stopped; the user’s separate 7071 tab was untouched. No commit or
push was made.
