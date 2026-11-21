# Milestone 3 — helper security review (lab)

Date: 2026-11-20. Scope: `webor-helper` IPC allowlist, approvals, unit hardening.

## Findings fixed during smoke

| Issue | Risk | Fix |
| --- | --- | --- |
| `ProtectSystem=full` on helper | `/etc` read-only → `useradd` fails | `ProtectSystem=true` |
| Helper `UMask=0077` | `public/index.html` mode 0600 → Caddy 403 | Dropped UMask; explicit `chmod 0644` after write |
| `/etc/webor` mode 750 | Caddy cannot traverse to `caddy.json` | Dir mode `0751`; `caddy.json` `0644` |
| `/srv/sites` mode 750 | Caddy cannot traverse site roots | Install uses `0755` |
| `caddy run --adapter json` | Caddy 2.x rejects adapter | Native JSON, no adapter |
| Shared `state.db` owned by root | Panel cannot open SQLite | Root `OpenDB` chowns to `webor` |

## Allowlist surface (reviewed)

Typed ops only (no shell strings): site user create/delete, public file write, static/app deploy, build, unit write, Caddy reload, cron/journal/resources, approvals, `DangerousDemo`.

- `Approve` / `ListApprovals`: root peercred (`uid==0`) only
- `DeleteSiteUser` + `delete_data`: requires consumed console approval bound to canonical args
- Audit: hash-chained `audit_events` on request / approve / delete-with-data

## Residual / follow-ups

- Panel still opens the same SQLite as the helper (shared state). Acceptable for v1; consider split DBs later
- Helper TOTP step-up not implemented (console `wbr approve` is the break-glass path per decision 0003)
- Package install / firewall changes not yet approval-class ops (M4+)
- Formal threat-model pass still owed at Milestone 6

## Lab exit check

Timed install ~2–3s (binaries prebuilt); first-run → static site → Caddy `:8080` → delete-with-data approval path verified on Debian 13 VM.
