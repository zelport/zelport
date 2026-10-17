# Distribution, Installer, and Website Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce internally consistent 0.10.0 distribution metadata and artifacts, move the installer to its own CalVer repository, and promote the complete website while preserving an exact verified installer copy.

**Architecture:** Zelport App owns binaries and OS packaging metadata; the rolling Homebrew tap references immutable App assets; Omarchy owns its integration bundle; `zelport-installer` owns the script and release-driven website synchronization; the website owns content but treats `/install.sh` as generated input.

**Tech Stack:** Go release builder, Python 3, Bash, Homebrew formula, Arch PKGBUILD, QML/JavaScript, GitHub Actions, static HTML/CSS/JavaScript.

**Spec:** `docs/superpowers/specs/2026-10-17-zelport-0.10-relay-v1-clean-break-design.md`

## Global Constraints

- Start only after the runtime clean-break plan passes locally and SDK/Relay dependency artifacts are fixed at `v0.10.0`.
- Never fabricate a checksum. Generate each checksum from the exact artifact later uploaded or referenced.
- Use `0.10.0` in manifests/formulas/script pins and `v0.10.0` for tags/releases; installer script value is `20261017.1` and its tag/release is `v20261017.1`.
- The tap and website have no independent version or tag. Packaging-only tap corrections use numeric Homebrew `revision`.
- The website installer is generated only from an exact released installer asset and is never edited as website content.
- Do not publish, force-push, replace a tag, change visibility, or submit to an external catalog during implementation tasks.

## Review Focus

- Confirm the full seven-module build is used by every App archive and package.
- Confirm release metadata, asset names, URLs, and checksums describe the same immutable bytes.
- Confirm installer ownership has no duplicate editable source left in App.
- Confirm the promoted website contains no temporary route, indexing block, placeholder install link, or unused third-party asset.

---

### Task 1: Lock App 0.10.0 release artifacts with tests

**Files:**
- Modify: `zelport/scripts/test_release.py`
- Modify: `zelport/scripts/release.py`
- Modify: `zelport/packaging/homebrew/zelport.rb.in`
- Modify: `zelport/Makefile`

- [ ] Add failing tests for numeric `0.10.0`, tag-derived URLs at `v0.10.0`, four platform archives, stable member names/modes, `SHA256SUMS`, and full-module release compilation.
- [ ] Remove prerelease acceptance from the release-version validator; reject `v` prefixes and prerelease qualifiers.
- [ ] Make release builds use the ordinary full App composition with no module-selection tags.
- [ ] Generate a clean candidate under `bin/homebrew/0.10.0`; inspect every archive and execute each host-compatible binary's `--version`.
- [ ] Run `python3 -m unittest discover -s scripts -p test_release.py -v` and `make release VERSION=0.10.0` from a clean committed App tree.
- [ ] Commit: `build(release)!: prepare zelport v0.10.0 artifacts`.

### Task 2: Consolidate Arch packaging under its product-owned directory

**Files:**
- Move: `zelport/contrib/distribution/prepare.py` to `zelport/contrib/archlinux/prepare.py`
- Move: `zelport/contrib/distribution/test_prepare.py` to `zelport/contrib/archlinux/test_prepare.py`
- Rewrite: `zelport/contrib/archlinux/README.md`
- Modify: `zelport/contrib/archlinux/PKGBUILD.in`
- Modify: `zelport/contrib/archlinux/PKGBUILD`
- Modify: `zelport/contrib/archlinux/.SRCINFO`
- Add after actual validation: `zelport/contrib/archlinux/VERIFICATION.md`
- Delete: remaining `zelport/contrib/distribution/`
- Modify: `zelport/Makefile`

- [ ] Move generator/tests without compatibility entrypoints and update packaging checks.
- [ ] Make the validator accept stable numeric SemVer only and generate App `0.10.0` package metadata without prerelease conversion.
- [ ] Generate PKGBUILD and `.SRCINFO` from the exact App source archive; insert only the measured source checksum.
- [ ] Run unit tests, `makepkg --printsrcinfo`, source verification, and the available native/disposable Arch build.
- [ ] Create `VERIFICATION.md` only from the actual results and explicitly name untested targets.
- [ ] Commit: `build(arch): consolidate 0.10.0 packaging`.

### Task 3: Prepare the rolling Homebrew tap

**Files:**
- Modify: `homebrew-tap/Formula/zelport.rb`
- Rewrite: `homebrew-tap/README.md`
- Modify: Homebrew tests if present

- [ ] Populate four `v0.10.0` asset URLs and their measured hashes from Task 1; set formula `version "0.10.0"` and omit `revision` for the initial release.
- [ ] Keep the service and install behavior aligned with Zelport App and remove release-stage/compatibility language.
- [ ] Use `0.0.0` for local formula fixtures.
- [ ] Run `brew audit --strict --online zelport/tap/zelport`, `brew style`, formula install/test in the available clean environment, and direct checksum comparison with the candidate assets.
- [ ] Commit: `build(homebrew): prepare zelport 0.10.0 formula`.

### Task 4: Prepare Omarchy Integration 0.10.0

**Files:**
- Modify: `zelport-omarchy/manifest.json`
- Modify: `zelport-omarchy/scripts/bundle.py`
- Modify: `zelport-omarchy/tests/test_bundle.py`
- Modify: `zelport-omarchy/tests/test_model.cjs`
- Modify: `zelport-omarchy/tests/test_config.py`
- Modify: `zelport-omarchy/tests/test_helper.py`
- Rename bundle-required setup document to `docs/ZELPORT_SETUP.md`

