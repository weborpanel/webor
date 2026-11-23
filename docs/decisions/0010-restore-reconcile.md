# Backup restore reconciles live routing

**Status:** accepted

## Context

Selective restore copied site files and merged `config.yaml` (including `https: true`) but left Caddy on whatever lab/listen config was already loaded. Result: DNS fine, nothing on 80/443 until HTTPS was toggled by hand.

## Decision

After a successful `BackupRestore`, the panel **must** reconcile live state from the saved config:

1. Ensure managed Node runtimes for restored app/backends when needed.
2. Re-ensure HTTP backend units.
3. Re-apply app units when any `type: app` sites exist.
4. Re-render and reload Caddy with the same listen/TLS rules as normal panel applies (`https` site flags → `:80`/`:443` unless the panel forced lab HTTP).

Restore still returns success if files landed; Caddy/unit failures surface as a `warning` on the API response.

## Consequences

- `POST /api/backup/restore` calls `reconcileAfterBackupRestore` after the helper finishes.
- Helper restore also ensures Node before backend units so units can start.