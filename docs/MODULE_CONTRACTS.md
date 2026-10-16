# Module contracts v1

**Status: implemented module boundary including cloud integration.** This document makes
[MODULES.md](MODULES.md) concrete. It covers in-process trusted modules, not a
sandbox for third-party plugins. The Go/JavaScript interfaces below are implemented. Cloud-specific scopes,
catalogs and dispatch flows describe the integrated cloud module.

## Packages and data ownership

`internal/feature` contains standard-library-only contracts and errors.
`internal/app` implements persistence, the registry and lifetime tracking;
`internal/feature` implements individual request leases.
`internal/web` implements the gateway, protected administration and shared assets.
`internal/modules/<id>` owns each feature. Module IDs equal their build tags.
No app/web/feature package imports a module, adapter or cloud protocol package.

```go
type Route struct {
    ID       string          `json:"id"`
    Module   string          `json:"module"`
    Domain   string          `json:"domain"`
    Settings json.RawMessage `json:"settings"`
}
type Descriptor struct {
    Maturity       string // required: stable or experimental
    ID, Name, TabID string
    UIPlacement    string // empty: standalone tab; "settings": shared Settings section
    Order          int
    Exportable     bool
    PollInterval   time.Duration
    Assets         fs.FS
}
type Summary struct {
    State, Detail, Target string
}
type RouteView struct {
    ID, Module, Domain string
    Summary            Summary
    Editor              json.RawMessage
}
type State struct {
    Revision uint64
    Routes   []RouteView
    Data     json.RawMessage
}
type Snapshot struct {
    EditRevision uint64 // Most recent user-visible commit in this process.
    Revision uint64
    Routes   []Route
    Settings json.RawMessage
}
type Module interface {
    Descriptor() Descriptor
    ValidateRoute(Route) error
    ValidateSettings(json.RawMessage) error
    Start(context.Context, Host) error
    Close() error
    State() State
    ServeRoute(http.ResponseWriter, *http.Request, Route)
    Admin() http.Handler
}
```

A factory is `func() feature.Module`; one positive-tagged main file registers it.
Descriptors are immutable; module ID is also the required tag. Reject duplicate
ID/TabID and unsafe asset paths. Sort by Order then ID: Sandboxes 10, Containers 20,
Local 30, Remote 40, SSH 50, Cloud 60. Dashboard belongs only to core. Tab IDs remain
`sandboxes`, `containers`, `local`, `remote`, `ssh`, `cloud`.

`State()` is a nonblocking, sanitized copy, including route summaries/editor
fields and optional inventory. It performs no network work and never returns
credentials or raw module settings. `State.Revision` identifies the configuration
snapshot used to build the state. Core ignores removed route IDs and marks stale
summaries while module reconciliation catches up. Summaries use explicit states:
`configured`, `ready`, `waiting`, `unavailable`, `stale`, `error`; ready means a
usable route, never verified application availability. Editor data is intentionally
allowlisted by each module; secrets require a separate protected explicit action.

## Host contract and committed changes

```go
type Scope struct { Name string; Generation uint64 }
type CommitOptions struct { ResetScopes []string; MetadataOnly bool }
type CommitResult struct { Revision uint64 }
type Draft struct { Routes []Route; Settings json.RawMessage }
type Host interface {
    Snapshot() Snapshot
    OwnServices() []Route
    GatewayAuthority(string) string
    Watch() (<-chan struct{}, func())
    Commit(context.Context, uint64, CommitOptions, func(*Draft) error) (CommitResult, error)
    Scope(string) Scope
    Acquire(context.Context, string, uint64, ...Scope) (*Lease, error)
    Track(uint64, Scope, io.Closer) (release func(), err error)
    Dispatch(http.ResponseWriter, *http.Request, *Lease)
    AddMiddleware(func(http.Handler) http.Handler) (remove func())
}
```

Each module receives a Host scoped to its own module ID. Snapshot includes only
its own routes and private settings; OwnServices returns copies of exportable
route envelopes across modules with Settings removed. Core retains full private
settings for dispatch. GatewayAuthority(domain) returns the domain with the actual Localhost port
(omitting port 80), so modules never guess listener configuration.
All returned slices/JSON bytes are deep copies.

Watch is a size-one coalescing notification channel with an idempotent unsubscribe.
It signals any committed configuration change, including other modules' route
changes. Read a fresh Snapshot/OwnServices after notification; events do not carry
secret data. Register the watch before reading the initial snapshot to avoid a
startup race. Closed host closes watchers. No callbacks execute under store locks.

