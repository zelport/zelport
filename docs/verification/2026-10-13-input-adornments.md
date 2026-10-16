# Input adornments and relay access-token rotation

Implemented local HTML/CSS/JS helpers for text prefixes, suffixes and inline action
buttons. Relay Settings uses reveal/hide, copy and change-token actions. The service
domain editor uses the same container for its scheme and domain/port suffix. No
Web Awesome code, assets, Node.js build step or new dependency was added.

The protected credentials POST supports `generate` and `replace` with configuration
revision checking. Tokens are validated as 32-byte base64url keys and stored in
canonical form. Replacement compares decoded keys to reject equivalent credentials.
Persistence precedes replacement of the relay server generation. Listeners remain
reserved; closing the previous generation revokes its upgraded connections.

Validation on macOS:

- `make check`: root and experimental race tests and vet, including all feature tags.
- `make check-modules`: compile-time feature/dependency matrix.
- `make build`: complete application, standalone relay and experimental commands.
- `go build -o bin/snitzel-core ./cmd/zelport`: core build.
- Targeted relay race tests: rotation, old-token rejection, new-token admission,
  active connection closure, restart persistence, failed persistence, stale revisions,
  concurrent rotations and equivalent-key rejection.
- Ten Node regression tests in `scripts/relay-ui.test.mjs` and
  `scripts/shell-ui.test.mjs`: drafts, metadata revision changes, delayed secret
  responses, navigation cleanup and dialog-owned errors. Node is used only as an
  optional test runner, not for building or running Snitzel.
- Isolated browser instances on ports 18080 and 18081: relay Settings layout,
  copy success feedback, reveal/hide, change-token dialog and custom-token mode,
  cancellation, draft retention across navigation, keyboard tab navigation,
  domain prefix/suffix editor, and core-only Settings. No console warnings/errors.
  Credential mutations themselves were exercised by Go API/network tests; browser
  checks inspected and cancelled the confirmation dialog.
- Full application restarted without arguments on localhost:8080 and the new
  Settings interface verified read-only. The user's token was not rotated.

Linux and Windows runtime/browser behavior was not exercised in this change.
