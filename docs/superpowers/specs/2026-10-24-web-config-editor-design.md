# Simple Web Configuration Editor

## Intent and chosen approach

Add a small integrated text editor for the saved Remote HTTP CA certificate
file and Ports over SSH configuration file. It works in local administration
and authenticated remote relay administration. The user selected this approach
over streaming a remote TUI. Existing local terminal editing stays available.
No filesystem mount, file browser, terminal emulation or new dependency is needed.

## User interaction

- Add **Edit in browser** beside **Edit in terminal** in both Configuration views.
- Open a shared modal with the saved file path, target hostname when remote,
  a labelled monospace multiline text field, **Cancel** and **Save**.
- Keep the interface compact. The editor fills the available dialog width and
  remains usable in narrow and short viewports and both themes.
- Block opening while the settings form has unsaved changes. Editing always
  refers to the saved path on the administered instance.
- Opening an absent file displays an empty draft and creates nothing. Only
  successful Save creates the file and any missing parent directories.
- Save shows validation/write errors without losing the draft. A concurrent
  file or path change refuses overwrite and keeps the draft for copying or
  explicit reload. There is no automatic overwrite retry.
- Closing a modified draft requires an explicit discard choice. Polling cannot
  replace text, selection, focus or scroll. Ctrl/Cmd+S saves inside the editor.
- After saving, refresh CA status or SSH host discovery. If discovery fails,
  distinguish that failure from a successfully saved file.

## Module and API boundaries

Use one shared frontend dialog and a small shared Go file-editing helper.
Modules own their endpoints, saved-path resolution, validation and post-save
resource handling. Proposed routes are `/api/modules/remote_http/ca-file` and
`/api/modules/ports_over_ssh/config-file`, each supporting GET and PUT.

The read response contains the effective saved path, content and an opaque
version bound to the settings/path and current file state. PUT accepts content,
that version and the administration revision; it does not accept a target path.
Check the saved path and version again while serializing settings/file edits.
Reject stale revisions, changed files, replaced targets and changed paths with
409. Existing authority, authenticated remote context, Origin, CSRF and relay
revocation checks remain in force. Responses use `Cache-Control: no-store`.
File content is fetched only when requested, never included in polling state,
logs, browser storage or the read-only integration API.

## File behavior and validation

Bound content to 1 MiB and require UTF-8 text without NUL. Reject directories,
devices and dangling symlinks. For an existing regular symlink target, retain
the link and bind the edit to the resolved target; reject target changes.
Preserve existing permission bits. New files use owner-only permissions and
new directories use owner-only access where the OS supports it.

Write through a temporary sibling and platform-appropriate atomic replacement;
check for external changes immediately before replacement. A validation or
pre-replacement write failure leaves the original file intact. No backup file
or empty file is created just by opening the editor. External programs that do
not participate in locking can still race after the final comparison; do not
claim a cross-process transactional filesystem guarantee.

CA reads allow malformed content so it can be repaired. CA saves require the
existing strict certificate-only PEM validation, including private-key and
empty-bundle rejection. A missing default continues to use system trust until
a valid file is explicitly saved. SSH accepts bounded text without executing
it for validation; the system SSH client retains responsibility for syntax and
runtime semantics. Reload host discovery and retire stale SSH pools after save.
CA trust must use the saved bytes on subsequent connections with no stale cache.

## Verification

Use temporary files and synthetic public configuration only. Cover saved-path
selection, local and remote administration, CSRF/Origin rejection, revisions,
external file/target changes, missing files, symlinks, limits, permissions,
validation and failed-write preservation. Test dialog drafts, closing, keyboard
save, pending requests, polling and errors. Run repository checks in sbx and
verify both modules in the integrated browser on port 7070, including narrow
and short viewports. Do not modify real SSH configuration or installed trust.

## Implementation choices

Native textarea, Go standard library and existing UI primitives are sufficient.
No highlighting, arbitrary-file navigation, Include-file editing, shell access,
autosave or general remote filesystem API is included.