Commit uses expected global revision. A user mutation may cross only intervening
metadata-only commits; a newer user-visible mutation or a future revision returns
`ErrConflict`. `Snapshot.EditRevision` identifies that floor. Internal
`MetadataOnly` commits always require exact CAS and increment the persisted
revision, but do not advance the user-edit floor. The floor is initialized to the
loaded revision on startup. Use this option only for counters/watermarks, never
effective configuration, credentials or authorization; core rejects route changes
and scope resets under it. Draft callbacks must preserve current metadata when
applying a user change. No network work occurs in a transaction. The pure draft callback may change only the caller's
routes/settings; modules must reload and retry after conflict. Core validates
module settings/routes, UUIDs, normalized unique domains and owner boundaries.
Validators may inspect immutable Host snapshots and cached inventory. Neither
validation nor draft callbacks may start transactions, perform I/O or mutate module
runtime state. Revision increments once per successful commit. No automatic retry
of user actions or externally executed commands.

The common route CRUD API runs the same core transaction path for the owning
module. Module endpoints use Host.Commit for multi-field updates, internal catalog
counters, reconciliation results and explicit provider actions. Module settings
update and route updates are atomic in one file; cloud pin/sequence writes cannot
race and overwrite a user change. Invalidations described below are installed
synchronously at commit, independently of asynchronous reconciliation.

## Configuration v6

```json
{
  "version": 6,
  "revision": 1,
  "modules": {
    "local_ports": {}
  },
  "routes": [
    {
      "id": "82958650-fb63-43c1-903b-f3b6f89b1667",
      "module": "local_ports",
      "domain": "example.localhost",
      "settings": {"aux_port": 3000}
    }
  ]
}
```

Missing file means an empty v6 config, revision zero; no write until the first
mutation. A selected module without saved settings receives `{}`. All module
validators accept this as disabled/default settings. Unknown configured module
IDs fail startup naming the missing tag, even if they have no routes. Only v6 is
supported; no migration or legacy decoding. Reject trailing JSON, duplicate object
keys, unknown envelope fields, invalid UUIDv4 IDs and duplicate IDs/domains. Each
module strictly validates its own schema. Core never drops unknown settings.

Use owner-only temporary files, fsync, atomic rename, and copy-on-write state. A
pre-rename failure preserves disk/in-memory config and existing active leases.
Successful rename is the commit point; install the new snapshot and invalidations
before acknowledging success. Serialize writers; keep reads available through old
immutable snapshots while the write is prepared. Report post-commit errors as
committed failures, never claim rollback after the atomic replacement succeeded.

Core owns generated UUIDs and domains. Route owner cannot change on edit; delete
and create instead. UUID is also the published service UUID. Typed settings:
Local `{aux_port}`, Remote `{target_url,skip_tls_verify,rewrite_origin}`, SSH
`{ssh_host,ssh_port}`, Sandbox `{sandbox_id,sandbox_name,sandbox_port,selected,deleting}`,
Containers `{container}`; preserve existing field semantics inside these objects.
Cloud import fields and module-private identity/credentials follow the cloud design.
No special cloud fields exist in the core envelope.

## Requests, leases and revocation

Every route dispatch acquires a Lease owned by core. Local gateway admission uses
an internal unrestricted route lookup; module Host.Acquire permits only exportable
routes. It can never target localhost administration or an import. A Lease captures
the exact route generation and any optional module scope generations atomically.
Acquire requires the expected configuration revision (the uint64 argument),
rejects a mismatch or removed/changed scope generations, and opens no backend.
Host.Track uses the same revision/scope gate for non-route resources such as a
cloud session; resetting its scope closes tracked resources synchronously.
A rejected Track immediately closes its supplied resource.

`Lease.Context() context.Context`, `Lease.Track(io.Closer) (release func(), err error)`,
`Lease.Close() error`, `Lease.Generation() uint64`, and `Lease.Route() Route` are the complete lease interface.
Track and Close are synchronized: adding a closer after cancellation immediately
closes it and returns an error. Release is idempotent and unregisters ownership.
Dispatch checks lease validity again, uses its captured route, attaches the lease
to the request context, applies registered middleware and invokes the owning
module. It does not perform a second name-based lookup. Helpers expose
`feature.RequestLease(r *http.Request) *Lease` to trusted module handlers.

Core wraps ResponseWriter preserving Flush, Hijack and Unwrap. Hijacked frontend
connections are tracked; Close cancels request context and closes tracked handles.
Each module also tracks backend connections/pipes through the same lease. Gateway handler
return releases normal request resources. A caller of Host.Dispatch owns its
borrowed lease through response serialization and must close it after the
in-memory HTTP connection finishes; closing before net/http flushes would truncate
the response. Hijacked connections remain tracked
until closed, not until ServeRoute returns. Exactly-once cleanup must be tested.

