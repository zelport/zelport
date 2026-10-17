# Native Release Binary Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build each Zelport App release once, execute the exact four packaged binaries on matching free GitHub-hosted OS/CPU runners, and publish only the bundle that passed every native acceptance test.

**Architecture:** One GitHub Actions workflow runs source checks, creates a single four-platform bundle, fans that immutable bundle out to native acceptance jobs, and conditionally publishes it through a verified draft release. A cross-platform Python harness validates the archive and exercises the supplied release executable as a reverse proxy, Local Integration provider, Ports over Relay client, and Built-in Relay server without rebuilding Zelport App.

**Tech Stack:** Go 1.27, Python 3 standard library, GNU Make, GitHub Actions, GitHub CLI

**Spec:** `docs/superpowers/specs/2026-10-17-native-release-binary-testing-design.md`

## Global Constraints

- Test `darwin/arm64` on `macos-15`, `darwin/amd64` on `macos-15-intel`, `linux/arm64` on `ubuntu-24.04-arm`, and `linux/amd64` on `ubuntu-24.04`.
- Use only standard free GitHub-hosted runners; do not use `*-latest`, larger runners, or self-hosted machines.
- Build the four `CGO_ENABLED=0` release executables and archives once; native and publication jobs must not rebuild or rewrite them.
- Pull requests and `main` use unpublished version `0.0.0`; stable `v<major>.<minor>.<patch>` tags use the numeric version without `v`.
- Keep temporary Actions artifacts for one day and keep published assets in immutable GitHub Releases.
- Use configuration schema v1, Relay Protocol v1 at `/v1/ws`, and `zelport-relay/v1/*` cryptographic domains without legacy compatibility.
- Treat Docker, Podman, Docker Sandboxes, and system SSH as supported third-party tools, not requirements of the four-platform acceptance matrix.
- Pin every GitHub Action to an immutable commit SHA and give `contents: write` only to the publication job.
- Do not run Node.js on the developer host.

## Review Focus

- A checksum-valid archive with an unexpected member, unsafe path, or wrong mode must fail before extraction or execution; Task 2 adds fixtures for each case.
- A binary reporting the wrong version, runtime platform, listener authority, or standard module set must fail with captured process diagnostics; Task 3 adds the native process assertions.
- A stalled backend, App, or relay connection must time out and clean up every child process; Tasks 3 and 4 add bounded failure tests and cleanup assertions.
- A missing third-party executable must remain a neutral module state and must not prevent base acceptance; Task 3 runs with a controlled minimal `PATH` and asserts all modules still load.
- A pre-existing release or interrupted asset upload must never overwrite assets or expose a partial public release; Task 6 pins preflight, draft verification, and cleanup behavior in workflow contract tests.

---

### Task 1: Align the Built-in Relay runtime identity

**Files:**
- Modify: `cmd/zelport/modules_standard_test.go`
- Modify: `internal/modules/built_in_relay/module.go`
- Modify: `internal/modules/built_in_relay/module_test.go`
- Modify: `internal/relay/integration/relay_runtime_test.go`

**Interfaces:**
- Consumes: the documented module ID `built_in_relay` from `docs/MODULES.md`
- Produces: descriptor, connection summary, asset, and administration paths consistently rooted at `built_in_relay`

- [ ] **Step 1: Change the standard-composition test to require the documented ID**

Update `TestStandardFactoriesContainEveryModule` to expect `built_in_relay` and reject the retired runtime ID `relay`.

- [ ] **Step 2: Run the focused test and verify the current descriptor fails**

Run: `GOWORK=off go test ./cmd/zelport -run TestStandardFactoriesContainEveryModule -count=1`

Expected: FAIL because the current descriptor reports `relay`.

- [ ] **Step 3: Rename the runtime identity without compatibility aliases**

Set the Built-in Relay descriptor ID and connection-summary ID to `built_in_relay`. Update module API paths and integration requests from `/api/modules/relay/...` to `/api/modules/built_in_relay/...`; do not accept the old path or old configuration key.

- [ ] **Step 4: Run focused and repository tests**

