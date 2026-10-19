# Zelport UI design system

This document defines shared presentation and interaction rules. Module-specific
layouts compose these components; they must not redefine global component styles.
There is no frontend build, external font, library runtime or CDN dependency.

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

Coarse-pointer controls have at least 44px targets. Main text never shrinks on
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
- Do not truncate a domain or address without an accessible way to read it.
- Empty states use the same surface language, not a different dashed container.

## Panels and controls

Resource and Settings accordions share title typography, spacing and panel radius.
Expanded Settings start closed on each entry to Settings. Resource expansion may
survive data updates. Nested service lists do not add another outer border.

Primary buttons submit the main action. Secondary buttons edit, configure or
cancel. Remove is quiet in a row; destructive confirmation is explicit. No action
is triggered by hovering. Switches toggle a setting; checkboxes select or confirm.
Input adornments support prefix/suffix text and keyboard-accessible actions such
as reveal/copy. Field labels remain visible. Inputs use body-sized text.

A compact row can grow for touch targets; fixed-height rows must not clip content.
Focus indication is visible for keyboard navigation. Active text fields may use a
focus-within border. Never remove focus feedback without an equivalent indicator.

The Dashboard's **Zelport runtime** panel keeps Version, Hostname, Configuration,
Listeners and Modules in one row. Modules uses a compact count button whose
non-modal popover lists the included module names and maturity. Runtime facts
become a two-column grid at intermediate widths and one column on mobile; the
Modules field spans both intermediate columns. Build time and uptime are not
shown in this panel. Configuration paths and listener values wrap or scroll
within their own cells without widening the page.

## Layers, feedback and lifecycle

Tooltips contain short supplementary text only. Popovers contain short contextual
content/actions and remain anchored to their trigger. Dialogs contain editing or
confirmation tasks; support Escape, focus return and scrolling on small screens.
Long diagnostics expand in document flow and never overlap neighboring sections.

Errors belong to the affected view or form. Pending controls prevent duplicate
submission. Refresh stays in the same navigation position across tabs, disabled
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

The shell exposes Dashboard, Services and Settings at every width. Services is a
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
- `formActions` creates Cancel, submit, an inline error and an optional draft
  notice. Credential rotation retains its specialized pending-state controls.
- `renderChanged` is the shared rendering gate, including generic service lists.
  Update current state first; the gate compares displayed data and only records
  successful renders. Unchanged polls retain DOM nodes and selection.

These helpers require no external library, CDN or frontend build. New source
modules should compose them instead of duplicating interaction handlers.
