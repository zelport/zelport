# Snitzel Modularization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every service feature independently selectable at compile time while keeping an untagged core-only binary and a complete release build.

**Architecture:** Standard-library module contracts and a registry separate common app/web infrastructure from feature implementations. Tagged composition-root files import selected factories, including their embedded frontend assets. Generic route dispatch is the integration point for future cloud sharing.

**Tech Stack:** Go build tags, standard-library HTTP/embed/JSON, browser JavaScript modules, existing CSS.

**Spec:** [Module architecture and agreed build behavior](../../MODULES.md) and
[normative module contracts](../../MODULE_CONTRACTS.md).

## Implementation record (2026-10-11)

The five-module boundary and build selection are implemented. See
[verification evidence](../../modularization-verification.md) for executed checks,
review fixes and remaining live/manual verification limits. The checklist below
is the original implementation specification; test cases were consolidated into
behavior-focused suites, so proposed filenames/test names are not an inventory
of the delivered files.

## Global Constraints

- No tag means core only. No core/minimal/custom/all tag. Use the exact feature tags in the spec.
- `make build-full` enumerates all implemented modules; distributed Snitzel is full.
- Every source feature is optional, including Local Ports. Cloud is implemented after this plan.
- Do not introduce legacy configuration migration. Use the new generic envelope and UUIDs directly.
- English documentation/code/UI, loopback admin, Origin/CSRF protection, atomic saves, global domain uniqueness.
- Preserve HTTP/WebSocket/stream behavior and source lifecycle semantics in full builds.
- Use fresh isolated configs/backends. No tests against the user's config or automatic restart of their instance.
- Exclusion applies to imports, assets, API and background work, not merely hidden UI.

## Review Focus

- A failed module initialization must close earlier modules and leave no discovery loops (Task 1).
- An absent module in configuration must error explicitly instead of losing settings (Task 1).
- Deletion racing an upgraded connection must cancel the correct owner (Tasks 2–3).
- Core assets must not accidentally embed the old all-features forms/scripts (Task 4).
- A full build may pass while individual builds fail from hidden cross-feature imports (Task 5).

### Task 1: Contracts, registry and generic configuration

**Files:** Create `internal/feature/module.go`, `host.go`, `lease.go`, `internal/app/registry.go`, `registry_test.go`, `leases.go`, `leases_test.go`; modify `internal/app/config.go`, `config_test.go`, `manager.go`; create `internal/app/modules_test.go`.

**Interfaces:** Implement the exact Route, Descriptor, State, RouteView, Snapshot,
Module, Host, Draft, Scope, Lease and Commit types/signatures from the contract.
`internal/feature` defines interfaces/errors/HTTP helpers; registry and ownership
implementations live in `internal/app`. Registration factories are `func() feature.Module`.
Core does not expose raw settings through HTTP.

- [ ] Write `TestEmptyRegistry`, `TestDuplicateModule`, `TestModuleInitRollback`, `TestUnknownConfiguredModule`, `TestRouteDomainCollision`, `TestAtomicModuleSettings`: assert core with no factories works, invalid registrations fail, started modules close on failure, missing module errors include its tag, cross-module collisions fail, failed writes leave state intact.
- [ ] Run `go test ./internal/feature ./internal/app` and observe failures for missing behavior.
- [x] Implement contracts/registry and new version-6 envelope (routes plus module-settings map), UUIDv4 identities and module validation. Reject older versions and unknown fields; remove legacy route-kind field decoding. Move feature data out of the core config model. Preserve UUIDs on edits; recreation generates new UUIDs.
- [ ] Add `TestCommitRevisionConflict`, `TestWatchCoalescing`, `TestScopeResetAcquireRace`, `TestTrackAfterClose`, `TestSettingsCounterKeepsStreams`, `TestDuplicateJSONKeys` with the contract’s exact conflict, cleanup, and non-revoking counter semantics.
- [x] Implement lifecycle rollback/shutdown and post-commit events; keep dispatch module-neutral. Implement route generations, scoped resources, synchronized lease acquisition/reset, tracked Hijack/Flush/Unwrap support and exportable-only module dispatch. Follow the atomic commit point and partial-start rollback rules.
- [x] Run race tests for the new registry and storage, adapt fixtures to v6, and review a scoped checkpoint. Later tasks replace feature-specific manager paths, not restore old configuration compatibility.

### Task 2: Local, Remote and SSH modules; core routing

**Files:** Create `internal/modules/local_ports/module.go`, `remote_http/module.go`, `ports_over_ssh/module.go` and tests; refactor `internal/web/server.go`, `remote.go`, `ssh.go`; update `internal/app/manager.go`; add tagged `cmd/zelport/module_local_ports.go`, `module_remote_http.go`, `module_ports_over_ssh.go` and common registry setup in `main.go`.

**Interfaces:** Each module exports `New() feature.Module` and privately owns its typed settings. Core domain lookup resolves a feature.Route and invokes its module handler. Shared proxy/status helpers belong in `internal/feature/http.go`, never in a feature-importing core file. The SSH module alone owns SSH pools and imports `internal/sshpipe`.

