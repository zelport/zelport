# Zelport UI design system

This document defines shared presentation and interaction rules. Module-specific
layouts compose these components; they must not redefine global component styles.
There is no frontend build, external font, library runtime or CDN dependency.

## Product language

Use **web service** as the canonical user-facing noun. Use **web app** only when
the subject is specifically an application with a browser user interface. The
primary flow describes where a service runs. It does not lead with its runtime
implementation.

## Foundations

The core stylesheet owns semantic colors, typography and spacing tokens. Preserve
the light/dark palette. Color never replaces a state label. Use neutral for missing
optional tools and disabled connections, green for available/connected, amber for
pending or degraded operation, and red for failures affecting configured services.

| Token / role | Value |
| --- | --- |
| `--text-body` / domains, targets, field values | 15px |
| `--text-help` / labels, help, metadata | 13px |
| `--text-label` / badges, column headings | 12px |
| `--text-heading` / section and dialog headings | 18px |
| Spacing scale | 4, 8, 12, 16, 24, 32px |
| `--row-padding-y` | 8px |
| `--panel-padding` | 16px |
| `--dialog-padding` | 24px; 16px on narrow screens |
| Control / panel radius | 6px / 10px |
| Control / compact action height | 40px / 32px |

Coarse-pointer and narrow-screen icon controls have at least 44px targets. Main text never shrinks on
mobile. Hero typography is an intentional branding exception. Monospace is for
secrets and raw diagnostics, not ordinary service names.

## Service lists and tables

`.service` is the shared service row used by source modules and imported services.
`.local-service-list` is its bordered surface; `.services` is its borderless variant
inside a resource accordion. `.data-table` supplies the same visual rules for
Services and relay catalog tables. They share tokens rather than forcing a table
and a list into the same DOM semantics.

- One outer surface, one subtle separator between adjacent rows.
- Domain: body size, semibold. Target: body size, normal weight.
- Row actions sit at the end. Footer creation actions follow the list.
- Desktop rows use 8px vertical padding; wrapped content expands naturally.
- Tables use `thead`, scoped column headers and `tbody`.
- Mobile tables become stacked items with `data-label` metadata. Column headings
  remain available to assistive technology. Actions align at the bottom right.
- Mobile module rows show the complete domain and target on separate lines, with
  indicators and actions below. Do not hide or ellipsize either value.
- Empty states use the same surface language, not a different dashed container.

## Panels and controls

Resource and Settings accordions share title typography, spacing and panel radius.
Expanded Settings start closed on each entry to Settings. Resource expansion may
survive data updates. Nested service lists do not add another outer border.

Primary buttons submit the main action. Secondary buttons edit, configure or
cancel. Remove is quiet in a row; destructive confirmation is explicit. No action
is triggered by hovering. Switches toggle a setting; checkboxes select or confirm.
Input adornments support prefix/suffix text and keyboard-accessible actions such
as reveal/copy, change/generate, and file editing/reload. Keep actions that operate
on one field inside that field’s InputAdornment. Use shared icons with explicit
accessible names and shared tooltips. Keep form submission and consequential
confirmation as text buttons. Field labels remain visible. Inputs use body-sized text.

A compact row can grow for touch targets; fixed-height rows must not clip content.
Focus indication is visible for keyboard navigation. Active text fields may use a
focus-within border. Never remove focus feedback without an equivalent indicator.

### Add service

Dashboard has one Add service action. It opens a drawer on the right: full
height, 520px wide (full width at 640px and below), with the title and close
control at the top and Cancel and the submit action fixed at the bottom. The
body scrolls. "Where does the service run?" sits at the top as a two-column
grid of source tiles, each with its own icon: Local Ports (This computer), Ports
over SSH (An SSH server), Remote HTTP (Any URL) and Ports over Relay (Your other
devices). The tiles are a radio group. Address follows, then the selected
source's fields.

