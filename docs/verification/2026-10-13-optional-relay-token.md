# Optional embedded relay access token

Embedded hosting defaults `require_access_token` to true, including when absent
from existing configuration. Explicitly setting it false selects Noise NN admission
with a separate open-mode prologue and no PSK. The protected NNpsk2 path is unchanged.
Clients with an empty access token select open admission without downgrade/retry.
Peer/group E2E authentication remains mandatory. The standalone CLI still requires
a token file.

Token settings now live in Advanced. Open hosting displays a persistent warning
badge outside Advanced. Client settings explicitly distinguish clearing a token
from keeping the stored value through an empty input.

Validation completed on macOS:

- `make check`: root/core/full and experimental module race tests and vet.
- `make check-modules`: feature build and dependency-exclusion matrix.
- `make build`: full application, standalone relay and experimental commands.
- `go build -o bin/snitzel-core ./cmd/zelport`.
- Eleven frontend regression tests using the optional Node test runner.
- Admission matrix: open/open and protected/protected succeed; mixed modes fail.
- Embedded mode transitions revoke existing connections, retain listener ownership,
  preserve the stored token, survive restart and retain protection on write failure.
- Token rotation while open does not silently enable protection.
- Separate E2E groups remain separated on an open relay; empty E2E secrets fail.
- Client APIs preserve saved tokens for blank input and clear them only explicitly.
- Existing full HTTP, streaming, WebSocket, administration and reconnect integration
  tests also run against an embedded open relay with two isolated clients.
- Browser checks on an isolated loopback instance: protected defaults, Advanced
  placement, persistent open badge, warning text, connected client without a token,
  and client access-token checkbox/required-input behavior. Console had no errors.

No user credentials or protection settings were changed during testing. Runtime
and browser tests were performed locally on macOS, not Linux or Windows. These
checks are not an independent cryptographic audit.