Run: `GOWORK=off go test ./cmd/zelport ./internal/modules/built_in_relay ./internal/relay/integration`

Expected: PASS.

- [ ] **Step 5: Commit the identity correction**

```bash
git add cmd/zelport/modules_standard_test.go internal/modules/built_in_relay/module.go internal/modules/built_in_relay/module_test.go internal/relay/integration/relay_runtime_test.go
git commit -m "fix(relay): align built-in module identity"
```

### Task 2: Validate release archives portably

**Files:**
- Create: `scripts/check_release_binary.py`
- Create: `scripts/test_release_acceptance.py`
- Modify: `Makefile`

**Interfaces:**
- Consumes: `scripts/release.py` platform names, stable version validation, archive names, and `SHA256SUMS`
- Produces: `verify_archive(bundle: Path, version: str, platform: str, destination: Path) -> Path`, returning the safely extracted `zelport` executable

- [ ] **Step 1: Add failing archive-validation tests**

Add tests named `test_valid_archive_is_verified_and_extracted`, `test_rejects_checksum_mismatch`, `test_rejects_missing_checksum`, `test_rejects_extra_or_unsafe_member`, `test_rejects_wrong_member_mode`, and `test_rejects_unknown_platform`. Fixtures contain only synthetic bytes and the three expected members.

- [ ] **Step 2: Run the new test module and verify it fails before the validator exists**

Run: `python3 -m unittest scripts/test_release_acceptance.py -v`

Expected: FAIL because `check_release_binary.py` or `verify_archive` does not exist.

- [ ] **Step 3: Implement strict bundle verification**

Implement `verify_archive(bundle: Path, version: str, platform: str, destination: Path) -> Path` with Python standard-library hashing and tar handling. Require one matching checksum entry, the exact member set `zelport`, `LICENSE`, and `THIRD_PARTY_NOTICES.txt`, regular files only, no path traversal, and modes `0755`, `0644`, `0644` before extraction.

- [ ] **Step 4: Add the new tests to packaging checks and run them**

Change `check-packaging` to discover `test_release*.py`, then run `make check-packaging`.

Expected: all release packaging and archive validation tests PASS.

- [ ] **Step 5: Commit portable archive validation**

```bash
git add Makefile scripts/check_release_binary.py scripts/test_release_acceptance.py
git commit -m "test(release): validate packaged executables"
```

### Task 3: Exercise the supplied binary as Zelport App

**Files:**
- Modify: `scripts/check_release_binary.py`
- Modify: `scripts/test_release_acceptance.py`

**Interfaces:**
- Consumes: the executable returned by `verify_archive(...)`
- Produces: `run_app_acceptance(binary: Path, version: str, platform: str) -> None` and CLI arguments `--bundle`, `--version`, and `--platform`

- [ ] **Step 1: Add failing host-process tests for the acceptance harness**

Add tests that supply small executable fixtures for wrong `--version`, early process exit, startup timeout, and ignored `SIGTERM`. Assert bounded failure, child cleanup, and inclusion of captured synthetic logs in the raised diagnostic.

- [ ] **Step 2: Run the process tests and verify they fail**

Run: `python3 -m unittest scripts.test_release_acceptance.ReleaseProcessTest -v`

Expected: FAIL because `run_app_acceptance` and its bounded process owner do not exist.

- [ ] **Step 3: Implement the base native acceptance flow**

Implement `run_app_acceptance(binary: Path, version: str, platform: str) -> None`. It must use a temporary `HOME`, `XDG_CONFIG_HOME`, and `XDG_STATE_HOME`; start an in-process HTTP backend; launch the supplied binary on an unused loopback port; and assert:

- exact `zelport <version>` output;
- `/api/state` version and platform;
- IPv4 loopback plus IPv6 loopback when supported by the runner;
- exactly the seven standard module IDs from the spec;
- a Local Ports route proxies a request and preserves the backend response;
- `/api/integration/v1/state` reports product `zelport`, API version `1`, and the route;
- missing external tools do not prevent startup or remove module descriptors;
- `SIGTERM` produces exit status zero before the five-second shutdown deadline.

