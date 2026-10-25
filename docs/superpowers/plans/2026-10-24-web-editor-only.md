# Web editor only

User-approved scope: SSH configuration and CA certificates expose one Edit file
adornment. Keep Refresh hosts / Reload certificates. Remove terminal launching,
its endpoints, availability state, icon and instructions. Retain shared file
editor validation, atomic writes, conflicts and remote administration.

Steps: simplify both forms; delete unused terminal backend; adapt existing tests
and current documentation; run full checks and macOS arm64 build; verify both
forms and editors in the integrated browser; deliver binary and refresh 7070.

Baseline: /tmp/zelport-before-web-only.tgz. Historical plans remain unchanged.

Verification: make check passed (208 frontend tests, all Go race tests, vet,
39 release + 3 packaging tests). macOS arm64 build passed, built at
2026-10-24 18:59:12 UTC. Fresh independent review found no actionable issues.
Integrated browser on isolated 7274: SSH and CA Edit file opens the saved path,
saves test-file changes, reports success; dirty settings disable editing;
Discard restores access; Refresh hosts / Reload certificates work. No terminal
action or availability message remains. Screenshots: web-only-ssh.png and
web-only-ca.png in the task artifacts directory.
