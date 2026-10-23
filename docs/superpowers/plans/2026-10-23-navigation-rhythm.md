# Navigation Rhythm Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Zelport a calmer navigation hierarchy with left-aligned global navigation and a responsive segmented Overview/Configuration control inside the Ports over Relay heading.

**Architecture:** Keep the shell-owned module heading and toolbar as the integration boundary. Ports over Relay will mount its local view navigation into the supplied toolbar beside its existing Refresh action, while its content and lifecycle stay module-owned. Shared heading layout remains in the shell stylesheet and the segmented-control appearance remains in the module stylesheet.

**Tech Stack:** Embedded HTML and CSS, native JavaScript modules, Node.js built-in test runner, Docker Sandbox through `sbx`

**Spec:** `docs/superpowers/specs/2026-10-23-navigation-rhythm-design.md`

## Global Constraints

- Keep the global labels exactly `Dashboard`, `Services`, and `Settings`.
- Keep the Ports over Relay local labels exactly `Overview` and `Configuration`.
- Keep all three global destinations visible and left aligned at every supported width. Do not add a hamburger menu.
- At narrow widths, stack the module heading and actions, give the two local choices equal width, and provide at least 44 px touch targets without horizontal scrolling.
- Preserve module hashes, browser history, polling, draft preservation, secret clearing, refresh behavior, and remote administration.
- Add no dependency, CDN asset, frontend build step, compatibility layer, or persisted configuration change.
- Run JavaScript and browser tooling only in Docker Sandbox through `sbx`.
- Do not commit `.superpowers/`, binaries, runtime configuration, or generated preview artifacts.

## Review Focus

- A 320 px viewport must keep Dashboard, Services, and Settings visible without distributing them across the full row or causing horizontal scrolling. Task 2 pins the explicit alignment and narrow gaps in a CSS contract test and visual review.
- A long module title plus the Experimental badge must wrap above the local control without overlapping it. Task 2 pins the stacked heading and wrapping rules and checks the 320 px gallery state.
- Refresh must be visible in Overview, hidden in Configuration, and must not make the segmented control overflow. Task 1 pins visibility behavior and Task 2 pins the narrow action-grid layout.
- Switching views must preserve configuration drafts while clearing only secrets that the existing lifecycle already clears. Task 1 updates and retains the existing draft and secret regression tests.
- Removing or disabling the module must not leave local navigation or Refresh controls in the shared heading. Task 1 adds disposal cleanup coverage.

## File Map

- `internal/modules/ports_over_relay/assets/module.js` owns local view labels, state, toolbar mounting, Refresh visibility, and disposal.
- `internal/modules/ports_over_relay/assets/style.css` owns the segmented control's visual treatment and module-specific sizing.
- `internal/web/assets/app.css` owns left-aligned global navigation and the responsive shell heading/action layout.
- `scripts/render-stability.test.mjs` exercises Ports over Relay DOM placement, view state, Refresh visibility, draft retention, secret lifecycle, and cleanup.
- `scripts/navigation.test.mjs` records the global navigation and responsive CSS contract.
- `scripts/ui/design-system.html` provides a real-CSS visual fixture for wide and narrow module headings.
- `docs/UI_DESIGN_SYSTEM.md` records the implemented navigation hierarchy and review widths.

---

### Task 1: Move and rename the Ports over Relay view navigation

**Files:**
- Modify: `scripts/render-stability.test.mjs:27-153`
- Modify: `internal/modules/ports_over_relay/assets/module.js:10-116`

**Interfaces:**
- Consumes: the existing module context `{panel, toolbar, ui, ...}` supplied by `internal/web/assets/app.js` and `refreshButton(label, handler)` from `ctx.ui`.
- Produces: a `nav.relay-view-nav` mounted in `ctx.toolbar`, buttons labelled `Overview` and `Configuration`, unchanged `aria-current="page"` semantics, and complete removal of both local navigation and Refresh during `dispose()`.

