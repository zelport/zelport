# Ideas and deferred work

Items here are non-binding product or engineering ideas. They do not authorize
implementation and should be removed when resolved.

## Configurable Localhost port

Consider storing the Localhost port in configuration schema v1 while retaining
an explicit CLI override. Resolve startup precedence, endpoint-hint updates,
conflict reporting and behavior when a saved port is unavailable before changing
the current `--port` model.

## Additional packaging

Evaluate native packages only after the `v0.10.0` release process is stable. Each
format must preserve regular-user execution, owner-only configuration, complete
third-party notices and the full standard Zelport App composition.

Potential targets include a maintained Arch package source, Debian-family
packages and other distribution-native formats. Omarchy Integration remains a
separate desktop consumer, not an App package.

## Relay performance evidence

If performance work becomes necessary, create new Relay Protocol v1 benchmarks
for catalog freshness hints, compact authenticated records, compression bounds,
idle behavior and separate-host latency. Record exact commits, fixtures and raw
results; do not reuse measurements from another protocol identity.

## Windows acceptance

Complete native Windows validation for configuration paths, owner-only file
replacement, loopback listeners, shutdown, console behavior and supported
external tools. Shared code should keep native path and process semantics in
platform adapters rather than assuming POSIX behavior.
