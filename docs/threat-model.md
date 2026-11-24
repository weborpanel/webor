# Threat model (Milestone 6)

Date: 2026-11-24. Scope: public beta of the native VPS panel (`webor`, `webor-helper`, `wbr`, Caddy). Complements the lab review in [spikes/m3-helper-review.md](spikes/m3-helper-review.md) and [SECURITY.md](../SECURITY.md).

## Assets

| Asset | Why it matters |
| --- | --- |
| Site content under `/srv/sites/<domain>/` | Customer data and code |
| Secrets under `state/secrets/` and DB/cache passwords | Credential theft → lateral move into apps |
| Helper Unix socket + allowlisted ops | Root-equivalent host control |
| Panel session / TOTP | Admin takeover of all sites |
| Caddy TLS material and routes | Traffic interception or site spoofing |
| Hash-chained audit log | Detection of privileged misuse |
| Backup repository (restic + S3) | Full host restore / exfiltration if password leaks |

## Trust boundaries

```
[Browser / CI deploy token]
        │  HTTPS (or high-port HTTP until panel domain)
        ▼
[webor panel — unprivileged]
        │  Unix socket + SO_PEERCRED
        ▼
[webor-helper — root, typed IPC only]
        │
        ├── systemd (site / cron / process / API units)
        ├── Caddy (admin API / reload)
        ├── package managers (Postgres, MariaDB, Valkey installs)
        └── restic (backup / restore)
```

`wbr` as root is the break-glass path for approvals and recovery. It must not be reachable from the panel process.

## Adversaries

1. **Remote unauthenticated attacker** — internet to panel port or site frontends.
2. **Stolen panel session** — cookie after phishing / XSS / weak password without TOTP.
3. **Compromised panel process** — RCE as the `webor` user (bug or malicious deploy UI path).
4. **Malicious site workload** — code running as the site Linux user (app, cron, API backend).
5. **Local root / host admin** — out of scope to fully stop; goal is detectability (audit) and not making compromise silent.
6. **Backup bucket attacker** — S3 credentials or world-readable objects without restic password.

## Controls (current)

| Control | Status |
| --- | --- |
| Unprivileged panel; root only in helper | Done |
| Typed IPC ops; no shell strings; argument validation | Done |
| `SO_PEERCRED` on helper socket | Done |
| Console-only `Approve` / `ListApprovals` (uid 0) | Done |
| Approval-gated: delete-with-data, Postgres/MariaDB ensure/drop, package installs, backup restore/forget, DangerousDemo | Done |
| Single-use, TTL, args-bound approvals | Done |
| Per-site Linux user + systemd hardening (`NoNewPrivileges`, `ProtectSystem`, cgroup memory/CPU, disk quota) | Done |
| Apps prefer Unix sockets / injected `PORT` over ad-hoc host ports | Done |
| Login rate limit + optional TOTP; `wbr recover` for lockout | Done |
| Hash-chained `audit_events` | Done |
| Secrets not in `config.yaml` | Done |

## Abuse cases and residual risk

| Scenario | Expected outcome | Residual risk |
| --- | --- | --- |
| Brute-force panel login | Rate limited; TOTP if enrolled | Operators who skip TOTP |
| XSS in panel UI | Session theft possible | Keep CSP/deps tight; TOTP limits password-only reuse |
| Panel RCE as `webor` | Can call routine helper ops (deploy, limits, sqlite DB, reload Caddy) | **Cannot** approve its own destructive ops; can still wipe/replace site content for sites it manages |
| Site user breakout | Contained by user + unit sandbox | Kernel / container-escape class bugs; Podman path not in beta |
| Delete site + data without console | Blocked pending `wbr approve` | Social engineering of the operator at the console |
| Restore backup / forget snapshots | Approval required | Stolen restic password + S3 access restores elsewhere |
| Install Postgres via helper | Approval required | After approve, packaging trust is distro + network |
| Change site memory/CPU/disk | Routine (no approval) — intentional for Settings UX | Compromised panel can OOM-kill site builds/units within presets |
| Shared SQLite between panel and helper | Convenient for v1 | Panel compromise can rewrite non-approval state; split DBs later |
| Firewall / panel port changes | Not fully approval-class yet | Operator should lock high port after panel domain is live |
| Helper TOTP step-up (in-panel) | Not implemented | Console approve remains the only step-up |

## Explicit non-goals for this beta

- Stopping a malicious **root** user on the VPS
- Multi-tenant untrusted panel users (single admin model)
- Formal verification or full fuzz coverage of every IPC arg path (validation + tests exist; ongoing)
- Podman / Quadlet isolation (deferred M5b)

## Beta acceptance

Ship public beta when:

1. Destructive ops that can destroy host data or install packages still require console approval (verified in [vps-checklist.md](vps-checklist.md)).
2. Compromised panel cannot call `Approve` (peercred uid 0 only).
3. Private reporting path exists ([SECURITY.md](../SECURITY.md)).
4. Operators have install, DNS/HTTPS, config, and troubleshooting docs.

## Follow-ups (post-beta)

- Split helper/panel databases
- Optional in-panel TOTP step-up alongside `wbr approve`
- Approval class for firewall / panel listen changes when automated
- Remote audit sink
- Expand fuzzing on IPC validators