Changing the source swaps only the drawer content. The drawer keeps its size and
position and keeps the typed address. Ports over Relay stays in the drawer: it
lists the services that other devices share, grouped by device with an
Online/Offline badge. Offline devices are shown but cannot be chosen. Picking a
service suggests its address, and the action reads Import service. Without a
relay or devices, one line explains the next step and links to Relay settings.
Hidden sources are not offered. The selected module owns its fields,
validation, persistence and runtime status. Escape, Cancel and the backdrop
close the drawer and return focus to Add service; a successful save returns to
Dashboard. Edit uses the same drawer without the source tiles. Unchanged polling
preserves the open drawer, focus and draft values. Modal dialogs remain only for
confirmations such as Remove service. Verify the drawer at 320 and 390 pixels.

The drawer slides in from the right and slides out on every close path (220ms
in, 180ms out, no motion with reduced motion preference).

Address keeps the `.localhost:<port>` suffix fixed at the right edge and aligns
the typed name to the right against it, so the field reads as one hostname.

The Local Ports service port and the SSH host field are also the filters for the
detected ports and configured hosts listed directly below them. Typing digits, a
process name or a host name narrows the list (case-insensitive); the initial value
does not filter. Use fills the field and shows the whole list again. A search icon
starts these fields; refresh (and the SSH help) sit at their end. The Local Ports
list grows to the drawer's remaining height (at least 180px). A value that is not a port number is
rejected with "Enter a port number or pick one below." beside the field. Local
Ports keeps manual entry available when listener discovery fails. Ports
over SSH accepts a valid configuration without requiring a successful connection.
The development server must be reachable through the existing system SSH setup
when the user expects the service to become available.

### Dashboard

Dashboard is the only list of services. The table has fixed columns: Address,
Source, Target, Status and the row menu (Open, Edit, Remove). Every row is one
line; long values truncate with an ellipsis. Status shows a dot or the shared
warning glyph with a short label (Listening, Configured, Available, No listener,
Exposed, Unencrypted, Unreachable, Unavailable). Its detail is a tooltip on the
label, never a second line. At 720px and below each service becomes a list item
with address, source, target and status on separate single lines; the status
detail follows the label on its line. The empty state is one short sentence.

### Source pages and Settings

Each source has a page in the Services menu: the source title and its
configuration form (at most 720px wide, the same width as the Settings
sections). Only Dashboard's table uses the full content width. Ports over Relay adds its device catalog
below the form. Source pages never repeat the service list.

Settings contains Built-in Relay, Service sources, Remote administration and
About. Remote administration has one switch, Allow remote file access (off by
default), which is read-only in a remote session. While it is off, remote
sessions show the SSH and CA path fields and Edit file disabled with a one-line reason. Service
sources controls visibility in the Services menu and Add service only; hidden sources keep
their services running.
About keeps Version, Hostname, Configuration, Listeners and Modules in one row.
Modules uses a compact count button whose non-modal popover lists the included
module names and maturity. Build time and uptime are not shown.

## Layers, feedback and lifecycle

Tooltips contain short supplementary text only. Popovers contain short contextual
content/actions and remain anchored to their trigger. Dialogs contain editing or
confirmation tasks; support Escape, focus return and scrolling on small screens.
The service drawer keeps its header and action footer outside the scrolling
body. Pending saves disable form controls and dismissal, and late responses from
closed dialogs cannot navigate the current view.
Long diagnostics expand in document flow and never overlap neighboring sections.

Errors belong to the affected view or form. Field errors appear beside the
field with aria-invalid and an associated description. Restore controls before
focusing and scrolling to the error. Unmapped errors receive focus themselves.
Pending controls prevent duplicate submission. Refresh stays in the same navigation position across tabs, disabled
with an explanation where it has no useful action.

User-facing errors state the problem first and put recovery guidance in a
separate sentence. Do not use semicolons, en dashes or em dashes as sentence
separators. Hyphens remain appropriate inside compound terms, command-line flags
and other code notation. Keep raw external diagnostics visually separate from
authored guidance because Zelport does not control their wording.

