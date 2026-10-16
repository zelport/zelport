# Final product rename to Zelport

This change applies to the active gateway, SDK, and relay repositories. Frozen
archives, the historical experiment, original benchmark output, and dated
verification records retain their historical names.

## Changed names

- CLI output, logs, help, UI titles, runtime labels, and current documentation use Zelport.
- Gateway defaults use `zelport/config.json` in the existing platform config root.
- Relay environment variables use `ZELPORT_RELAY_*`; old `SNITZEL_RELAY_*` names
  are not accepted. Flags remain unchanged.
- Remote administration aliases use `*.zelport.localhost`.
- The traversal header is `X-Zelport-Cloud-Trace`.
- Relay reservation files use `~/.zelport/relay-locks`. SSH socket and temporary
  configuration filename prefixes also use Zelport.

## Updating an existing installation

Stop the old gateway and relay before switching, particularly because the relay
lock directory changed. Back up the current config. Either continue using its
existing location with `--config`, or copy the complete file to the new default
location with owner-only permissions. Preserve device and service UUIDs, private
keys, E2E secrets, admission tokens, provider pins, and catalog watermarks; do not
start with an empty config to perform the rename. No runtime automatic migration
or fallback to the old path is provided.

Update service definitions, environment names, scripts, and saved administration
URLs. Upgrade all connected clients together: administration catalog validation
and traversal headers changed. Do not mix old and renamed clients, since loop
metadata must be interpreted consistently. The protocol version remains 3 and
is not a compatibility guarantee for a mixed branding rollout.

No user's configuration, credentials, or running processes were changed by this
source rename. Newly built binaries are in each project's `bin` directory.

## Fixed protocol identifiers

The `snitzel-cloud/v1` and `snitzel-cloud/v3` domain-separation strings in HKDF,
Noise prologues, and authenticated catalog data deliberately remain unchanged.
They are cryptographic protocol constants, not product labels. Renaming them
would change derived keys, routing locators, and handshake authentication. This
rename does not change cryptographic algorithms or those constants.
