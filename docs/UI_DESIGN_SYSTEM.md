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
Dashboard and relay catalog tables. They share tokens rather than forcing a table
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

### Add service chooser

The Dashboard has one Add service action. It opens a shell owned dialog that asks
Where does the service run. The choices are This computer and Development server.
This computer opens the Local Ports form. Development server opens the Ports over
SSH form. Both choices explain their matching source and SSH requirements.
Remote HTTP and Ports over Relay appear under Other service sources. Remote HTTP
opens its form; Ports over Relay opens its configuration.

Keep an unavailable choice visible and disabled with a short reason. Do not copy
module fields into the chooser. The selected module owns its form, validation,
persistence and runtime status. Escape and the close action restore focus to Add
service. Selecting a source opens its form over the original page. Back returns
to the chooser and retains raw field values, selection and expanded details for
each source. Cancel and Escape leave the original page unchanged. Only a
successful save navigates to the selected source. Unchanged polling must preserve the open chooser, an open service form,
focus and draft values. Verify the dialog at 320 and 390 pixels.

Local Ports and SSH discovery pickers provide case-insensitive search and explicit
Use/Selected actions. Searching never changes the manual target, and Enter in a
search field must not submit the service form. Local Ports keeps manual entry
available when listener discovery fails. Ports
over SSH accepts a valid configuration without requiring a successful connection.
The development server must be reachable through the existing system SSH setup
when the user expects the service to become available.

The empty Dashboard explains how a service port becomes a named localhost
address. Its **Zelport runtime** disclosure starts expanded on desktop and
collapsed at 720px and below; subsequent toggles remain under user control.
The panel keeps Version, Hostname, Configuration,
Listeners and Modules in one row. Modules uses a compact count button whose
non-modal popover lists the included module names and maturity. Runtime facts
become a two-column grid at intermediate widths and one column on mobile; the
Modules field spans both intermediate columns. Build time and uptime are not
shown in this panel. Configuration paths and listener values wrap or scroll
within their own cells without widening the page.

Service Overview empty states and relay onboarding share the same full-width
card, padding, centered text and centered action below the message. Messages
use the entire inner card width and wrap naturally, without a character-width
limit. Relay onboarding uses the same message-and-action layout without an
additional heading. The single empty-state action is the primary next step.

## Layers, feedback and lifecycle

Tooltips contain short supplementary text only. Popovers contain short contextual
content/actions and remain anchored to their trigger. Dialogs contain editing or
confirmation tasks; support Escape, focus return and scrolling on small screens.
Service edit dialogs keep their header and action footer outside the scrolling
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

The shell exposes Dashboard, Services and Settings at every width. The three
destinations remain visible and left aligned with a consistent gap; they never
collapse into a hamburger or spread across the available row. All service sources, including experimental ones, appear in navigation by default;
explicit saved choices take precedence. This changes visibility only, not module
connections, sharing or listeners. Services is a
navigation disclosure containing enabled source modules, with flask icons for experimental modules. The icons have native Experimental
tooltips; button accessible names also include the maturity. The current module
heading repeats the same icon and accessible maturity label. Pointer opening keeps
focus on Services; keyboard opening focuses the current module. Navigation keyboard
focus uses a thin rounded outline and a soft background. The selected module highlights Services and its
own menu item, and appears as a heading above the module content. Refresh remains
at the right of the toolbar. The disclosure supports arrows, Home/End, Escape,
outside click and focus return; ordinary Tab navigation remains available.

Module hashes, history and lifecycle remain unchanged. Settings still owns embedded
relay hosting and module UI availability. With no available source modules, Services
is disabled. Generic navigation buttons use aria-current rather than tab semantics.

The header brand links to `#dashboard` within the current document. Returning home
must not reload the application or rebuild the navigation.

Relay connection status distinguishes Not configured, Disabled, Connecting,
Connected and Connection failed. First-use and disabled overviews show guidance
and a configuration action. Header Open settings selects Configuration directly.
Relay setup, configuration and onboarding use the full content width of their
panels, matching Settings. Do not constrain their fields to a narrower column.
The E2E field groups Reveal, Copy and Change (edit icon) when a secret is stored;
Change becomes Generate (shuffle icon) while editing. Generation remains an
unsaved draft. Reveal/Copy are disabled for an empty draft. Instructions wrap
outside secret inputs; no separate action row sits beneath the field.

Every module exposes Overview and Configuration as a shared segmented control in
the module heading. This local control uses a filled selected segment instead of
repeating the global underline grammar. On wide layouts it sits to the right of
the module title and maturity badge. At 720px and below the heading stacks, the
control stays aligned to the right on its own row, icon actions sit beside the
module title above it, and both
segments keep equal widths with a minimum 44px touch target. The title and maturity badge may
wrap without overlapping the actions. Review this layout at 720px, 390px and
320px and require no horizontal scrolling.