- [ ] **Step 1: Give the relay test harness a toolbar and write the failing placement and naming test**

Update `context()` to return a sibling `toolbar` node. Add a test named `relay local navigation uses unique labels in the shared module toolbar` that asserts:

- `relay-view-nav` is a descendant of `ctx.toolbar` and not `ctx.panel`
- the two button labels are exactly `Overview` and `Configuration`
- the navigation label is exactly `Ports over Relay views`
- Overview initially has `aria-current="page"`
- Configuration initially has `aria-current="false"`
- no local button is labelled `Services`

- [ ] **Step 2: Extend the failing test to cover Refresh and disposal**

Assert that the Refresh catalog control is visible in Overview, becomes hidden after clicking Configuration, becomes visible after clicking Overview, and that `view.dispose()` removes both the navigation and Refresh nodes from `ctx.toolbar`.

- [ ] **Step 3: Update the existing relay lifecycle tests to select Overview**

Replace local button lookups for `Services` with `Overview` in the revealed-secret and unsaved-draft tests. Keep their existing assertions unchanged so the rename cannot weaken secret clearing or draft preservation.

- [ ] **Step 4: Run the focused test and verify it fails for the intended reasons**

Run in Docker Sandbox: `node --test scripts/render-stability.test.mjs`

Expected: FAIL because the current navigation remains inside the panel, still says Services, and is not removed during module disposal.

- [ ] **Step 5: Implement the toolbar-owned Overview and Configuration navigation**

In `mount(ctx)`, create `overviewView`, `configurationView`, `overviewButton`, and `configurationButton`. Append the navigation to `ctx.toolbar` before the Refresh control, append only the two content sections to `ctx.panel`, and use `showView('overview')` as the initial state. Keep the existing configuration form, polling, draft, and secret branches intact. Extend `dispose()` to remove `navigation` and `refresh` before disposing the shared service view.

- [ ] **Step 6: Run the focused test and verify it passes**

Run in Docker Sandbox: `node --test scripts/render-stability.test.mjs`

Expected: PASS with the new placement, labels, state transitions, cleanup, and all existing lifecycle regressions intact.

- [ ] **Step 7: Commit the functional navigation change**

```bash
git add internal/modules/ports_over_relay/assets/module.js scripts/render-stability.test.mjs
git commit -m "refactor(ui): clarify relay view navigation"
```

### Task 2: Apply the responsive navigation hierarchy

**Files:**
- Modify: `scripts/navigation.test.mjs:1-67`
- Modify: `internal/web/assets/app.css:193-215`
- Modify: `internal/modules/ports_over_relay/assets/style.css:1-35`
- Modify: `scripts/ui/design-system.html:1-20`
- Modify: `docs/UI_DESIGN_SYSTEM.md:138-160`

**Interfaces:**
- Consumes: `nav.relay-view-nav` and `.relay-view-button` from Task 1, plus the existing `.module-heading`, `.module-actions`, `.module-toolbar`, and `.primary-navigation` shell structure.
- Produces: explicit left alignment for global navigation, an inline segmented control on wide layouts, a stacked full-width heading/action layout at 720 px and below, and a reusable gallery fixture for visual review.

- [ ] **Step 1: Write the failing global and local navigation CSS contract test**

In `scripts/navigation.test.mjs`, read both stylesheets and add a test named `global and relay navigation keep distinct responsive visual grammars`. Assert these exact contracts:

- `.primary-navigation` includes `justify-content:flex-start` and a 24 px wide-layout gap
- the 480 px rule reduces the global gap to 16 px and the 360 px rule reduces it to 12 px
- `.relay-view-nav` uses a two-column grid and does not use a bottom border as its selected-state grammar
- `.relay-view-button[aria-current=page]` uses a filled surface and shadow, not `border-bottom-color`
- the 720 px rules stack `.module-heading`, expand `.module-actions` and `.module-toolbar`, let `.relay-view-nav` grow without a fixed minimum width, and set local buttons to at least 44 px high