Persistent configuration warnings use the compact shared notice treatment: a
plain-language heading, a restrained amber marker, and technical identifiers in
monospace. They remain distinct from red operational errors and purple remote
administration context.

Remote administration uses a pale red page canvas (`#fff4f4` in light mode and
`#211719` in dark mode). Cards, forms and controls keep the shared neutral
surface palette so the context remains visible without reducing readability.

Unchanged polling retains DOM nodes, selection, focus, scroll and drafts. Current
revision/state updates independently of rendering. A display redesign must not
weaken mutation protection, expose secrets, or change module availability rules.

## Ownership and verification

Core owns shared primitives and tokens in `internal/web/assets`. Modules own their
content, API, forms and exceptional layout. Module selectors must be scoped to a
module or uniquely named component; generic selectors such as `.input-adornment`
are forbidden in module CSS. Module exclusion must also exclude its assets.

The development-only gallery at `scripts/ui/design-system.html` loads real core
and module CSS and shared JS. Run `python3 scripts/ui/preview.py` and open the printed loopback URL.
The server copies only the gallery and its required public assets into a temporary
directory; it never exposes the repository root. It never calls an administration API. Its theme
preview reuses the actual stylesheet dark-mode rules. It is not embedded or served
by the production administration server.

Review 1280px, intermediate widths and 390px (also 320px for long text), light and
dark, keyboard focus, empty/failed states, concurrent diagnostic disclosures,
dialogs, and unchanged/changed polls. Check real module screens as well as the
gallery. A passing DOM test does not prove visual consistency or accessibility.

## Primary navigation

The header holds the logo, Dashboard, the Services menu and Settings. Services
opens a menu of source pages sorted by name, with maturity badges; it is disabled
in a core-only build. The items remain visible and left aligned at every width;
below 480px the logo shows only its symbol. They never collapse into a hamburger.
The current destination uses the accent underline; a source page highlights
Services. Generic navigation buttons use aria-current rather than tab semantics.
The logo links to `#dashboard` without reloading the application.

The header shows a relay indicator only for a configured relay connection or an
enabled Built-in Relay. In remote administration a Remote · E2E label replaces it.

Relay connection status distinguishes Not configured, Disabled, Connecting,
Connected and Connection failed. The E2E field groups Reveal, Copy and Change
(edit icon) when a secret is stored; Change becomes Generate (shuffle icon) while
editing. Generation remains an unsaved draft. Reveal/Copy are disabled for an
empty draft.

## Layout rhythm

Tokens in `app.css` define every dimension: header 72px, page heading row 104px
with a 28px title, table header 40px, table rows 56px, controls 40px (44px on
touch), 24px card padding and grid gaps, 12px radius on surfaces and 8px on
controls. Do not introduce one-off sizes in modules.

Text buttons start a task, change data or navigate: Add service, Import service, Save,
Discard, Remove, Cancel. Icon-only buttons are reserved for repeated, universally
understood actions in tight spaces (row menu, close, field adornments) and always
have an accessible name and the shared tooltip. There is no gear icon.

## Shared JavaScript components

Use `ctx.ui` from module frontends; core components live in
`internal/web/assets/ui.js`. Keep module-specific fields, API calls, validation,
revision handling and secret lifecycle in their owning modules.

- `icon` owns SVG paths and decorative accessibility attributes;
  `adornmentButton`, `refreshControl` and `experimentalIcon` use it.
- `disclosureMenu` owns pointer versus keyboard opening, current-item focus,
  arrows, Home/End, Escape, outside dismissal and listener cleanup. Services
  navigation and port mapping pickers use it with their own semantics. Open
  pickers also close when their containing dialog closes.
- `dialogHeader` assigns the accessible name. `showDialog` owns mounting and
  removal on every native close path, accepts initial focus and a cleanup hook,
  and closes on the owning module's abort signal. Use the cleanup hook to erase
  secrets. Native dialog focus restoration remains in effect.