On any failure, terminate owned children and include the App log in the exception without exposing the real user environment.

- [ ] **Step 4: Build and run one host-native `0.0.0` acceptance test**

Run `make release VERSION=0.0.0`, determine the current host's `darwin_arm64`, `darwin_amd64`, `linux_arm64`, or `linux_amd64` platform, and invoke `python3 scripts/check_release_binary.py --bundle bin/homebrew/0.0.0 --version 0.0.0 --platform <host-platform>`.

Expected: `PASS` for archive, startup, platform, modules, reverse proxy, Local Integration, and graceful shutdown.

- [ ] **Step 5: Commit base release-binary acceptance**

```bash
git add scripts/check_release_binary.py scripts/test_release_acceptance.py
git commit -m "test(release): exercise native app binaries"
```

### Task 4: Exercise Ports over Relay through Built-in Relay

**Files:**
- Modify: `scripts/check_release_binary.py`
- Modify: `scripts/test_release_acceptance.py`

**Interfaces:**
- Consumes: the process/API helpers and supplied release executable from Task 3
- Produces: `run_relay_acceptance(binary: Path) -> None`, called by the CLI after base acceptance

- [ ] **Step 1: Add failing relay lifecycle tests**

Add controlled tests proving that failure to connect within the deadline reports the last state and that terminating the check closes both App processes and the backend server.

- [ ] **Step 2: Run the relay-focused tests and verify they fail**

Run: `python3 -m unittest scripts.test_release_acceptance.ReleaseRelayTest -v`

Expected: FAIL because `run_relay_acceptance` does not exist.

- [ ] **Step 3: Implement the exact-binary relay smoke test**

Implement `run_relay_acceptance(binary: Path) -> None` using two instances of the supplied executable. Enable unauthenticated Built-in Relay on an unused loopback port in the first instance, connect both Ports over Relay modules with one freshly generated synthetic group token, publish a Local Ports service from the first instance, import it in the second, and require one end-to-end HTTP response through `/v1/ws`. Use only temporary configuration and bounded polling.

- [ ] **Step 4: Run the host-native acceptance CLI again**

Run the Task 3 CLI command for the current platform.

Expected: the final `PASS` also reports Built-in Relay and Ports over Relay end-to-end data flow, and both App processes exit cleanly.

- [ ] **Step 5: Commit relay release-binary acceptance**

```bash
git add scripts/check_release_binary.py scripts/test_release_acceptance.py
git commit -m "test(release): verify built-in relay data flow"
```

### Task 5: Add the source, build, and native matrix workflow

**Files:**
- Create: `.github/workflows/release.yml`
- Create: `scripts/test_release_workflow.py`
- Modify: `Makefile`

**Interfaces:**
- Consumes: `make check`, `make check-modules`, `make release VERSION=<version>`, and `scripts/check_release_binary.py`
- Produces: workflow artifact `zelport-release-<run-id>` containing the four archives, generated formula, and `SHA256SUMS`; job output `version`

- [ ] **Step 1: Add failing workflow contract tests**

Add tests asserting the exact event filters, four runner/platform pairs, `needs` ordering, one-day artifact retention, read-only default permissions, immutable action references, absence of `*-latest`, `0.0.0` for non-tags, stable tag parsing, and invocation of the acceptance CLI against the downloaded bundle.

- [ ] **Step 2: Run the workflow tests and verify they fail**

Run: `python3 -m unittest scripts/test_release_workflow.py -v`

Expected: FAIL because `.github/workflows/release.yml` does not exist.

- [ ] **Step 3: Implement verification, build, and native jobs**

Create one workflow for pull requests, pushes to `main`, and stable version tags. Pin:

- `actions/checkout` to `11d5960a326750d5838078e36cf38b85af677262` (`v4`);
- `actions/setup-go` to `924ae3a1cded613372ab5595356fb5720e22ba16` (`v6`);
- `actions/upload-artifact` to `ea165f8d65b6e75b540449e92b4886f43607fa02` (`v4`);
- `actions/download-artifact` to `634f93cb2916e3fdff6788551b99b062d0335ce0` (`v5`).