Editing/deleting a route invalidates all its active leases and closes their
connections, including local and exported requests. HTTP transports are currently
per-request with keepalive disabled; SSH pool reconciliation removes obsolete pools. Modules key pools by
route content generation, never just UUID; a stale dispatch cannot reuse an old
pool after invalidation. Module-private settings changes do not automatically
close every request (e.g. persisting catalog sequence). Callers list ResetScopes
when their semantics require revocation. A scoped Host can reset only its own
named scopes; names are bounded local identifiers, not global module names.

Cloud owns a `sharing` scope and a settings/client generation scope. On publication
disable its transaction updates `publish=false` and resets `sharing`. Core's commit
gate switches the snapshot and invalidates old leases atomically with new lease
admission. Cloud opens only after observing publish=true and captures the current
sharing generation; it passes that snapshot revision to Acquire. No new client acknowledges service-open without a valid lease.
A stale attempt races either into a lease closed by commit or gets rejected.
Catalog withdrawal/network cleanup follows local revocation; relay availability
cannot delay disabling sharing. Group/relay/token replacement resets the client
scope and synchronously closes old sessions before returning API success.

Start in descriptor order after all config validation. A failed Start closes the
failing module plus already-started modules in reverse order; Close is idempotent
and must tolerate partial startup. Shutdown blocks new requests, cancels leases,
closes hijacked connections, then cancels module contexts and closes modules.
Module Close waits for its own goroutines/processes; it cannot wait for another
module's locks or make a network operation necessary for cleanup.

## Middleware and cloud dispatch

Host.AddMiddleware installs a module-owned wrapper, deterministically ordered by
descriptor order, before route handlers on both gateway and leased internal
dispatch. It never wraps administration. Removal is idempotent and registered
only during Start/Close. No dynamic registration on every request.

Cloud middleware verifies/adds traversal metadata on exportable routes; the import
handler forwards the validated context into encrypted stream-open metadata. Core
only transports request context and headers, with no crypto/schema awareness.
Internal cloud serving uses an in-memory connection attached to a Lease; the HTTP
handler calls Host.Dispatch directly. Do not call the public gateway or allocate
a local TCP listener. Cloud controls Host/Origin translation and service-open
acknowledgments; source modules retain their own backend behavior.

## Administration endpoints and frontend contract

Core endpoints: `GET /api/state`, `POST /api/routes`, `DELETE /api/routes/<uuid>`.
POST has `{revision,route}`; creation omits ID, edits retain ID and owner. DELETE
carries the expected revision; no global unpublish flag. Both use the same store
and return the new revision. Sandbox-specific removal with mapping uses its own
protected module endpoint. Module APIs mount at `/api/modules/<module-id>/`;
module assets at `/assets/modules/<module-id>/`. Missing modules return 404.

Core applies exact localhost authority, no-store, Origin and CSRF to all mutations,
including explicitly revealing secrets. GET is read-only. State returns global
revision, descriptors (without fs.FS), runtime metadata and sanitized module states.
No generic endpoint returns raw module settings or the complete stored config.
A conflicting user revision yields 409; validation 400; missing route/module 404. Errors carry English messages; core errors use JSON and module action errors
may use plain text. UI refreshes after conflict; it never blindly
resubmits a mutation. Module inventory failures stay module-local and visible.

`Descriptor.UIPlacement` defaults to an independent module tab. A `settings`
placement requires an empty `TabID`; the shell mounts that module into its own
section inside `#settings`. Standalone tab IDs cannot be `dashboard` or `settings`.
Multiple settings modules are allowed, each with independent assets and lifecycle.

Every module's `assets/module.js` exports `mount(context)` returning:

```js
{
  render(moduleState),
  activate(),
  deactivate(),
  edit(routeView), // optional for modules that do not own routes
  dispose()
}
```

Core mounts each module exactly once into an owned hidden panel. Context is:
`{moduleId, panel, toolbar, signal, api, refresh, mutate, navigate, ui}`. `signal` aborts on
module dispose; event listeners use that signal. `api(path, options)` permits only
same-origin core API paths, adds CSRF on mutations, and rejects HTTP errors.
`refresh()` coalesces GET state requests. `mutate(work)` runs one user operation,
shows busy/error state, then refreshes; work rejection is displayed, not retried.
`navigate(tabId)` owns URL history. `ui` supplies shared DOM, dialog, field and
error helpers; no feature names or markup are hardcoded in it.

render receives a detached sanitized module state; it updates existing views and
must not reset open editors or duplicate listeners. activate/deactivate switch
visibility without remounting. edit receives the complete sanitized RouteView.
dispose closes dialogs, unregisters resources and aborts listeners exactly once.
Async results must check signal/generation before modifying the DOM. Failure to
load a module script marks that tab unavailable and does not break Dashboard.