- `serviceModule(ctx, definition).edit(route, group, options)` opens the service
  drawer. `definition.fields(settings, group, state, {address, isNew})` returns
  `{element, read, dispose}`; `read` may throw to report a missing choice. With
  `options.container` set to the open drawer, `edit` swaps the content in place
  and disposes the previous fields instead of opening a second dialog.
- `formActions` creates Cancel, submit and an inline error. `formFeedback`
  associates field errors, reveals collapsed ancestors, and restores focus and
  visibility after failed actions. Credential rotation retains its specialized
  pending-state controls.
- `renderChanged` is the shared rendering gate, including generic service lists.
  Update current state first; the gate compares displayed data and only records
  successful renders. Unchanged polls retain DOM nodes and selection.

These helpers require no external library, CDN or frontend build. New source
modules should compose them instead of duplicating interaction handlers.


## Source configuration forms

Every source page uses `moduleViews(ctx, {configuration, overview, overviewPlacement})`.
Configuration is always visible. `overviewPlacement` is `hidden` (default),
`before` (Built-in Relay runtime status) or `after` (Ports over Relay catalog).
The module toolbar holds only content-specific actions such as Refresh catalog.

Forms stay compact: fields, a one-line hint where needed and a right-aligned
Discard / Save footer. Ports over SSH has one path field with Edit file and host
refresh adornments and shows file status only when the file is missing. Remote
HTTP has the additional CA bundle (Edit file, Reload certificates) and an outbound
proxy with authentication in a disclosure; proxy passwords are write-only. Local
Ports has Automatic port detection, Ignored ports and Ignored processes. Drafts
survive polling and navigation; conflicts require explicit retry or discard.

### Configuration file editor

Remote HTTP and Ports over SSH expose one **Edit file** action. Use the shared `fileEditor` dialog: saved path, remote hostname,
labelled monospace textarea and Cancel/Save. The dialog uses available width up
to 960px, keeps text scrollable in narrow/short viewports and inherits theme
colors. Keep drafts through polling, errors and conflicts. A dirty close asks
for explicit discard. Save uses the loaded file version/revision, never the
latest polled revision. Do not preload file contents into module state.


Tooltips size to their text up to 300px and use a bordered pointer aimed at the
trigger center. Prefer placement above, flip below near the top, and keep the
bubble inside viewport edges while adjusting the pointer. Show one tooltip at
a time. Action tooltips close when activated or blurred; Escape, scrolling and
resizing dismiss them. Shared behavior applies to module action icons, status
indicators and truncated text, including inside dialogs.
Use `setTooltip` for existing controls and changing descriptions. Reuse the
existing tooltip when an action changes between Reveal and Hide. Header status,
footer links, file paths and dialog close icons use this same component; HTML
`title` attributes must not introduce browser-native tooltips. Keep disabled
action explanations visible in the form rather than repeating them in tooltips.

## Audit consistency rules

Services displays route diagnostics and module-provided listener observations
in the Status column. Local Ports asks the operating system for listening ports
once when Services opens and again only on Refresh status or the next visit;
the tooltip states when they were checked. Changed discovery settings or routes
invalidate the observation. There is no background re-read and no network probe.

All configuration forms enable Save and Discard only when
values differ from saved settings. Reverting an edit returns to the clean state,
including edits made while a save is pending. Discard restores saved values
without requiring a network reload.

Removal confirmation identifies the service, says the service itself keeps
running, and ends with a shared Cancel / Remove footer. Close controls never
shrink below 44 by 44 pixels.

Escape is consumed by a tooltip only while that tooltip is visible. Otherwise
it reaches the enclosing dialog or popover. Popover-trigger focus waits until
the native transition completes before considering a tooltip.

### Shared relay catalog

A provider card lists source service addresses including that provider's Zelport
port and the Imported / Not imported state. One plus icon in the device header
opens import; multiple services use a labelled service chooser. Service rows
have no creation buttons. Repeated import proposes an unused local domain.
