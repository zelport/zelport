# Relay configuration repair implementation plan

> **For agentic workers:** Use superpowers:executing-plans. User approved the compact single-form design after the two-instance audit and explicitly requested implementation without verbose UI copy.

**Goal:** Make relay access and E2E setup discoverable, editable, and understandable without explanatory walls of text.
**Architecture:** Keep existing module APIs and cryptographic protocol. Use one configuration form, explicit local E2E drafts, and shared connection feedback in Overview/Configuration. Require an explicit E2E choice on first settings save.
**Tech Stack:** Embedded vanilla JS/CSS, Go, existing UI components.
**Spec:** Approved audit recommendation in conversation and relay-two-instance-audit.html (24 October).

## Constraints and rulings
- Preserve existing saved credentials, route IDs, provider pins, drafts, focus and module boundaries.
- No automatic fallback from token authentication to open access.
- Zero peers does not prove an incorrect E2E secret; show count plus a short actionable hint.
- Generation creates a local draft; Save is the sole persistence point for the new UI.
- Keep stored E2E read-only in its single field until Change; editable draft occupies the same field. Reveal/copy use the current draft or explicitly fetch the stored secret.
- Ruling: continue in current codex/ux-audit-fixes checkout, preserving ongoing uncommitted work. Keep pre-change archive /tmp/zelport-before-relay-form.tgz for scoped review. No commits requested.
- Ruling: user already approved the concrete audit proposal; no further approval round. Keep execution ledger here instead of touching other tasks' .superpowers scratch.

## Review focus
- Revealed stored values must never become replacement drafts or be submitted accidentally.
- Late secret responses must not repopulate a hidden/disposed form or overwrite user input.
- Switching access modes must preserve stored tokens until Save and restore on Discard.
- External credential changes must not silently let an old replacement draft bypass revision conflict.
- A successful settings save does not establish connectivity; show runtime status separately.

## Tasks
- [x] Add failing Go tests for first-save E2E requirement and unchanged stored state on rejection; update first-save fixtures to choose a secret.
- [x] Add failing UI tests for a single visible E2E control, local generation + discard, change/reveal/copy semantics, visible access mode, late responses, honest error feedback and separate device count.
- [x] Implement compact configuration in module-owned configuration.js, explicit first-save requirement, connection feedback, import title and provider/availability labels.
- [x] Run targeted tests RED→GREEN, complete suite, Go race/vet/build. Review scoped diff with fresh reviewer; fix material findings.
- [x] Verify two isolated instances through matching/missing/wrong token, differing E2E groups, replacement, import, sharing, reconnect; inspect actual UI and responsive layout. Update preview and requested bin/zelport build without rewriting live configuration.

## Evidence / ledger
- Baseline: 31 runtime audit scenarios + browser import and actual HTTP success; 30 relay UI tests; integration suite passed (long-renewal skipped).

- Implementation complete: one form, single editable E2E location, explicit admission mode, local Generate draft, Save/Discard, concise runtime diagnostics and device count, corrected import action/provider/availability labels.
- RED→GREEN: first-save Go validation, initial UI behavior cases, empty Change cancellation, deferred clipboard rejection (Change/Discard/navigation/revision). Generated replacement protection also verified.
- Final review: fresh gpt-6-astra reviewer compared against the pre-change archive. One important clipboard continuation race found and fixed; no other critical/important/minor findings. Reviewer deliberately excluded unrelated pre-existing changes, as scoped.
- Final: fixed stale clipboard failure restoring stored credentials — generation + revision guard on both completion paths; regression tests RED→GREEN; full frontend suite 201/201.
- Checks: go test -race ./..., go vet ./..., Darwin ARM64 make build all passed. Packaging checks 39 + 3 passed. git diff --check passed.
- Runtime: 29 recorded cases on isolated native instances 7170/7171 with loopback relay8170 and backend7172, plus explicit empty-first-secret rejection on both instances. Covered open/protected relay, missing/wrong/correct/rotated access tokens, matching/different/replaced E2E, import via browser, real HTTP200, sharing revocation503 and recovery200, reconnect/restart, invalid values rejected400, URL variants. Final build preserves settings/imports across restart.
- Browser: single stored field → Change → same editable field → Discard verified; actionable authentication failure; Connected with zero devices; import title/action/provider label; real service and remote E2E administration. Configuration inspected at 1280px, 390×844 (no page overflow, controls >=44px), and 1280×600 (all actions reachable). Viewport override reset. Final7070 browser console had no warnings/errors.
- Artifacts: relay-repair-configuration.png, relay-repair-mobile.png, relay-repair-7070.png and relay-repair-results.json in this thread visualization directory.
- Installed verified Mach-O arm64 build at bin/zelport and preview7070. Preview configuration checksum unchanged. Original user browser tab left intact to protect possible drafts; new UI loads after refreshing.
- Limits: two processes on one macOS host, not two physical operating systems; long session-renewal timing was not exercised. Existing unrelated work was preserved and not re-audited.
