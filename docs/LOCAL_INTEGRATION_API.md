# Read-only local integration API

Zelport App exposes a small loopback API for desktop integrations such as
Zelport Integration for Omarchy. It reports current service and runtime state;
it cannot create, edit, remove, enable or disable anything.

## Discovery

The active Localhost port is written to the per-user state file
`zelport/endpoint.json` using an atomic owner-only write. Integrations may use
that hint, then fall back to the default port `8080`. A stale or invalid hint is
ignored. The file is advisory and does not grant trust.

## HTTP contract

Clients connect directly to loopback and send an exact `Host: localhost:<port>`
authority. Redirects and non-loopback targets are rejected. Successful JSON
responses identify:

```json
{
  "product": "zelport",
  "api_version": 1
}
```

Consumers must validate both fields and the expected response structure before
using any data. Unknown versions or fields outside the documented response
model are not interpreted as a compatible service.

The API contains only read-only views already available to the local
administration UI: runtime identity, module status and configured service
summaries. It exposes no CSRF token, credentials, private relay material or
mutation endpoint.

## Trust and lifecycle

The integration is a consumer, not an owner. It never starts, stops, installs or
configures Zelport App. Failure to connect is displayed as unavailable/stale
state and must not cause configuration changes.

One Zelport App instance per desktop user is the supported local integration
model. The App and integration should run in the same user session so their
per-user state paths and loopback namespace agree.

## Verification

Contract tests cover exact authority, loopback-only transport, endpoint-hint
fallback, product/API identity, strict JSON validation, read-only methods and
absence of secrets. Native integrations additionally verify stale-state UI and
prove that interactions do not change App configuration.
