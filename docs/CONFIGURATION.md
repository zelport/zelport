# Configuration

Use the administration UI to create and edit routes. The default file is
`$XDG_CONFIG_HOME/zelport/config.json` or `~/.config/zelport/config.json` on
macOS and Linux, and the native per-user configuration directory on Windows.
Pass `--config` to select another path.

The schema is exactly version `1`. A missing file is created as a valid empty
configuration. Other versions, unknown fields and invalid module settings are
rejected; Zelport does not import or rewrite earlier schemas.

```json
{
  "version": 1,
  "revision": 1,
  "modules": {},
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

Writes are atomic and owner-only. Route UUIDs survive edits, and concurrent
changes are rejected instead of silently overwriting one another.