Built-in Relay Overview owns its live listener and client status. Configuration
contains editable settings only and keeps Discard changes / Save settings aligned
to the right like the other configuration forms.

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
- `formActions` creates Cancel, submit and an inline error. `formFeedback`
  associates field errors, reveals collapsed ancestors, and restores focus and
  visibility after failed actions. Credential rotation retains its specialized
  pending-state controls.
- `renderChanged` is the shared rendering gate, including generic service lists.
  Update current state first; the gate compares displayed data and only records
  successful renders. Unchanged polls retain DOM nodes and selection.

These helpers require no external library, CDN or frontend build. New source
modules should compose them instead of duplicating interaction handlers.


## Ports over SSH views

Ports over SSH uses Overview and Configuration in its module heading, with the
same shared component and selected-segment language as the other modules. Its
content CSS is owned by the SSH module. Overview keeps service creation and the service list;
Configuration uses full-width panels and hides Add service. Narrow headings keep
icon actions beside the title above the segmented control without horizontal scrolling.
Keep this form compact: one path field, one default-path hint, and editor and
save actions. Edit file and host refresh are adornments of the
path field; Discard changes / Save settings remain in a right-aligned footer. Avoid repeated section headings and instructional
paragraphs. Show file status only when the file is missing. Do not attach a
tooltip to the path input: automatic focus must not cover its label. The web
editor shows the resolved saved path.

The SSH configuration path remains a draft until saved. Polling preserves that
draft and conflicts require explicit retry or discard. Edit file opens the saved
file in the shared web editor, locally or remotely. It is disabled while settings
are dirty or an action is pending. Show a concise save/discard hint for drafts.

## Shared module views

Use `moduleViews(ctx, {overview, configuration, onChange})` for every module.
The helper owns navigation and a separate actions slot. Module header switches
share a 260 px width and the same right content edge. Overview actions come
before the switch on desktop and below it on narrow screens. Hide actions in
Configuration. Creation uses an accessible plus icon, with the label Add service.
Local Ports leaves Configuration empty until settings exist. Remote HTTP uses a
compact form for an additional CA bundle and outbound proxy, with authentication
fields in a disclosure. Proxy passwords are write-only and never refilled by polling.
Built-in Relay keeps its Settings placement, showing runtime state in Overview
and its existing form in Configuration. View switches preserve drafts and clear
revealed credentials when leaving configuration.

Remote HTTP shows the platform default `additional-ca.pem` as the CA field's
placeholder. CA actions are Edit file and Reload certificates, grouped
inside that field’s InputAdornment. Editor reasons are visible only when relevant. Editing works locally and remotely, uses the saved path and is blocked by unsaved/pending changes. Reload
refreshes saved certificate validation while preserving proxy/password drafts.


### Configuration file editor

Remote HTTP and Ports over SSH expose one **Edit file** action. Use the shared `fileEditor` dialog: saved path, remote hostname,
labelled monospace textarea and Cancel/Save. The dialog uses available width up
to 960px, keeps text scrollable in narrow/short viewports and inherits theme
colors. Keep drafts through polling, errors and conflicts. A dirty close asks
for explicit discard. Save uses the loaded file version/revision, never the
latest polled revision. Do not preload file contents into module state.


Selecting a source from Services always opens Overview, including selecting the
current source again and returning through browser history. Configuration is an
explicit action; its draft survives navigation and polling. Source modules expose
`openOverview()` so the shell resets only their selected view on navigation.
Settings sections retain their own disclosure behavior.


Local Ports Configuration uses one compact form: Automatic port detection,
Ignored ports and Ignored processes, with Discard changes / Save settings.
Ignore lists remain editable while detection is off. A disabled picker explains
that manual ports still work and does not display stale suggestions. Filters do
not remove indicators from configured services; disabled discovery withholds
unverified indicators. Keep drafts, focus, selection and search through polling.


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

Dashboard displays existing route diagnostics and module-provided listener
observations beneath the target. Cached observations say “Last detected” and
are invalidated after changed discovery settings/routes or a failed lookup.
Dashboard rendering must not start extra health probes.

All configuration forms enable Save settings and Discard changes only when
values differ from saved settings. Reverting an edit returns to the clean state,
including edits made while a save is pending. Discard restores saved values
without requiring a network reload.

Removal confirmation identifies the service, explains that only the mapping is
removed, and ends with a shared Cancel / Remove footer. Close controls never
shrink below 44 by 44 pixels.

Escape is consumed by a tooltip only while that tooltip is visible. Otherwise
it reaches the enclosing dialog or popover. Popover-trigger focus waits until
the native transition completes before considering a tooltip.

### Shared relay catalog and dashboard warnings

A provider card lists source service addresses including that provider's Zelport
port and the Imported / Not imported state. One plus icon in the device header
opens import; multiple services use a labelled service chooser. Service rows
have no creation buttons. Repeated import proposes an unused local domain.
The Dashboard places the shared warning triangle and tooltip in the Actions
cell, immediately before the row menu, for HTTP and disabled TLS verification
warnings. Keep diagnostic details
available to keyboard and pointer users without expanding every table row.