- [ ] **Step 2: Run the navigation test and verify it fails**

Run in Docker Sandbox: `node --test scripts/navigation.test.mjs`

Expected: FAIL because the shell does not state left alignment, the relay control still repeats the underline pattern, and the module heading has no narrow stacked layout.

- [ ] **Step 3: Implement the shared shell spacing and responsive heading layout**

In `app.css`, make the global navigation explicitly left aligned with gaps of 24 px, 16 px at 480 px and below, and 12 px at 360 px and below. Give `.module-heading` 32 px top and 24 px bottom separation. At 720 px and below, stack `.module-heading`, stretch `.module-actions` and the active `.module-toolbar`, and use a `minmax(0,1fr) auto` action layout so the local control consumes remaining width beside Refresh. Preserve wrapping on `.module-title` for the maturity badge.

- [ ] **Step 4: Implement the module-scoped segmented control**

In the relay stylesheet, replace the underline row with a two-column segmented control using existing `--surface`, `--bg`, `--line`, `--muted`, `--ink`, `--accent-soft`, radius, and shadow tokens. Give the control a 260 px wide-layout minimum, 4 px inner padding, a 1 px line border, and `--radius-panel`. Use equal columns, a minimum 40 px button height on wide layouts, visible focus, and a filled selected surface. At 720 px and below, remove the fixed minimum width, allow the control to grow, and raise both buttons to a 44 px minimum height.

- [ ] **Step 5: Add the real-CSS gallery fixture and update the design-system documentation**

Add a gallery section that composes `.primary-navigation`, `.module-heading`, `.module-title`, `.module-actions`, `.module-toolbar`, `.relay-view-nav`, the Experimental badge, and a Refresh icon control. Document that global navigation is left aligned and underlined while Ports over Relay uses the segmented Overview/Configuration control in its heading and stacks it below the title at narrow widths.

- [ ] **Step 6: Run focused automated verification**

Run in Docker Sandbox: `node --test scripts/navigation.test.mjs scripts/render-stability.test.mjs scripts/ui-primitives.test.mjs scripts/shell-ui.test.mjs`

Expected: PASS.

- [ ] **Step 7: Perform visual verification with real application CSS**

Run `python3 scripts/ui/preview.py` in Docker Sandbox and inspect the gallery at 1280 px, 720 px, 390 px, and 320 px in light and dark themes. Verify left alignment, no hamburger, equal local segments, title and badge wrapping, visible keyboard focus, at least 44 px local controls on narrow widths, a usable trailing Refresh control, and no horizontal scrolling.

- [ ] **Step 8: Commit the responsive visual system and documentation**

```bash
git add internal/web/assets/app.css internal/modules/ports_over_relay/assets/style.css scripts/navigation.test.mjs scripts/ui/design-system.html docs/UI_DESIGN_SYSTEM.md docs/superpowers/specs/2026-10-23-navigation-rhythm-design.md docs/superpowers/plans/2026-10-23-navigation-rhythm.md
git commit -m "feat(ui): establish responsive navigation hierarchy"
```

### Task 3: Verify the complete App change

**Files:**
- Verify only: all files committed by Tasks 1 and 2

**Interfaces:**
- Consumes: the functional and visual navigation changes from Tasks 1 and 2.
- Produces: repository-level evidence that the embedded UI, Go application, module matrix, and build remain valid.

- [ ] **Step 1: Run complete repository verification in Docker Sandbox**

Run: `make verify`

Expected: all JavaScript, Go, packaging, vet, and module-build checks pass.

- [ ] **Step 2: Build the application in Docker Sandbox**

Run: `make build`

Expected: `bin/zelport` is produced successfully and remains untracked.

- [ ] **Step 3: Inspect the final repository state**

Run: `git status --short`, `git diff --check`, and `git log -2 --oneline`.

Expected: only the intentionally untracked `.superpowers/` preview workspace and ignored build output may remain; there is no whitespace error; the two focused Conventional Commits are present.
