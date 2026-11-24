# Milestone 4 — databases + per-DB DBAdminer

## What landed

- Parent multi-DB: `databases: [{id, engine}]` on parent sites; blank id →
  domain slug (`example.com` → `example_com`)
- Subdomains: child sites with `parent:` and their own `/srv/sites/<fqdn>/`
  tree; they inherit parent DB credentials (no per-child DB attach)
- Helper: `EnsureSiteDatabase` / `DropSiteDatabase` (id-aware);
  `RotateDatabasePassword`; `InstallPostgres` / `InstallMariaDB` (approval)
- Postgres/MariaDB add/remove consume step-up approvals bound to
  `{domain, engine, id}`; SQLite does not; rotate is immediate after panel auth
- Per-DB DBAdminer: `/db/{slug}/` (read-only) and `/db/{slug}/edit/` (full);
  `/db/` index lists links (decision 0012)
- UI: top app bar; parent Subdomains + Databases (optional id, rotate);
  child inherited-DB panel
- **Valkey (shared):** `InstallValkey` (approval), `EnsureSiteCache` /
  `DropSiteCache` / `RotateCachePassword`; ACL prefix `wbr_<slug>:*`;
  parent Cache UI; child inherit; decision `docs/decisions/0005-valkey-cache.md`

## Lab

```bash
# from host (see docs/dev/lab-ssh-testing.md)
rsync … webooza@127.0.0.1:~/webooza/
ssh.exe -p 2222 -o BatchMode=yes webooza@127.0.0.1 'bash -s' <<'EOF'
cd ~/webooza && make web build && make lab-start
# parent + subdomain + sqlite + postgres (approve Ensure) + rotate
EOF
```

Panel smoke:

1. Onboard parent → Add subdomain → both serve distinct default HTML.
2. Add two DBs on parent (sqlite + postgres with approval); `/db/` index lists
   both with Browse + Edit; RO rejects writes, Edit can mutate.
3. Rotate password; username unchanged; new secret works.
4. Child site page shows inherited DBs only; parent Remove DB does not delete
   child site trees.

## Isolation

Site A peer cannot `CONNECT` to site B’s Postgres DB (`REVOKE … FROM PUBLIC`).