Core owns one polling timer and one in-flight state fetch. Dashboard polls at 15
seconds if any selected module requests polling; a module tab polls at its declared
interval. Settings polls at the minimum nonzero interval of its contributors.
Sandboxes/Containers/SSH/Cloud/Relay declare 15 seconds, Local/Remote zero.
Hidden documents suspend polling. Focus/tab return requests fresh state. This
polls cached state only; explicit module refresh actions trigger discovery.
Core renders Dashboard rows from RouteView and calls that module's edit hook.
Styles are scoped beneath `[data-module="<id>"]`; shared tokens/components stay in
core CSS. Load only registered same-origin scripts/styles, consistent with CSP.

## Optional trusted administration bridge

`AdministrationProvider.ServeAdministration` handles only the reserved namespace.
`AdministrationHost.DispatchAdministration` calls a separately registered web handler
with typed `RemoteAdminContext` (consumer/provider IDs, managed hostname, external
authority, receiver service links). A module must authenticate and authorize this
context before dispatch; headers never grant access. Ordinary leases, exports and
middleware do not gain administration permission. Core-excluded cloud builds retain
namespace rejection without transport dependencies.

`CommitOptions.StrictRevision` requires the exact storage revision even for effective
writes. Internal read-modify-write snapshots use it so metadata updates cannot be
lost. `MetadataOnly` remains restricted to counters and watermarks, never peer pins
or alias authorization changes.

### Shared input adornments

The shell UI helpers include `inputAdornment(input, {prefix, suffix, startActions,
actions})`, `adornedField(label, value, type, adornments)`, and
`adornmentButton(label, icon, handler)`. Prefix/suffix text and optional buttons
share the input border. Labels target only the input; action buttons are siblings,
with accessible names and keyboard focus. Modules retain ownership of actions,
validation, secrets and lifecycle. These are local HTML/CSS/JS helpers with no
third-party components or frontend build step.

`context.mutate(work, {reportError: false})` permits a dialog to display its own
mutation error instead of duplicating it in the tab banner. It retains the shell's
serialization and refresh behavior; callers must catch and present failures.

### Maturity and module availability

Every descriptor must declare `Maturity` as `feature.Stable` or
`feature.Experimental`; missing or unknown values fail registration. Maturity is
implementation metadata, not user configuration. Stable standalone interfaces
are available by default; experimental ones require an explicit user choice.
Settings modules retain their own functional enable setting.

Core persists explicit navigation choices in the version-6 top-level
`module_availability` map (module ID to boolean). `GET /api/state` exposes the
effective availability, configured service count, `can_disable` and a reason for
each compiled standalone module. `PUT /api/module-availability` accepts
`{revision,module,enabled}` under the normal authority, Origin and CSRF protections.
The serialized configuration transaction checks route ownership and optional
module-specific conditions before writing. It never stops a module or resets
resource scopes. Unknown and Settings-only modules cannot be toggled.

A module may implement `feature.AvailabilityGuard.UnavailableReason(settings)`
to supply an additional blocker from its configuration snapshot. This method must
be pure: it must not acquire module runtime locks or access the host. Ports over
Relay blocks hiding while its desired connection setting is enabled. Any route,
regardless of health, also blocks hiding. Used modules are retained as available
on load and effective configuration commits, preventing updates from hiding them.

The shell mounts each module once. Availability changes hide/show its tab, retain
stable IDs and exclude hidden tabs from keyboard navigation. Unavailable deep links
and a remotely disabled active tab return to Settings. Settings modules mount inside
native details/summary accordions; opening/closing calls activate/deactivate while
preserving DOM, drafts and polling behavior.

### Header connection status

A frontend module may export `headerStatus(state)`. It returns `null` when its indicator should be hidden,
otherwise `{status, tone, lines}` with non-secret, plain-text runtime details;
`tone` is `neutral`, `ok`, `pending`, or `error`. The shell uses the descriptor name as the
compact indicator label and supplies an accessible native popover and navigation.
Hiding a module through UI availability also hides its indicator and closes its popover.
Modules own the status interpretation. The shell polls state every five seconds
while a status provider is loaded and the document is visible, including outside
that module's tab. Polling preserves open popovers and keyboard focus.

The optional `toolbar` mount container holds module-owned navigation actions.
The shell hides it when the module is not active; modules clean up their controls
on disposal. It is outside the tablist, preserving tab keyboard navigation.

Polling still invokes `render(state)` with the latest configuration revision.
Modules must refresh their action state even when no DOM update is needed. Use
`ui.setText(node, value)` for retained text and
`ui.renderChanged(node, displayedContent, draw)` for independently rendered
sections. Include time-dependent transitions such as expiration in displayed
content, but omit undisplayed counters and discovery timestamps. Do not detach
unchanged sections or overwrite form drafts during polling.
