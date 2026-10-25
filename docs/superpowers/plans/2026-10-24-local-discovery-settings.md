# Local Ports Discovery Settings Plan

**Goal:** Add the three accepted discovery preferences to Local Ports Configuration.
**Architecture:** Module settings/API filters scanner results while returning separate configured-port metadata; a module-owned configuration form updates shared picker state.
**Tech stack:** Go standard library, embedded JS/CSS, sbx.
**Spec:** docs/superpowers/specs/2026-10-24-local-discovery-settings-design.md.

- [x] Backend RED/GREEN: default/validated/persisted settings, filtering, disabled scanner, configured-route inventory and protected mutations.
- [x] Frontend RED/GREEN: form drafts/save/conflict, discovery-off manual entry, live picker and inventory invalidation.
- [x] Full checks/build/module matrix, independent review and browser verification on 7070; restore test preferences afterward.

Ruling: The user explicitly accepted the three proposed controls and instructed implementation. Match semantics and bounds are routine choices documented above; proceed in the existing feature checkout without another approval loop. Preserve all earlier changes and leave `.superpowers` untouched.


Verification: backend and frontend RED tests observed, then passed. `make check`
passes (170 frontend tests, Go race tests, vet and 42 packaging checks). Linux
and macOS preview builds pass. `make check-modules` passes all 12 compositions.
Independent review found route-inventory and comma-containing process-name
cases; both reproduced, fixed and independently rechecked. Tests cover local and
remote Origin/CSRF, persistence/revision conflicts, disabled scanner, manual
forwarding, unknown/mixed owners and preserved configured-listener warnings.
Browser on 7070 verified invalid range rejection, a real controlled port hidden
by inclusive range and mixed-case process name, discovery off with editable
manual port, and restored default preferences. Widths 320px and 800px (400px
height) show no horizontal overflow. Viewport reset and fixture server stopped.
Only temporary preview preferences were used, no service was created or changed.
