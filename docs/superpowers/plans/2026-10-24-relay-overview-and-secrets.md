# Relay Overview and user-entered secrets implementation plan

> Inline execution using superpowers:executing-plans; no further design gate because the user explicitly rejects more microiterations and requests a coherent repair.

**Goal:** A service-oriented overview and ordinary user-entered E2E secrets, verified across macOS host and Linux Docker Sandbox.
**Architecture:** Keep the relay protocol unchanged. Store the original entered secret alongside its derived group key; generated raw keys retain existing semantics. Group remote administration and services into one device card instead of two duplicate lists.
**Tech Stack:** Go, x/crypto Argon2id, embedded JS/CSS, CUA and sbx.
**Spec:** User requests in this conversation; concrete design below.

## Design and rulings
- User-entered E2E secret: minimum 16 Unicode characters, maximum 256, meaningful non-whitespace input; preserve exact input for reveal/copy. Existing canonical 32-byte base64url keys remain byte-for-byte compatible.
- Derive non-key inputs with Argon2id (time3, memory64MiB, threads4, key32) using a fixed versioned application domain salt so devices need only share the secret. No per-device random salt because that would prevent identical secrets from pairing. Fixed salt permits cross-group precomputation; minimum length and memory-hard KDF mitigate but cannot make predictable phrases strong.
- Persist optional group_secret and the protocol group_token. Validate their consistency. A module-owned one-entry digest/key cache serializes KDF work and avoids repeated derivation during metadata commits; no unbounded secret cache.
- Keep original secret out of state/diagnostics. Credentials reveal explicitly returns original if present. Replacement clears stale original when a generated key is chosen.
- Overview: Imported services, then one Shared services area with cards per other device. Each card owns its availability, one Open administration action and services. No standalone administration list, raw UUID addresses or meaningless provider port column. No-device state has one short explanation. Server connection is labeled Relay server, distinct from device availability. Configuration keeps its existing form.
- Ruling: use existing dirty work branch; baseline /tmp/zelport-before-cross-os-overview.tgz isolates this repair. Preserve live7070 configuration. No commits requested.

## Tasks
- [x] Reproduce host↔sandbox sharing before edits; controlled loopback backends and separate configs only.
- [x] Add failing secret/API/UI cases; implement KDF input handling with legacy preservation and concise validation.
- [x] Add failing Overview cases (no peers, no shared services, shared/imported/offline, remote admin and unchanged polling); implement unified cards.
- [x] Run relevant and full tests, fresh review, Linux/macOS builds. Verify 40-character synthetic secret across OS, both transfer directions, wrong/changed secret, revoke/recover, actual browser UI and narrow layout.
- [x] Update bin and preview, preserve user settings, clean temporary processes and port mappings.

## Review focus
- KDF identity stability and old generated-key compatibility, original-secret redaction, bounded memory/concurrency, no lost provider pins/imports.
- Distinguish local sharing, remote availability and relay admission; do not infer E2E mismatch solely from zero peers.
- DOM stability, inaccessible admin actions while offline, duplicate hostnames, import settings remain pinned.

## Evidence

- Cross-OS runtime: 13 recorded cases, separate macOS host and Linux Docker Sandbox, controlled loopback backends, protected relay. Real HTTP 200 with the expected backend body in both directions, including ordinary 40-character input, sharing revoke/recover and restart. Different secrets isolate devices; short input does not replace the saved key.
- Browser: populated and empty Overview, remote administration over the relay tunnel, final Configuration hint on 7070, desktop and 390px layout. Mobile admin action measured 44px; no horizontal overflow. Final preview console had no errors. Dark mode was not specifically exercised in this repair.
- Automated: 207 frontend tests pass; go test -race ./... and go vet ./... pass; packaging suites 39 + 3 pass. Linux native and Darwin arm64 builds pass. The final CSS-only touch-target adjustment was rebuilt and browser-checked after the full suites.
- Fresh independent review found no critical/important issues. Both minor suggestions resolved: gofmt and fixed v1 derivation test vector.
- Delivered Darwin arm64 bin/zelport, version dev-a0bead5-dirty, build 2026-10-24 18:20:48 UTC; preview 7070 runs that binary. Ordinary text secrets require updated builds on both devices. Existing generated keys retain their key identity.
- No user-setting mutations were issued on live 7070. Runtime metadata changes mean the whole configuration file checksum is not a preservation invariant.
- Test fixture interruptions: Linux backend initially stopped during restart, causing expected HTTP 502; restarted the controlled backend and confirmed HTTP 200 both ways. An initial test-import cleanup without CSRF returned 403; retried with the normal CSRF flow. Neither was treated as a passing transfer.
- Cleanup: stopped only test instances 7270/7271 and controlled backends 7272/7273, removed temporary relay mapping 8270, closed agent-created test tabs. Preview 7070 and original user tab retained.
- Evidence (local task artifacts, not published): relay-cross-os-results.jsonl, relay-cross-os-overview.png, relay-cross-os-mobile.png.
