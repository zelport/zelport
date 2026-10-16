# zelport

**Historical repository intent. Source has since been extracted; see the current [README](../README.md).**

Local HTTP gateway, administration UI, and compile-time feature modules.

The source modules stay in this repository. `ports_over_relay` integrates the
shared client SDK with gateway configuration and UI. `built_in_relay` integrates
the relay library with settings, UI, and owned server lifecycle; it does not
contain a second relay implementation.

Future dependencies: `github.com/zelport/zelport-sdk-go` and, for embedded relay
hosting, `github.com/zelport/zelport-relay`. No cloud management API client is
required by the gateway. Core remains implicit; features require explicit build
tags. The planned embedded relay tag is `built_in_relay`.

See the [workspace overview](../README.md) for repository boundaries and extraction notes.
