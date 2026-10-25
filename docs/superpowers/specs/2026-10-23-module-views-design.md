# Consistent module views

All five modules expose Overview and Configuration. Local Ports and Remote HTTP currently have empty Configuration views. SSH and Ports over Relay preserve their existing configuration and drafts. Built-in Relay keeps its settings placement, exposes runtime status in Overview and its existing form in Configuration.

One shared UI component owns the two labels, equal segment widths, active state, action slot and view visibility. The switch is always the last control and aligned to the right content edge, independent of refresh or creation actions. Icon actions occupy a separate slot before the switch on desktop and below it on narrow screens. Configuration hides Overview actions. Replace the module toolbar Add service text button with an accessible plus icon. Keep empty-state calls to action and the Dashboard unchanged.

Preserve module API boundaries, draft nodes, polling, lifecycle, local-only editor checks and secret cleanup. No backend changes. Use English UI copy and existing icon primitives.
