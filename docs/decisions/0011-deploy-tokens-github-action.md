# Deploy tokens + official GitHub Action

**Status:** accepted

## Context

CI builds Linux artifacts (decision 0007), but operators still hand-copy them onto the VPS. Cookie sessions cannot be used from GitHub Actions. Deploy-from-URL (0008) remains a separate pull path.

## Decision

1. **Site-scoped deploy tokens** (`wbr_dt_…`), stored hashed in SQLite, with scopes `files` | `deploy` | `runtime` | `api`. Bound to one site. Cannot create/delete sites or mint tokens.
2. **Bearer auth** on site deploy/FS/runtime routes alongside admin sessions. Admin-only routes stay session-only.
3. **CI probe** `GET /api/sites/{name}/ci` for Action preflight (type, runtime, backends, suggested kind). Missing site → 404; never creates.
4. **App deploy-zip** `POST /api/sites/{name}/deploy-zip` for Node bundles into the app site root.
5. **Official Action** [`weborpanel/deploy-action@v1`](https://github.com/weborpanel/deploy-action) with explicit `kind`: `static` | `node` | `binary` | `api`. Binary path gets `chmod +x`; panel install already forces `0755`.

## Consequences

- Panel Site → **CI** mints tokens and shows a workflow snippet.
- Kind mismatch or missing site fails the job.
- Complements 0007/0008; does not replace Deploy-from-URL.
