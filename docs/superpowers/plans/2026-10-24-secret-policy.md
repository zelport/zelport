# E2E secret generation and minimum length

Implementation: inline under executing-plans; user approved the exact policy and requested execution. Existing dirty checkout retained; no commits requested. Baseline: /tmp/zelport-before-secret-policy.tgz.

Goal/spec: Generate exactly 40 uniformly random alphanumeric characters (a-z A-Z 0-9). New secrets accept any valid Unicode characters, including unchanged whitespace, with minimum 32 code points and existing maximum 256. Existing saved shorter secrets and canonical raw keys keep their identity. Relay admission tokens are unchanged.

Architecture: Separate new-input policy from persisted-key derivation. Keep Argon2 v1 unchanged. Use cryptographic rejection sampling in browser and backend generation.

Task 1 — secret.go, config.go, api.go, assets/configuration.js, existing secret/API/frontend tests and CONFIGURATION.md:
- [x] Write boundary, exact-preservation, generation and legacy-load tests; run focused Go and JS suites and observe policy failures.
- [x] Implement new-input validation at API boundary and browser, alphanumeric generation on both paths, preserve persisted derivation; update docs.
- [x] Run focused suites, full race/vet/frontend/packaging and builds; fresh review scoped against baseline.
- [x] Browser-check isolated configuration generation/validation and polling; replace bin and preview7070, preserve live settings, clean test resources.

Review focus: Unicode code points versus bytes; boundary whitespace; existing 16–31 character persisted identities; uniform random selection without modulo bias; API generate and browser draft/save agree. No wire protocol or admission-token changes.

## Verification and rulings
- RED: four Go policy tests and four frontend policy tests failed against the prior behavior. GREEN: focused Go suite and frontend cases pass.
- Full final `make check`: 214 frontend tests, full Go race suite, go vet, 39 release packaging and 3 Arch packaging tests pass. Linux and Darwin arm64 builds pass. `git diff --check` clean.
- Verification fixture repairs: removed AppleDouble metadata only from temporary sandbox source after the source parser encountered `._*.go`; created a temporary Git snapshot in that copy because release tests require git archive. No repository history changed.
- Fresh review: no core generation/KDF/compatibility findings. Important finding: native inputs strip line endings. Fixed with separate exact-secret storage and a revealed multiline editor; paste, hide/reveal, save, clipboard and manual-copy regressions observed RED→GREEN. Reviewer's declined items (full tests, build, live browser, unrelated dirty changes): full tests/build/browser verified by implementer; unrelated changes remain outside this task.
- Ruling: `newIdentity` still creates an internal raw protocol key. The user-facing credentials Generate endpoint replaces it with the new 40-character secret and derived key; browser Generate uses the same unbiased alphabet. This avoids changing the internal identity constructor contract.
- Ruling: count Unicode code points, preserve the existing maximum256. Persisted matching secret/key pairs bypass the new minimum; the API validates every nonempty replacement before derivation.
- Browser on isolated macOS7274: generation/reveal verified length40 and alphanumeric; Save succeeds;31 rejected;32 code points (30 emoji plus boundary spaces) preserved; pasted CRLF/CR secret survives hide/save/reveal and Copy byte-for-byte. Draft survives Overview/Configuration navigation and polling. 390px viewport has scrollWidth390; screenshot saved. Console error list empty. Dark theme was not separately exercised; styling uses the existing theme variables.
- Delivery: Darwin arm64 bin/zelport, build2026-10-24 18:38:38 UTC; preview7070 uses same binary and existing config. No live settings were mutated through the API. Temporary7274 stopped; temporary browser tab closed; viewport reset.
- Evidence (local task artifacts, not published): relay-secret-policy.png and relay-secret-policy-mobile.png.