The source job runs `make check` and `make check-modules`. The build job then
runs once on `ubuntu-24.04`, derives and validates `version`, runs
`make release`, and uploads the entire output directory with
`retention-days: 1`. The native job matrix downloads that artifact on each
exact runner and runs the acceptance CLI. Set `fail-fast: false` so all
platform diagnostics are retained.

- [ ] **Step 4: Run repository workflow and packaging checks**

Run: `make check-packaging`

Expected: all packaging, acceptance-unit, and workflow-contract tests PASS.

- [ ] **Step 5: Commit the native workflow**

```bash
git add .github/workflows/release.yml Makefile scripts/test_release_workflow.py
git commit -m "ci(release): test binaries on native runners"
```

### Task 6: Publish only the verified bundle

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `scripts/test_release_workflow.py`

**Interfaces:**
- Consumes: the tested workflow artifact and validated stable tag/version output from Task 5
- Produces: one public GitHub Release whose assets are the unchanged four archives and `SHA256SUMS`

- [ ] **Step 1: Extend workflow tests with failing publication-safety assertions**

Require that publication runs only for `refs/tags/v<stable-semver>`, needs the complete native matrix, has job-local `contents: write`, refuses an existing release, creates a draft, downloads its uploaded assets into a fresh directory, revalidates `SHA256SUMS`, publishes only after validation, and removes only the draft created by the current job on failure.

- [ ] **Step 2: Run the publication contract test and verify it fails**

Run: `python3 -m unittest scripts.test_release_workflow.ReleasePublicationWorkflowTest -v`

Expected: FAIL because the workflow has no publication job.

- [ ] **Step 3: Implement guarded draft publication**

Add a tag-only `publish` job after the native matrix. Download the tested artifact, preflight that the tag has no existing release, create a draft with all four archives and `SHA256SUMS`, download those draft assets through authenticated `gh`, validate every checksum, and then make the draft public. Track whether this job created the draft and install an exit trap that deletes only that unexposed draft after a failure.

- [ ] **Step 4: Run workflow tests and inspect permissions**

Run: `make check-packaging && rg -n "permissions:|contents: write|gh release" .github/workflows/release.yml`

Expected: tests PASS; default permissions are read-only and only `publish` contains `contents: write`.

- [ ] **Step 5: Commit safe publication**

```bash
git add .github/workflows/release.yml scripts/test_release_workflow.py
git commit -m "ci(release): publish verified artifacts atomically"
```

### Task 7: Document and verify the complete pipeline

**Files:**
- Modify: `README.md`
- Modify: `docs/HOMEBREW.md`

**Interfaces:**
- Consumes: the workflow and acceptance behavior from Tasks 1–6
- Produces: operator-facing release documentation and a green four-platform GitHub Actions run on `main`

- [ ] **Step 1: Update release documentation**

Document that the four archives are built once, executed on matching native runners, and published unchanged. State that Homebrew and installer automation consume immutable App release assets and never rebuild them.

- [ ] **Step 2: Run local repository verification**

Run:

```bash
go test -race ./...
go vet ./...
make build
make check-modules
git diff --check
```

Expected: every command exits zero. No release archive or `bin/` output is staged.

- [ ] **Step 3: Commit documentation**

```bash
git add README.md docs/HOMEBREW.md
git commit -m "docs(release): describe native artifact gates"
```

- [ ] **Step 4: Push `main` and observe the real native matrix**

Push the accumulated commits to `origin/main`, locate the triggered workflow run, and wait for source verification, release build, and all four native acceptance jobs to complete.

Expected: the `0.0.0` workflow is green on `macos-15`, `macos-15-intel`, `ubuntu-24.04-arm`, and `ubuntu-24.04`; no publication job runs.

- [ ] **Step 5: Perform final branch review**

Review the complete diff against the spec, verify `git status --short --branch` is clean and synchronized, and record the green workflow URL in the handoff. Do not create a release tag as part of this implementation.