- [ ] Set manifest version to `0.10.0` and update bundle inputs to the approved current filenames.
- [ ] Replace product prerelease fixtures with neutral `0.0.0`; remove old response fallbacks and missing-module product variants.
- [ ] Run all Python and Node tests, generate the deterministic bundle twice, and prove identical SHA-256 output.
- [ ] Perform the mandatory native Omarchy acceptance checklist; if its exact environment is unavailable, stop and obtain an explicit waiver naming only that check.
- [ ] Commit: `feat(omarchy)!: prepare integration v0.10.0`.

### Task 5: Create the standalone installer repository locally

**Files:**
- Create: `zelport-installer/AGENTS.md`
- Move: App `install.sh` to `zelport-installer/install.sh`
- Move: App installer tests to `zelport-installer/tests/test_install.py`
- Move/rewrite: App installer guide to `zelport-installer/README.md`
- Add: `zelport-installer/Makefile`
- Add: `zelport-installer/LICENSE`
- Add: `zelport-installer/.gitignore`
- Add: `zelport-installer/.github/workflows/release.yml`
- Delete from App: installer source, tests, guide, and old website-sync workflow
- Modify: `workspace/Makefile`

- [ ] Initialize the local repository with branch `main` and required author identity; do not create the remote yet.
- [ ] Move the canonical implementation and tests, preserving history where Git can represent it, and leave no duplicate editable source in App.
- [ ] Set `ZELPORT_INSTALLER_VERSION=20261017.1` and `ZELPORT_RELEASE=0.10.0`; remove future-platform notices and retired terminology.
- [ ] Pin the reviewed Arch recipe commit and its measured SHA-256 only after that App commit exists.
- [ ] Add checks for Bash syntax, complete mocked platform/update behavior, version/pin exactness, and deterministic checksum manifest generation.
- [ ] Make the release workflow trigger only on `vYYYYMMDD.N`, verify tag/script equality, create `install.sh` plus `SHA256SUMS`, and use read-only permissions except the release upload requirement.
- [ ] Run `bash -n install.sh`, the complete Python suite, and `make check`.
- [ ] Commit in App: `refactor(installer)!: move installer to standalone repository`.
- [ ] Commit in installer: `feat: establish standalone zelport installer`.

### Task 6: Implement release-driven installer synchronization

**Files:**
- Modify: `zelport-installer/.github/workflows/release.yml`
- Add: `zelport-installer/scripts/verify-release-sync.py`
- Add: `zelport-installer/tests/test_release_sync.py`
- Treat as generated only: `zelport-web/install.sh`

- [ ] Test a workflow helper that verifies the released `install.sh` against the released checksum manifest, refuses mismatched/multiple assets, and emits the exact expected website commit message.
- [ ] After release creation, make the workflow download the exact tagged release assets, verify them, obtain a repository-scoped installation token, and copy only the verified script into the existing website repository.
- [ ] Configure author identity and commit message exactly `chore(installer): sync v20261017.1`; use ordinary push and fail visibly on divergence.
- [ ] Add idempotence: identical website bytes produce no commit and report all compared hashes.
- [ ] Run helper unit tests and static workflow validation.
- [ ] Commit: `ci: sync verified installer releases to website`.

### Task 7: Promote the complete website to root

**Files:**
- Replace root site source from approved content currently under `zelport-web/working/`
- Rewrite: `zelport-web/build.py`
- Rewrite: `zelport-web/README.md`
- Move: third-party inventory to `zelport-web/THIRD_PARTY.md`
- Retain/move: active assets, `content/home.md`, template, styles, scripts, and active dashboard fixture
- Delete: temporary landing page assets, temporary route tree, extra README, unused font licenses/assets, generated `dist/`
- Preserve exactly: `zelport-web/install.sh`

- [ ] Snapshot the installer SHA-256 before any website edit and make tests fail if the build or promotion changes it.
- [ ] Promote the complete site to `/`; remove the temporary subpath and indexing block from source, build output, canonical links, sitemap/robots behavior, and tests.
- [ ] Replace installation placeholders only with artifacts that have passed local candidate verification; public resolution is checked later during publication.
- [ ] Retain notices only for shipped Inter, Phosphor Icons, Omarchy artwork, and daisyUI assets; determine exact upstream versions and checksums from the shipped bytes.
- [ ] Remove unused Instrument Sans and Geist material and the already identified historical captures; retain the active synthetic dashboard illustration.
- [ ] Build twice from clean output, compare trees, serve locally, and run link, accessibility, interaction, responsive-layout, and no-index/no-temporary-route checks.
- [ ] Recompute `install.sh` SHA-256 and require it to equal the snapshot.
- [ ] Commit: `feat(web): publish the complete zelport website`.

### Task 8: Distribution self-review gate

**Files:**
- Review: all changes from Tasks 1–7

- [ ] Verify every artifact name, checksum, URL, script pin, manifest, and human label against the spec's version table.
- [ ] Verify App has no installer source/workflow and website has no editable installer provenance.
- [ ] Verify the tap has no tag/release version and the website has no product version.
- [ ] Verify no catalog submission, remote repository creation, push, tag, release, or deployment occurred.
- [ ] Run `git diff --check` in App, tap, Omarchy, installer, and website source repositories.
