# Navigation Rhythm Design

## Goal

Make the administration console feel calmer and more spacious while giving the
global and module navigation visibly different roles. Every visible navigation
label must be unique within the current hierarchy.

The change stays inside the existing embedded HTML, CSS, and JavaScript system.
It adds no frontend build step, runtime dependency, or external asset.

## Navigation vocabulary

The global navigation remains:

1. `Dashboard`
2. `Services`
3. `Settings`

`Services` remains a disclosure rather than a destination. It contains the
enabled service source modules such as Local Ports, Remote HTTP, Ports over SSH,
and Ports over Relay.

Ports over Relay exposes these local views:

1. `Overview`
2. `Configuration`

`Overview` replaces the duplicate local `Services` label. The existing content
headings such as Imported services and Shared services remain unchanged. Internal
module identifiers, URLs, hashes, API routes, and persisted configuration do not
change.

## Visual hierarchy

The global navigation stays below the Zelport identity and uses the existing
underlined active item. Its items are left aligned at every supported width.
They use a consistent gap rather than filling the available row. With only three
global destinations, a hamburger menu would hide useful orientation and add an
unnecessary interaction.

The Ports over Relay view navigation becomes a segmented control in the module
heading. This deliberately removes the second underlined navigation row. The
module title and maturity badge sit on the left. The segmented `Overview` and
`Configuration` control sits on the right with module actions such as Refresh.

The segmented control uses the existing surfaces, border, accent, spacing, and
focus tokens. Both options have equal width. The selected option uses a solid
surface and subtle shadow rather than another underline.

Vertical spacing follows the current token scale. The module heading receives
more separation from both the global navigation and the first content heading,
creating an airy admin-console rhythm without increasing every component.

## Responsive behavior

Wide content keeps the title and segmented control on one row.

When the available content width reaches the narrow layout threshold, the module
heading stacks into two rows. The title and maturity badge occupy the first row.
The module actions occupy the second row. The segmented control grows to the
available width and keeps equal columns. Any refresh action remains a trailing
icon control and is shown only in Overview as it is today.

At 390 px and 320 px:

- the global navigation remains visible and left aligned
- navigation labels never use `space-between`
- the module title and maturity badge may wrap without overlapping
- the segmented control never causes horizontal scrolling
- interactive controls provide at least a 44 px touch target

The responsive rule depends on available layout width, not device identity. No
dropdown replaces the two local views because both choices fit and should remain
discoverable.

## Component ownership

The shell continues to own the module heading and its action container. Ports
over Relay mounts its local navigation into the existing module toolbar instead
of rendering it as the first row inside the module panel. Its Overview and
Configuration content remains owned by the module.

This keeps the change narrow. It does not introduce a new general tab framework
before another module needs the same pattern. The styles remain module scoped,
while shared heading and responsive layout rules stay in the shell stylesheet.

When the module is disposed, it removes both its segmented navigation and refresh
control. Entering Configuration continues to hide Refresh. Returning to Overview
continues to clear configuration secrets through the existing lifecycle.

## Interaction and accessibility

The segmented control remains a navigation region labelled `Ports over Relay
views`. Its buttons use `aria-current="page"`, retain ordinary Tab navigation,
and expose visible keyboard focus. The selected appearance is not the only source
of state because `aria-current` remains authoritative.

Moving the navigation does not change view state, polling, draft preservation,
secret clearing, module activation, browser history, or remote administration.
The Services disclosure keeps its current keyboard behavior including arrow keys,
Home, End, Escape, outside dismissal, and focus return.

## Verification

Automated DOM checks cover:

- the global labels Dashboard, Services, and Settings
- the local labels Overview and Configuration
- absence of the duplicate local Services label
- placement of the local navigation in the module heading actions
- equal view state and `aria-current` behavior
- Refresh visibility in Overview and Configuration
- module disposal cleanup

Visual review covers 1280 px, 720 px, 390 px, and 320 px in light and dark themes.
It checks left alignment, wrapping, touch targets, focus, long maturity labels, and
the absence of horizontal scrolling. Existing application tests must remain green.
