# Ports over Relay

Ports over Relay lets one Zelport App instance consume HTTP services published
by another through an encrypted peer connection coordinated by Zelport Relay.
It is one of Zelport App's service sources, not a separate product edition.

## Roles

- A **provider** publishes selected local routes.
- A **consumer** imports selected published routes under its own `*.localhost`
  names.
- **Zelport Relay** or **Built-in Relay** coordinates discovery and peer
  traversal.

Standalone Zelport Relay is recommended for a persistent shared server.
Built-in Relay is useful when an existing Zelport App instance should also host
the relay. Both use the same server implementation and Relay Protocol v1.

## Trust and credentials

A group is one mutually trusted administration boundary. Every member can
authenticate peer identity, decrypt the group catalog and, when enabled, use
remote administration. Do not place mutually untrusted people or environments
in one group.

Relay access tokens protect server admission. Group credentials and device
identity protect end-to-end peer operations. Share only the enrollment material
needed by the joining instance; do not copy a complete configuration or private
device identity.

Start with fresh version-1 settings and credentials. Zelport does not import an
earlier relay protocol or configuration.

## Operating model

1. Configure a reachable standalone or Built-in Relay and its admission policy.
2. Create a Ports over Relay group on the first Zelport App instance.
3. Enroll the other instance and verify its pinned identity.
4. Publish only the intended local routes.
5. Import a published route under a unique local name on the consumer.

Imported services use the consumer's Localhost port. The consumer owns its
local name; the provider owns the published service and can revoke it. Catalog
updates and peer sessions are bounded, authenticated and tied to configuration
lifecycle. Removing a route, group or peer closes associated active work.

## Security boundary

The relay sees protocol routing metadata needed to connect clients. Application
requests, catalog contents and remote administration messages are protected
between Zelport App instances. This does not encrypt the provider's connection
to its own backend or make an insecure backend trustworthy.

Relay access, group membership and route publication are separate decisions.
Loop prevention rejects paths that would return to the same Zelport listener or
recursively traverse relay routes.

## Technical contract

The normative wire format, validation, limits and cryptographic contexts are in
[Relay Protocol v1](docs/ports-over-relay-v1.md). The Go implementation is split
between [zelport-sdk-go](https://github.com/zelport/zelport-sdk-go),
[zelport-relay](https://github.com/zelport/zelport-relay) and the App's
`ports_over_relay` module.
