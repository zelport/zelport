# Ports over SSH configuration

The user requests Overview/Configuration views like Ports over Relay, a
configurable SSH configuration file path, and an action opening that file in the
default editor in a terminal. Retain full-width panels and the existing UI style.

## Approach

Use the existing module-owned frontend, settings commits and system OpenSSH.
An embedded web editor would not satisfy the terminal-editor request. A route-
specific path would duplicate one device-level setting across every service.
The selected file therefore belongs to the Ports over SSH module.

- Overview keeps the SSH service list and Add service action.
- Configuration has SSH configuration file, its effective saved path/existence,
  Save settings, Discard changes and Edit in terminal.
- Blank means ~/.ssh/config. Accept absolute paths and ~/ paths; reject relative
  paths, `%` (inconsistent across OpenSSH versions), NUL/newlines and existing directories/nonregular files. A missing file
  is permitted, and editing creates it with owner-only permissions.
- The saved path controls both host discovery and connection configuration.
  Preserve hardened wrapper options for ProxyJump children. Never delete the
  user's config when removing a generated SSH wrapper.
- Module settings use ssh_config_path. State data includes ssh_config_path,
  effective_config_path, config_exists, editor_available, editor_error and
  connections. No file contents are sent to the browser.
- PUT /settings uses revision and the usual atomic module commit. Changed paths
  retire pools so later connections use the new file. POST /edit-config accepts
  revision and opens only the saved path. Reject stale revisions.
- Editor launch is allowed only from the local localhost administration authority;
  remote administration displays a reason and cannot trigger a local process.
- Resolve VISUAL/EDITOR inside the user's terminal shell, falling back to vi on
  Unix. Keep platform launchers behind OS-specific files. Windows launches a
  terminal and uses the configured editor or an available console editor; report
  absence clearly. Paths must be passed/quoted without command injection.
- Opening an editor does not claim the file was saved. Offer Refresh hosts after
  saving externally. Preserve drafts on polling and surface launch/save errors.

## Verification

Tests use temporary SSH files and injected process launchers, never the user's
SSH file. Cover custom discovery/SSH wrapper, persistence, stale revisions,
missing files, path injection, pending/draft frontend behavior and remote launch
rejection. Run full repository checks inside sbx. Verify desktop/narrow UI on
7070 after a safe native preview build; editor smoke tests use a temporary file.
