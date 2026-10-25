# Consistent Module Views Implementation Plan

**Goal:** Give every module the same right-aligned Overview/Configuration switch and separate icon actions.
**Architecture:** Shared moduleViews helper owns navigation, visibility and the action slot. Modules own contents and settings lifecycle.
**Tech Stack:** Native JavaScript/CSS and existing Go embedded assets.
**Spec:** docs/superpowers/specs/2026-10-23-module-views-design.md

## Constraints and review focus

Preserve existing uncommitted work, drafts and node identity. Do not modify real SSH configuration or relay credentials. Check local/remote module integration, settings placement, configuration navigation, late secret results, disposal, and constant right alignment at desktop and narrow widths. All JavaScript tooling runs through sbx.

## Tasks

- [x] Add failing shared navigation/visibility/action/disposal tests and replace the existing toolbar text-action expectation with accessible icon behavior.
- [x] Implement shared moduleViews in ui-module-views.js, export through ui.js, migrate all five module assets, remove duplicate module navigation CSS, and add shared responsive CSS.
- [x] Adapt existing module tests without weakening draft, credential or lifecycle checks. Verify all frontend tests, Go race/vet/build and module matrix.
- [x] Review integration, refresh preview on 7070, check all modules in browser, right alignment and narrow viewport. Record final evidence.

## Verification

- Red first: shared moduleViews API and plus-icon regression tests failed before implementation.
- Final checks: 136 frontend tests, full Go race suite, vet, 42 packaging tests, Linux build and macOS arm64 preview build passed.
- Independent focused review found no actionable issues.
- Browser: all four source switches measure 260 px wide with right edge at 1055 px in a 1095 px viewport, in Overview and Configuration. Local Ports plus opens the service dialog. Local Ports and Remote HTTP Configuration are empty. Built-in Relay switches between runtime and its existing form.
- 320 px and 390 px viewports retain right alignment and no page overflow. Temporary drafts are discarded, settings and credentials remain unchanged.
- Preview refreshed on localhost:7070.

- Final module matrix passed all 12 compositions, including core, individual modules, pairs and explicit-all.
