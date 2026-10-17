# Documentation, Cleanliness, and Coordinated Publication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply every approved document disposition, enforce workspace-wide product/domain cleanliness, remove unused project scaffolds, and publish the verified 0.10.0 release set safely in dependency order.

**Architecture:** The workspace remains an optional coordinator over independent repositories. A binary-aware cleanliness guard covers every surviving project and generated artifact. Publication is a resumable dependency-ordered procedure that verifies each public boundary before exposing its consumer.

**Tech Stack:** Markdown, Python 3, Git, Go module proxy format, GitHub CLI/API, release assets, static-site verification, Make.

**Spec:** `docs/superpowers/specs/2026-10-17-zelport-0.10-relay-v1-clean-break-design.md`

## Global Constraints

- Begin document rewrites only from the already approved path-by-path dispositions in the spec; do not preserve superseded wording for historical context.
- Remove both unused hypothetical project scaffolds completely. Do not create remote replacements or dependencies for them.
- Keep the canonical design spec until implementation and release verification are complete; delete completed implementation plans before the final cleanliness scan if their execution details contain retired terms.
- Exclude only `.git` object/history storage from the cleanliness scan. Scan tracked, untracked, ignored, generated, compressed, and executable content.
- Publication authorization covers prepared commits, approved tags/releases/assets, creation of only `zelport-installer`, and the existing website deployment path. It excludes force pushes, tag replacement, visibility changes, other repository creation/deletion, and external catalog submission.
- Stop at any failed mandatory check, remote divergence, existing target tag/release, checksum mismatch, or authentication failure; report completed and pending stages without rewriting published state.

## Review Focus

- Confirm the documentation taxonomy is consistent and concise rather than a search-and-replace of old prose.
- Confirm third-party integrations are described as supported external tools, not bundled or globally required dependencies.
- Confirm the scanner cannot hide Zelport-owned violations through broad exceptions.
- Confirm every public module/artifact hash is matched to the local candidate before its consumer is published.

---

### Task 1: Rewrite workspace and repository instructions

**Files:**
- Rewrite: root `README.md`
- Rewrite: `workspace/README.md`
- Rewrite: every surviving `AGENTS.md`
- Add: `zelport-installer/AGENTS.md` if not already committed
- Delete with scaffolds: instructions belonging to removed hypothetical projects

- [ ] Make the root README a nine-project index: workspace, App, SDK, Relay, Omarchy Integration, installer, Homebrew tap, identity kit, website.
- [ ] Describe dependency order, independent repositories, full standard App build, diagnostic `zelport_core`, schema v1, Relay Protocol v1, and publication responsibilities.
- [ ] Apply the common author, Conventional Commit, synthetic-data, dirty-tree preservation, and pre-v1.0.0 history-reset exception consistently.
- [ ] Add repository-specific rules from the spec for SDK/Relay cryptographic identity, installer CalVer/sync, Homebrew revision, Omarchy native acceptance, web generated installer, and identity-kit provenance.
- [ ] Run a Markdown link check and `git diff --check` for every changed repository.
- [ ] Commit each repository separately with `docs: align repository instructions`.

### Task 2: Rewrite the App's living product and architecture documentation

**Files:**
- Rewrite: `zelport/README.md`
- Rewrite: `zelport/DESIGN.md`
- Rename/rewrite: top-level Ports over Relay overview to `zelport/PORTS_OVER_RELAY.md`
- Rename/rewrite: normative protocol document to `zelport/docs/ports-over-relay-v1.md`
- Rewrite: `zelport/docs/MODULES.md`
- Rewrite: `zelport/docs/MODULE_CONTRACTS.md`
- Expand: `zelport/docs/UI_DESIGN_SYSTEM.md`
- Rename/rewrite: `zelport/docs/LOCAL_INTEGRATION_API.md`
- Rewrite: `zelport/docs/HOMEBREW.md`
- Update: `zelport/TODO.md`
- Update: Containers and Docker Sandboxes module READMEs

