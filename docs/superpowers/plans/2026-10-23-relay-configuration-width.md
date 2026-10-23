# Relay Configuration Width Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Ports over Relay Configuration the same full-width page frame as Overview while retaining a readable 680px inner form column.

**Architecture:** Keep the existing DOM and move the width constraint from the two panel containers to their inner content through module-scoped CSS. No JavaScript, state, or shared shell behavior changes.

**Tech Stack:** Embedded CSS, Node.js built-in test runner, Docker Sandbox through `sbx`

**Spec:** `docs/superpowers/specs/2026-10-23-relay-configuration-width-design.md`

## Global Constraints

- Both Configuration panels fill the available module width.
- Inner setup blocks and the configuration form retain a 680px maximum width.
- Existing narrow-screen padding and natural shrinking remain unchanged.
- Add no dependency, frontend build step, JavaScript behavior, or persisted configuration change.
- Run JavaScript tooling only in Docker Sandbox through `sbx`.

## Review Focus

- A wide viewport must show full-width panel borders with left-aligned readable content.
- A narrow viewport must not gain horizontal overflow.
- The Relay connection badge must remain aligned within the readable content column.
- Overview width and navigation must remain unchanged.
- Light and dark themes must retain existing surfaces and borders.

---

### Task 1: Align Configuration panel width

**Files:**
- Modify: `scripts/navigation.test.mjs`
- Modify: `internal/modules/ports_over_relay/assets/style.css`

**Interfaces:**
- Consumes: existing `.relay-settings-panel`, `.relay-configuration-view .relay-setup`, and `.relay-configuration-form` elements.
- Produces: full-width Configuration panel containers with 680px inner content caps.

- [ ] **Step 1: Write the failing CSS contract test**

Add `relay configuration panels share the overview frame while forms keep a readable width`. Assert that `.relay-settings-panel` and `.relay-configuration-view .relay-setup` do not declare `max-width`, while `.relay-configuration-form` and direct children of `.relay-configuration-view .relay-setup` declare `max-width:680px`.

- [ ] **Step 2: Run the focused test and verify it fails**

Run in Docker Sandbox: `node --test scripts/navigation.test.mjs`

Expected: FAIL because both panel containers currently declare `max-width:680px`.

- [ ] **Step 3: Move the width constraint to inner content**

Remove `max-width` from `.relay-settings-panel` and `.relay-configuration-view .relay-setup`. Add a combined module-scoped rule that caps `.relay-configuration-form` and every direct child of `.relay-configuration-view .relay-setup` at 680px.

- [ ] **Step 4: Run frontend and repository verification**

Run in Docker Sandbox: `node --test scripts/*.test.mjs`, then `make verify` and `make build`.

Expected: all commands pass.

- [ ] **Step 5: Review in the browser and refresh the host test build**

Inspect Overview and Configuration on `localhost:7070` at wide and narrow widths. Confirm full-width panel frames, readable inner content, unchanged navigation, and no horizontal overflow. Rebuild the native macOS arm64 binary and restart the loopback-only instance on port 7070.

- [ ] **Step 6: Commit**

```bash
git add internal/modules/ports_over_relay/assets/style.css scripts/navigation.test.mjs docs/superpowers/specs/2026-10-23-relay-configuration-width-design.md docs/superpowers/plans/2026-10-23-relay-configuration-width.md
git commit -m "fix(ui): align relay configuration panels"
```