- [ ] Move/add tests for Local Aux routing, Remote base paths/TLS/Origin, SSH setup/backoff, HTTP 403, streaming and WebSockets. Add `TestCoreUnknownRoute`, `TestModuleRouteDeletionRace` and test renamed UUID routes retaining module identity.
- [x] Run affected tests to establish failing registry dispatch, then move handlers/state into modules and extract only genuinely shared HTTP helpers. Preserve existing proxy semantics and explicit operation ownership.
- [x] Add positive tagged factory files, one per module, without negative/default-all constraints. Main starts only registered modules. Remove all Local/Remote/SSH type branches and imports from core.
- [x] Run core, each of the three single-module and combined tests/builds; assert removed module endpoint/asset lookups fail. Review and checkpoint scoped changes.

### Task 3: Docker Sandboxes and Containers modules

**Files:** Create `internal/modules/docker_sandboxes/{module,manager,terminal}.go`, `internal/modules/containers/{module,manager}.go` and tests; move feature behavior out of `internal/app/manager.go`, `containers.go`, `internal/web/terminal.go`, `containers_test.go`; add `cmd/zelport/module_docker_sandboxes.go` and `module_containers.go`.

**Interfaces:** Both factories follow Task 2. Their private managers own discovery snapshots/reconciliation, errors and refresh timers. Only modules import CLI adapters; Sandbox owns Start/Open terminal/unpublish actions; Containers only discovers existing mappings. Core Host is the single authority for durable route/settings changes and event delivery.

- [ ] Add/move regression tests: stopped sandbox save without start, missing mapping reconciliation, CLI outage retaining routes, stable source identity, deletion/unpublish transaction and shared mapping protection, container published-port changes, terminal launch explicit only.
- [x] Run tests to expose unsupported generic dispatch; move typed data/logic into modules and remove feature fields/imports and background jobs from core startup/manager.
- [ ] Test that excluded modules never execute CLI discovery or create timers, and shutdown/failed startup cleans owned work. Preserve source error statuses without treating configured mappings as application health.
- [x] Run individual and full five-module race tests and inspect CLI dependency graphs; checkpoint.

### Task 4: Module-owned frontend and protected API

**Files:** Refactor `internal/web/assets/{index.html,app.js,app.css}`, add shared `internal/web/assets/ui.js`; create `internal/modules/<module>/assets/{module.js,style.css}` for each implemented module and Go embedding files; modify core web API/asset serving tests.

**Interfaces:** Implement the exact frontend mount/context/render/activate/deactivate/edit/dispose contract and protected endpoint envelopes from MODULE_CONTRACTS.md. Core owns one polling/fetch scheduler; module State() is sanitized and nonblocking. No feature markup or private configuration is included in the core shell.

- [ ] Write `TestAbsentModuleAPIAndAssets`, `TestModuleMutationSecurity`, `TestModuleStateRedaction` and registry asset tests. Assert missing modules return 404, all module mutations share exact Host/Origin/CSRF validation, and opaque settings are never blindly serialized into API state.
- [x] Implement core protected mounts and descriptor-driven state/assets; move feature-specific API handlers behind modules, keeping common validated route CRUD in core. Render only registered tabs and load only their same-origin scripts/CSS.
- [ ] Verify core-only and full builds in the browser with fresh configs: empty Dashboard, every existing editor, URL reload/Back, unavailable tab fallback, keyboard navigation, narrow screens, light/dark, visible operation errors and refresh behavior.
- [ ] Add `TestStateRevisionFiltering`, `TestMutationConflictNoRetry`, `TestModulePollingIntervals` and browser checks for repeated tab activation, stale async responses, editor preservation during polling and module-load failure isolation.
- [x] Verify feature strings/forms/scripts are absent from core embedded assets, not merely hidden. Run web/module tests and checkpoint.

### Task 5: Build matrix, release targets and cloud handoff

**Files:** Modify `Makefile`, `README.md`, `DESIGN.md`, `AGENTS.md`; create `scripts/check-module-builds.sh`; update cloud design/plan references to concrete interfaces established by Tasks 1–4.

- [x] Add a matrix script for core, each single module, all five and selected mixed builds. Use temporary build outputs/configs and `go list -deps` assertions proving absent feature/adapters are excluded. Assert core's command graph has no cloud/Noise/WebSocket/yamux feature dependencies.
- [x] Add `build-core`, `build-full`, and `check-modules` targets. Maintain one explicit implemented-feature tag list shared by full build/tests/vet; no core tag. Keep repository-wide `make build` and `make check` including the independent experiment. Do not list unimplemented cloud as available.
- [x] Run `go test -race ./...` and `go vet ./...` for core and each matrix tag set, `make check-modules`, `make check`, `make build-full`, `make build-core`, and `make build`. Expect all commands to exit zero; rerun affected cases after fixes.
- [x] Document actual selection syntax and default full release artifact. Update current-architecture docs only after implementation; retain pending cloud status. Inspect module ownership/security boundaries and summarize browser/build evidence.
- [x] Verify the implemented registry matches MODULE_CONTRACTS.md and the cloud plan; update all three together if implementation reveals a contract defect. Cloud-only must support imports without source modules. This is a compatibility check against specified interfaces, not a deferred architectural redesign.