- [ ] Keep README operational and short: App definition, full sources list, supported external tool boundaries, build/run/config schema v1, and links to deeper contracts.
- [ ] Put implementation architecture in DESIGN, complete wire/security requirements in the protocol document, and UI rules in the design system.
- [ ] State that modules are internal code boundaries, not user product editions; standard builds always contain all seven.
- [ ] Preserve approved live Containers/Docker Sandboxes operational knowledge and the user's unrelated edits.
- [ ] Make the local integration API explicitly read-only and use uppercase only in the proper product name “Zelport Integration”.
- [ ] Redistribute still-useful backlog items to TODO and delete superseded/historical instructions rather than documenting migrations.
- [ ] Run all referenced commands/snippets that can execute locally, Markdown links, and terminology checks.
- [ ] Commit: `docs!: redefine zelport app and relay v1`.

### Task 3: Rewrite Relay, SDK, Omarchy, tap, identity, and website documentation

**Files:**
- Rewrite: `zelport-sdk-go/README.md`
- Rewrite: `zelport-relay/README.md`
- Rewrite: `zelport-relay/tests/sdk-client/README.md`
- Rewrite: `zelport-omarchy/README.md`
- Rewrite: `zelport-omarchy/docs/CONFIGURATION.md`
- Rename/rewrite: `zelport-omarchy/docs/ZELPORT_SETUP.md`
- Rewrite: `zelport-omarchy/docs/NATIVE_ACCEPTANCE.md`
- Rewrite: `zelport-omarchy/docs/distribution/PUBLISHING.md`
- Rewrite: `homebrew-tap/README.md`
- Rewrite: `zelport-identity-kit/README.md`
- Rewrite: `zelport-web/README.md`
- Rewrite: `zelport-web/THIRD_PARTY.md`

- [ ] Apply every approved disposition verbatim in scope: stable 0.10.0 identity, no compatibility narrative, independent installer, read-only integration, mandatory native checklist, no implicit catalog submission, no tap/web release version.
- [ ] Explicitly distinguish support for Docker, Podman, sbx, and system SSH from bundling them.
- [ ] Make identity kit authoritative for canonical SVG provenance and website third-party inventory exact to shipped bytes.
- [ ] Validate code examples, relative links, bundle-required file lists, and asset references.
- [ ] Commit each repository separately using focused `docs:` commits.

### Task 4: Remove unused scaffolds and obsolete active-tree artifacts

**Files:**
- Delete: both top-level hypothetical project scaffold directories
- Delete: ignored/untracked obsolete binaries, archives, caches, temporary local proxy, and completed plan files after use
- Retain: canonical design spec and actual 0.10.0 verification records required by the spec

- [ ] Confirm scaffold directories contain no user-owned uncommitted work beyond the approved deletion scope, then remove them completely.
- [ ] Use `git status --ignored --short` and filesystem inventory to identify generated stale artifacts; remove only validated targets and report what is recoverable from Git.
- [ ] Delete old executables rather than renaming them; rebuild only `zelport`, `zelport-relay`, and diagnostic `zelport-core` from current source.
- [ ] Confirm the workspace now contains exactly the nine approved projects.
- [ ] Commit tracked deletions in their owning repositories with focused `chore:` commits.

### Task 5: Implement the binary-aware workspace cleanliness guard

**Files:**
- Add: `workspace/scripts/check-cleanliness.py`
- Add: `workspace/tests/test_cleanliness.py`
- Modify: `workspace/Makefile`

- [ ] Build prohibited tokens from fragments inside the checker so its own source remains clean.
- [ ] Test path-component and byte scanning across text, NUL-containing binaries, executable printable strings, ZIP members/content, tar/gzip members/content, symlinks, unreadable paths, ignored files, and missing removed-scaffold detection.
- [ ] Restrict prerelease validation to known product-version fields so ordinary UI/tooling preview language is not misclassified.
- [ ] Implement only exact path-and-token exceptions for genuine external proper nouns or verbatim third-party material; test that directory-wide and Zelport-owned exceptions are rejected.
- [ ] Scan the workspace coordinator and all nine surviving projects while excluding only `.git`; report every violation in one run and fail on unreadable in-scope data.
- [ ] Add the guard to `workspace make check` and run its unit suite against seeded positive/negative fixtures.
- [ ] Commit: `test(workspace): enforce product and domain cleanliness`.

### Task 6: Complete the all-repository local release preflight

**Files:**
- Add/update only actual verification records approved by the spec

