# Per-DB DBAdminer paths (RO + Edit)

**Status:** accepted (supersedes the “one global always-read-only `/db/`” mount in [0004](0004-dbadminer.md))

## Context

Decision 0004 embedded a single multi-DB, always-read-only DBAdminer at `/db/`.
Operators need a direct link per database and, when inspecting or fixing data, a
full-access explorer. [DBAdminer](https://github.com/dbadminer/dbadminer)
already supports `AccessReadOnly` and `AccessFull` on a single-DB handler.

## Decision

1. Mount **one explorer per parent database and access mode**, session-gated:
   - Browse (RO): `/db/{slug}/`
   - Edit (full): `/db/{slug}/edit/`
2. `/db/` is a session-gated **index** of those links (not the multi-DB switcher).
3. Slug remains `AdminerSlug` (default id → id; else `parentSlug_id`). The path
   segment `edit` is reserved under a slug.
4. **Browse** uses the RO role (Postgres/MariaDB secret user) or SQLite `mode=ro`.
   **Edit** reuses the shared password from the same `.ro.json` as **AppRole**, or
   opens SQLite without `mode=ro`. No separate RW secret files.
5. Panel session auth gates both modes. No step-up approval for Edit in v1.
   There is still no detach/disable control for Adminer.
6. Site Databases UI exposes per-row **Browse** and **Edit** links
   (`browse_path` / `edit_path` on each database object).

Items 3–7 of decision 0004 (multi-engine ownership, approvals, naming, children)
are unchanged.

## Consequences

- Global multi-DB switcher at `/db/` is removed.
- Stolen panel session can write through Edit explorers (same trust as other
  authenticated panel actions that mutate site data).
- Runbook: [`docs/spikes/m4-databases.md`](../spikes/m4-databases.md).
