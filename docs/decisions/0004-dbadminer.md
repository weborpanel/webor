# Spike M4: Global DBAdminer + multi-engine site DBs

**Status:** accepted (supersedes per-site Adminer mount)

## Context

Sites need optional SQL storage (SQLite, Postgres, or MariaDB) and a safe way
to browse data. [DBAdminer](https://github.com/dbadminer/dbadminer) starts
read-only and supports a multi-database switcher. Parent sites may attach
multiple databases; subdomains inherit credentials but keep their own static
docroots.

## Decision

1. Embed DBAdminer **once** in the panel at `/db/` (session-gated, Host rewrite
   to `127.0.0.1:8787`). There is **no** detach/disable control for Adminer.
2. Always **read-only**. Rebuild the multi-DB catalog whenever parent DBs change
   (one `*sql.DB` per parent database id; `MountAPI` under `/api/databases/{slug}`).
   Slug is the default id when that is the domain slug; otherwise `parentSlug_id`.
3. Parent sites own `databases: [{id, engine}]`. Blank id on create becomes
   `slug(parent)` (e.g. `example.com` → `example_com`). Child sites set
   `parent:` and **cannot** declare databases.
4. **SQLite** add/remove is routine (default id → `data/app.db`; other ids →
   `data/<id>.db`).
5. **Postgres / MariaDB** add/remove require step-up approval bound to
   `{domain, engine, id}`. Package install is **operator/root** (same as Valkey):
   the panel does not open an Install* approval from Add; if the engine is
   missing or inactive, Add returns `install_required` with apt/systemctl
   hints. Helper `InstallPostgres` / `InstallMariaDB` ops remain for CLI use.
6. Usernames are fixed (`sites.Username(parent)` for default id; else
   `username_id`). Passwords are rotatable via `RotateDatabasePassword`.
7. Subdomains are separate sites (`CreateSiteUser` + own `public/`) that share
   parent DB credentials (env/copy from parent list API).

## Consequences

- Per-site `/sites/{name}/db/` mounts are removed; Browse links to `/db/`.
- Legacy singular `database:` migrates to `databases: [{id: slug, engine}]`.
- Valkey/cache: see decision 0005 (shared ACL; no Valkey Adminer).
- Runbook: [`docs/spikes/m4-databases.md`](../spikes/m4-databases.md).
