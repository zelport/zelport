# Local Ports discovery configuration

The user accepted Automatic detection, Ignored ports/ranges and Ignored processes.
Implement these in Local Ports Configuration using existing Go/module UI patterns.
Defaults: detection enabled, both ignore lists empty. These preferences affect
suggestions only; route identities, HTTP forwarding and manual ports remain valid.

Store `discovery_enabled`, `ignored_ports` and `ignored_processes` in module settings.
Persist through the protected revision-aware module settings API. Port entries are
1–65535 or inclusive ascending ranges, e.g. 8000-8099. Bound lists to 128 entries;
process names to 256 UTF-8 bytes, excluding controls. Match exact reported process
names case-insensitively without globs. Retain unknown owners and mixed ports:
only ports whose reported owners are all ignored are excluded by process name.

GET listeners returns filtered suggestions, an enabled flag, and unfiltered
listeners for already configured route ports. Never classify an ignored port as
not listening. Disabled discovery runs no OS scanner, returns empty inventories
and disables only discovery controls; existing routes and manual entry work.

A compact Configuration form has a toggle, two comma-separated text fields and
Discard changes / Save settings. Retain filters while disabled. Polling/navigation
preserve draft nodes, focus and selection; conflicts keep drafts. Saving refreshes
inventory. Open pickers must discard late stale scan results after settings change,
refresh their suggestions and keep manual input/search text.

Use no extra dependencies. Test defaults/persistence, validation, scanner off,
filters, configured status, security/revisions, draft retention, live picker updates
and late requests. Run in sbx and verify the native UI on 7070 with test-only settings.
