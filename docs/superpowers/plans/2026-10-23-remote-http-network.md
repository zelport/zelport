# Remote HTTP Network Configuration Implementation Plan

**Goal:** Add custom CA trust and authenticated HTTP/SOCKS proxy to Remote HTTP.
**Architecture:** Module-owned validated settings and UI. Shared modulekit accepts explicit TLS roots and proxy endpoints, retaining lease tracking.
**Tech Stack:** Go standard library, native JavaScript/CSS, sbx.
**Spec:** docs/superpowers/specs/2026-10-23-remote-http-network-design.md

## Constraints and review focus

Preserve existing uncommitted UX/SSH work. No host Node or real credential/configuration edits. Focus certificate hostname validation and invalid-file recovery, HTTPS proxy trust independent of insecure origin, secret leaks in errors/state, forwarding Proxy-Authorization, revision/scope races and WebSocket cancellation.

- [x] Backend: write failing behavior tests, implement remote_http settings/CA/proxy and neutral modulekit options, verify targeted race tests.
- [x] Frontend: write failing configuration/draft/password/conflict tests, implement concise module form, verify frontend suite.
- [x] Integrate, document schema/runtime behavior, run full checks/module matrix and independent review.
- [x] Refresh7070, test browser settings/narrow viewports with controlled fixtures, restore preview configuration and record evidence.

## Verification

- Backend settings and six frontend behavior tests failed before implementation.
- Final clean snapshot: make check (142 frontend tests, full Go race suite, vet, 42 packaging tests), make build, macOS arm64 preview build and all 12 module compositions passed.
- Controlled tests cover custom/system trust, hostname mismatch, invalid/missing/replaced/malformed/oversized CA bundles, HTTP/HTTPS CONNECT, authenticated/unauthenticated SOCKS5 and SOCKS5H, proxy-side DNS, secret redaction, Origin/CSRF, stale writes, persistence and synchronous stream/WebSocket revocation.
- Independent review found a nested PEM BEGIN acceptance case. A failing regression was added, strict framing fixed, and the complete Go race suite and full checks passed afterward.
- Native macOS preview on7070 saved a temporary CA and authenticated local HTTP proxy. A temporary HTTPS service with TLS verification enabled returned the expected response through that proxy; fixture counters confirmed CONNECT and origin requests. Integrated browser blocked rendering the test *.localhost alias, so the transfer was verified with a local HTTP client using the service Host header.
- Browser verified invalid-path errors, write-only stored password behavior, saved settings, retained drafts across views, and no horizontal overflow at320/390 px.
- Removed only the test service, reset Remote HTTP defaults, and confirmed no test credentials remain in the temporary application configuration.
