# Local Port Discovery Design

## Purpose

Local Ports keeps manual Aux-port configuration as its primary contract while
offering a read-only shortcut for choosing a TCP port that is already listening
on the current machine. Discovery is informational. It does not scan port ranges,
open network connections, validate HTTP, start processes, or change configuration.

## User experience

The **Aux port** field remains an editable number input accepting any value from
1 through 65535, including a port with no current listener.

The input gains a trailing disclosure button with the accessible label
**Choose a listening port**. Opening it lazily requests the current inventory and
shows a compact list sorted by port. Each row contains:

- the TCP port;
- the process name when available, otherwise `Unknown process`;
- the PID when available;
- a compact warning indicator when the listener also accepts connections beyond
  localhost.

The warning reuses the shared warning-indicator component. Its tooltip says
`The port listens on non-loopback interfaces too.`
It deliberately does not claim that the port is publicly reachable: host firewall,
VPN, routing, and NAT remain outside Zelport's knowledge.

Selecting a row copies its port into the Aux-port input and closes the disclosure.
It does not submit the form. A compact refresh action inside the disclosure runs
discovery again without replacing the dialog or losing the domain draft, current
input value, keyboard focus, or scroll position.

The list omits Zelport App's own public listener because selecting it would create
a rejected self-target. It may include a port already referenced by another route;
multiple domains may intentionally address the same backend.

An empty result says `No listening ports found.` A discovery failure is shown only
inside the disclosure as the non-blocking message `Listening ports couldn't be
detected. Enter the Aux port manually.` Manual entry and form submission remain
usable, and the refresh action offers a retry. Loading uses the shared spinner.
The control supports pointer and keyboard opening, arrows, Home/End, Escape,
outside dismissal, and focus return through the existing shared disclosure
primitive.

## Discovery boundary

Discovery reads the operating system's current TCP listener table. It never
connects to a discovered port and therefore cannot infer HTTP, HTTPS, service
health, or an application status code.

Only listeners reachable through the local loopback path are candidates:

- IPv4 loopback;
- IPv6 loopback;
- IPv4 or IPv6 wildcard listeners.

Listeners bound only to another specific interface address are excluded because
Local Ports targets localhost. The UI shows one row per port. Duplicate IPv4/IPv6
records are collapsed; when multiple processes share a port, their available
identities are combined in that row.

The scanner still observes the bind scope of every record for a candidate port. A
row is marked `listens_beyond_loopback` when any record for that port is bound to
an IPv4 or IPv6 wildcard address or to a specific non-loopback address. A port
whose records are exclusively loopback-bound is not marked. This is a conservative
bind-scope warning, not a network reachability test.

Process metadata is best effort. Permission restrictions may reveal a listener
without its owning PID or process name; such a listener remains selectable.

## Platform adapters

The supported release platforms receive separate, build-tagged adapters behind a
small internal scanner interface. Port discovery is the required contract;
process identity is best effort and never determines whether a listener is shown.

The common layer normalizes addresses, filters candidates, merges duplicate
records, sorts the result, excludes the active Zelport listener, and limits
resource use. Platform adapters only collect raw listener records:

- Linux reads `/proc/net/tcp` and `/proc/net/tcp6`, then resolves socket inodes
  through readable `/proc/<pid>/fd` entries and process names through `/proc`.
  Listener discovery does not depend on process-directory access.
- macOS invokes the operating system's `lsof` executable directly, without a
  shell, using its NUL-delimited field output rather than its human-readable
  table. Known system paths are checked before `PATH`.

Every scan has a short context deadline and a strict output-size limit. Parsers
reject malformed or oversized records without returning an unbounded partial
result. A socket closing during discovery is a normal race and is omitted rather
than reported as a module failure.

Platform parsing is isolated from the module and tested with synthetic fixtures
captured from both supported operating systems and architectures. Missing process
permissions produce partial metadata. Failure to obtain the listener table itself
produces a typed discovery error and never disables manual Local Ports entry.

No shared fallback parses localized `netstat`, `ss`, or formatted `lsof` tables.
No adapter uses a shell pipeline. Linux has no external executable dependency;
macOS relies only on its system `lsof` field interface.

## Module and API ownership

Local Ports owns the scanner, endpoint, response model, and UI. The module exposes
`GET /api/modules/local_ports/listeners`. The endpoint runs on demand with a
bounded context and returns a fresh immutable list. Results are neither persisted
in configuration nor included in route exports.

The response model contains only the port, a possibly empty list of process
identities containing PID and name, and the derived `listens_beyond_loopback`
boolean. Concrete bind addresses, command line, environment, executable path,
socket inode, username, and process arguments are not exposed.

The same authenticated administration boundary applies during remote
administration: discovery runs on the administered Zelport host and its result is
returned only through that existing protected session.

## Concurrency and lifecycle

Only one discovery request runs for a dialog at a time. The refresh action is
disabled and marked busy until completion. Closing the dialog or navigating away
cancels the frontend request; the server-side scan also has a short timeout.
Late responses cannot update a closed or replaced dialog.

Discovery state is independent of configuration revisions because it is read-only.
Saving the form still uses the current edit revision and existing validation.

## Verification

Tests cover:

- parsing Linux and macOS listener fixtures;
- loopback and wildcard inclusion, specific-interface exclusion, and deduplication;
- loopback-only, wildcard, and mixed-bind derivation of the bind-scope warning;
- missing process metadata and permission failures;
- omission of the active Zelport listener;
- endpoint methods, cancellation, timeout, and stable JSON output;
- manual entry without discovery;
- lazy loading, refresh, selection, keyboard interaction, failure recovery, and
  draft preservation;
- Linux and macOS builds with `CGO_ENABLED=0`;
- a native Linux and macOS test that opens a temporary loopback TCP listener and
  requires discovery to return its assigned port without assuming process metadata;
- the complete Go, JavaScript, and full-build checks required by `AGENTS.md`.