- [ ] Confirm required author identity, intended commits, clean trees except preserved unrelated App edits, remote URLs, branch ancestry, and absence of target tags/releases.
- [ ] Run SDK, Relay, App, workspace, installer, tap, Omarchy, website, and identity checks defined by the other two plans.
- [ ] Build clean committed SDK and Relay module archives as exact `v0.10.0`, record `h1:` sums, and verify Relay/App consumers with `GOWORK=off` against a local file proxy.
- [ ] Build App release archives, checksum manifest, Homebrew formula, Arch package metadata, Omarchy bundle/checksum, and installer script/checksum from clean commits.
- [ ] Run the cleanliness guard after removing the temporary proxy and stale build outputs, then again against only the final rebuilt artifacts.
- [ ] Compare every intended checksum/URL/pin and confirm no generated website installer edit is present.
- [ ] If any environment-specific mandatory check cannot run, stop and request one explicit waiver naming that exact check and prerequisite.
- [ ] Record a signed-off preflight checklist with candidate commit IDs and hashes; do not publish until every item passes.

### Task 7: Reauthenticate and perform remote safety preflight

**Files:** none

- [ ] Reauthenticate the GitHub CLI with an account authorized for every existing repository and organization repository creation.
- [ ] Fetch without modifying branches; confirm each prepared branch is fast-forwardable and remotes have not advanced since local review.
- [ ] Query tags/releases to prove `v0.10.0` and `v20261017.1` do not exist; re-check immediately before each creation.
- [ ] Confirm exact local artifact hashes and repository visibility without changing visibility.
- [ ] Create only the approved `zelport-installer` remote, using the existing local repository and its intended visibility/policy; if visibility is not already unambiguously authorized by organization defaults, stop for user direction.
- [ ] Stop on any mismatch or insufficient permission and report the precise blocker.

### Task 8: Publish SDK and Relay in dependency order

**Files:** none

- [ ] Push the SDK prepared commit, create/push tag `v0.10.0`, and verify a fresh public `GOWORK=off` module download matches the recorded `h1:` sum.
- [ ] Only then push Relay prepared commit and tag `v0.10.0`; verify its public SDK dependency and its own fresh public module checksum.
- [ ] Confirm public module ZIP contents contain no uncommitted/generated files and pass the cleanliness guard.
- [ ] Stop if either public checksum differs; do not publish consumers.

### Task 9: Publish App, Omarchy, Homebrew, and installer

**Files:** none

- [ ] Push App tag `v0.10.0` without first moving App `main`; create release `v0.10.0` and upload the exact four archives plus `SHA256SUMS`.
- [ ] Redownload and verify every App asset; then fast-forward App `main` to the prepared commit.
- [ ] Push Omarchy commit/tag `v0.10.0`, create its release, upload the exact bundle and checksum, and redownload/verify both.
- [ ] Push the Homebrew formula only after all referenced App URLs and hashes verify publicly; run the public formula audit/install check.
- [ ] Push installer commit/tag `v20261017.1`, create the release, upload exact `install.sh` and `SHA256SUMS`, and redownload/verify both.
- [ ] Do not submit Omarchy to any external catalog.

### Task 10: Verify installer synchronization and publish the complete website

**Files:** none

- [ ] Observe the installer release workflow; require the website sync commit `chore(installer): sync v20261017.1` and ordinary fast-forward push.
- [ ] Compare SHA-256 across installer source commit, released asset, website repository copy, and deployed `/install.sh`.
- [ ] Rebase or fast-forward the already reviewed website-content commit onto the generated sync commit without modifying `install.sh`; rerun build, link, interaction, indexing, and cleanliness checks.
- [ ] Push the complete website source and verify deployment at `/`, absence of the temporary subpath and `noindex`, all public installation links, and unchanged public installer hash.
- [ ] The website receives no tag or independent release.

### Task 11: Final completion audit

**Files:**
- Review: all surviving workspace files and public artifacts

- [ ] Run the complete cleanliness guard locally and against downloaded public source/module/release artifacts.
- [ ] Verify exactly nine workspace projects, full App build behavior, schema v1 only, Relay Protocol v1 only, canonical versions, and all approved document dispositions.
- [ ] Verify every intended commit and tag is public, no force push/tag replacement/visibility change occurred, and unrelated App changes remain preserved.
- [ ] Report commit IDs, tags, release URLs, module sums, artifact hashes, public installer hash, website deployment result, and any explicitly waived environment check.
- [ ] Mark implementation complete only when every spec completion criterion is evidenced.
