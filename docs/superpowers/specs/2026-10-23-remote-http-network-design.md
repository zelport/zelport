# Remote HTTP custom trust and outbound proxy

Implement module-wide Additional CA certificate file and outbound HTTP(S)/SOCKS5 proxy in Remote HTTP Configuration. Keep the compact UI and shared module views. Defaults preserve direct connections and system trust. Per-route TLS verification and Origin rewriting remain unchanged.

## Configuration and API

Persist modules.remote_http with ca_cert_path, proxy_url, proxy_username, proxy_password. Blank CA uses system trust. A nonblank absolute or ~/ path references a bounded regular PEM certificate bundle added to system roots. Do not return file contents. Reject malformed bundles and private key blocks on save. Missing/corrupt runtime CA fails closed with a recoverable module error, without blocking startup or unrelated module configuration edits. File updates must affect subsequent connections; old cached roots must not silently be used after read/parse failure.

Proxy URL accepts http://, https://, socks5:// and socks5h:// endpoints without embedded credentials, query or fragment. SOCKS5 uses proxy-side target name resolution as implemented by Go HTTP Transport. Blank proxy means direct, with no environment proxy fallback. Separate username and password are optional. State exposes proxy_username and has_proxy_password, never proxy_password. A blank password update preserves a stored password only for the unchanged endpoint and username. Changing either requires re-entering or explicitly clearing a stored password. Disabling proxy clears credentials.

GET /api/modules/remote_http/settings returns module feature.State. PUT accepts revision, ca_cert_path, proxy_url, proxy_username, proxy_password and clear_proxy_password. A successful atomic Host.Commit preserves route identities, resets outbound scopes, and synchronously closes old requests and upgrades. Stale edits return409 and preserve prior configuration. Exact Origin/CSRF remain core-owned. ValidateSettings stays configuration-only (no runtime filesystem dependency).

State data includes ca_cert_path, effective_ca_cert_path, proxy_url, proxy_username, has_proxy_password, configuration_error. Errors and diagnostics must redact proxy credentials and Basic headers, including failed dial errors. Settings errors must not echo submitted secret URL userinfo. Keep self-target guards effective and do not forward inbound Proxy-Authorization to origins.

## Transport and lifecycle

Extend modulekit.ProxyOptions neutrally with RootCAs and ProxyURL. Remote HTTP owns settings, parsing, CA loading/cache and scope-bound request leases. Standard Go transport owns CONNECT, SOCKS5 negotiation and proxy authentication. Preserve streaming, WebSockets, cancellation, origin trust checks, self-listener guards, response status, and other modules' default behavior. Added CA roots verify HTTPS origins and HTTPS proxies even when origin TLS verification is disabled; proxy TLS must remain verified. No new dependencies.

## UI

Two main fields: Additional CA certificate file and Proxy URL. Optional Authentication details contains username, password and explicit Clear stored password only when applicable. Empty password keeps an unchanged stored password. Short default/path/protocol hints, Save settings and Discard changes. No file contents or passwords on polling. Keep DOM, focus, selection and drafts across polls, view changes and revision conflicts. Server errors remain local to the form. Display recoverable configuration errors, with concise saved feedback.

## Verification

Temporary CA/leaf certificates and controlled HTTP CONNECT/SOCKS5 servers only. Verify direct trust rejection, custom CA success and hostname checking, HTTP/HTTPS/SOCKS proxy routing/auth and no-auth, credential redaction, no secret state response, failure atomicity, stale saves, connection revocation and WebSocket/streaming behavior. Run frontend draft/secret tests, full race/vet/build, module matrix, and browser preview on7070.
