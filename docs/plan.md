# Webooza: Development Plan (v2)

*Revised v2, November 2026. For a single senior developer building a production-grade, zero-Docker, systemd-native VPS management panel. The name, binary names and paths below are provisional until the domain and trademark checks are done.*

*Progress note (2026-11-20): Milestone 0–1 lab-complete. Milestone 2 in progress: socket apps, deploy+rollback, idle-stop, cron, panel deploy API + Svelte deploy UI, Node sample (`make lab-node`). Still open in M2: proxyd idle fallback, journal viewer UI, long-running job services, resource monitoring. M1 VPS leftovers: green DNS → HTTPS, live build OOM.*

## 1. Product scope

**Pitch:** a lightweight, non-Docker hosting panel for a VPS owner who wants to run several sites and apps: static sites (with automated build commands), native Node/Go/Rust apps and optional rootless containers, with automatic SSL, per-site isolation and cron. Mail comes later.

**In scope for v1**

- Install script with preflight checks and a first-run wizard
- Domains and subdomains, with DNS left to the user's provider (record checklist plus live checker)
- Automatic SSL through Caddy (Let's Encrypt / ZeroSSL)
- Three workload types: static site (with build step support), native app, container (optional rootless Podman)
- Per-site Linux user, Unix-socket networking, systemd units and timers, with journal size limits (`SystemMaxUse`) and disk quota monitoring
- Long-running tasks and cron per site
- Databases (Postgres or MariaDB) and cache (Valkey) installed on demand
- One admin login with TOTP, plus step-up approval for dangerous operations
- UI and a declarative config file as equal ways to configure

**Out of scope for v1 (planned later):** mail, S3 backups, DNS provider APIs and wildcard certs, multi-user roles, Varnish, PHP, multi-server management.

**Design principle:** the panel being down must never mean the sites are down. Caddy and the site units run independently, so panel restarts and upgrades do not interrupt traffic.

## 2. Project setup and naming

The GitHub organization is already secured. Names below follow the Webooza brand and can be changed in one place if the name changes.

| Item | Name |
| --- | --- |
| Panel daemon (API, UI) | `webooza` |
| Root helper | `webooza-helper` |
| Admin CLI (console approvals, recovery, diagnostics) | `wbz` |
| Config file | `/etc/webooza/config.yaml` |
| Runtime state (SQLite, audit log) | `/var/lib/webooza/` |
| Site data | `/srv/sites/<domain>/` |
| systemd units | `webooza.service`, `webooza-helper.service`, `webooza-caddy.service` |
| Service users | `webooza` (panel), `caddy` (web server), one user per site; the helper runs as root |
| Repositories | `webooza` (monorepo) and `.github` (shared templates and security policy) |

**Setup tasks before the first commit**

1. Register the domain (installer URL and docs site depend on it). — open
2. Reserve the same name on npm, PyPI, crates.io and GHCR, using placeholder packages. — open
3. Choose the license. — **done: Apache-2.0**
4. Add `SECURITY.md` with a private vulnerability reporting route, since the project ships a root helper. — **done**
5. Run a trademark search, paying attention to similar-sounding names such as Wowza. — open
6. Enable branch protection, required CI and enforced 2FA for the organization. — open (CI workflow exists in-repo; org protection still pending)

## 3. Key technical decisions

| Decision | Choice | Reason |
| --- | --- | --- |
| Language | Go | Caddy can be embedded or driven easily, mature systemd and Podman bindings, single static binary |
| Web server and TLS | Caddy, run as a separate managed process | Automatic HTTPS, JSON admin API, live reload without dropping connections |
| Process supervision | systemd | Per-site services, timers, socket activation, cgroup limits, journald logs. No custom supervisor to build |
| Containers | Rootless Podman with Quadlet (optional) | Daemonless, runs per site user, containers become ordinary systemd units |
| State | `config.yaml` (desired state) plus SQLite (runtime state) | Readable, git-friendly config; no database server needed for the panel itself |
| Frontend | SvelteKit static export, embedded in the binary | One-file deployment, small footprint, JSON API shared with the CLI |
| Supported OS | Debian 12 and newer, Ubuntu 22.04 LTS and newer | Limits systemd and cgroup variation |
| Mail engine (later) | Stalwart, driven through its management API | Avoids writing a mail server |
| DNS | None hosted. Record checklist plus live checker | Users keep their provider's DNS manager |

## 4. Architecture

| Component | Runs as | Responsibility |
| --- | --- | --- |
| `webooza` (API and UI) | `webooza`, unprivileged | Authentication, config editing, planning, DNS checker, status views |
| `webooza-helper` | root, systemd unit | Executes a fixed allowlist of operations, verifies step-up approvals |
| `caddy` | `caddy`, with `CAP_NET_BIND_SERVICE` | TLS and routing for every site |
| `wbz` | root, over SSH | Console approvals, recovery, diagnostics |
| Site units | one Linux user per site | Apps, timers, jobs and builds |

**Reconcile loop.** Desired state (`config.yaml`) is compared with actual state, the diff becomes a plan of operations, the helper applies it, and the result is recorded in SQLite and shown in the UI. Every step is idempotent, so the loop can be re-run safely after a crash.

**Providers.** Each managed technology (Caddy, systemd, database, cache, container, and later mail and backup) implements the same interface: render config, plan changes, apply, report status.

**Helper IPC.** A Unix socket where the helper checks the caller with `SO_PEERCRED`. Requests are typed JSON, never shell strings. Every argument is validated (domain syntax, paths confined to `/srv/sites`, numeric limits).

**Request path.** Browser to Caddy, then either static files from the site's `public` directory or a Unix socket owned by the site user.

**Build execution model.** Build commands (for example `npm install && npm run build`) are the riskiest new feature on a small VPS, so they run under strict rules:

- Each build runs as a transient systemd unit under the site's own Linux user, never as root or as the panel user.
- Limits on every build: `MemoryMax`, `CPUQuota`, a timeout and low priority, so a build cannot starve running sites.
- One build at a time across the server, queued, to protect a 1 GB machine.
- Writes are confined to the site's own directory. Network access is allowed in v1 because package installs need it.
- Output is streamed to the UI. A failed build leaves the current release untouched, and a successful one is activated by an atomic symlink swap.

## 5. Repository layout

```
cmd/
  webooza/         API server and embedded UI
  webooza-helper/  root helper
  wbz/             admin CLI
internal/
  config/          schema, validation, migrations
  reconcile/       diff, plan, apply
  providers/       caddy, systemd, database, cache, container
  build/           build runner and queue
  dnscheck/        resolver interface plus fake for tests
  auth/            sessions, TOTP, approvals
  audit/           append-only audit log
  ipc/             helper protocol and validators
web/               frontend source (SvelteKit)
install/           install.sh, uninstall, systemd unit templates
test/
  isolation/       cross-site access tests
  e2e/             installer and site lifecycle tests
docs/
  decisions/       short written decisions for new features
```

## 6. Development environment and testing

**Daily environment:** a local Ubuntu or Debian VM (Multipass, Lima, WSL2 with systemd, or VirtualBox), with a clean snapshot restored before each installer run. Docker is only for quick smoke tests, since it cannot faithfully reproduce users, cgroups and systemd sandboxing.

| Layer | Approach |
| --- | --- |
| Unit tests | Config parsing, argument validators, renderers. Golden-file tests for generated Caddy JSON and systemd units |
| DNS | The checker takes a resolver interface, so tests inject a fake one. Test domains use `/etc/hosts` or a `.test` zone |
| ACME | Pebble (local ACME test server) in development, Let's Encrypt staging on the VPS, production only for release rehearsal |
| Integration and end-to-end | Incus system containers in CI run the real installer and a scripted site lifecycle on Debian and Ubuntu |
| Isolation tests | Automated, run on every change. As site A's user, attempt to read site B's files, connect to B's socket, read B's process environment, open B's database and read B's cache keys. Every attempt must fail |
| Build tests | Builds that exceed memory or time limits are killed without affecting running sites |
| Security | Fuzz the helper's validators, run `govulncheck` and dependency scanning, review the helper allowlist each milestone |
| Release rehearsal | A small 1 GB VPS on at least two providers before each release |

**Deferred to real VPS** (lab VM / NAT cannot fully exercise these; check off when a public IP + domain are available):

- [ ] Let's Encrypt **staging** certificate for a real domain (M0 exit residual)
- [ ] Public DNS checklist end-to-end: A/AAAA at registrar → checker green → Caddy issues cert
- [ ] Authoritative NS + CAA lookups against live nameservers (not only the fake resolver)
- [ ] HTTP-01 / TLS-ALPN challenge path from the public internet (ports 80/443)
- [ ] PTR / forward-confirmed reverse DNS for the installer suggestions (M3)
- [ ] Build OOM kill under real cgroup pressure on a 1 GB machine
- [ ] Idle memory footprint target (~150 MB panel+Caddy+helper) on a 1 GB VPS
- [ ] Cross-provider rehearsal (at least two providers) before each release

## 7. Roadmap

The milestones add up to about 16 weeks of focused work. For one developer, add a 25 to 30 percent buffer for debugging, security review and rework, and plan for roughly 20 to 22 weeks. Each milestone has exit criteria, and the next one does not start until they are met.

### Milestone 0: Foundations and spikes (weeks 1 to 2)

**Status:** complete for lab (staging ACME cert deferred until a real domain/DNS)

**Deliverables**

- [x] Repository, CI and the VM development environment (Debian 13 netinst + shared folder; Docker smoke for Caddy only)
- [x] Config schema v1 with validation, and SQLite migrations
- [x] Helper with IPC and three operations (create site user, write unit file, reload Caddy) — verified on VM
- [x] Spike A: Caddy admin API validate → reload → last-good rollback — decision `docs/decisions/0001-caddy-admin-api.md`
- [x] Spike B: socket activation (Go + Node) + `systemd-socket-proxyd` fallback + warmup — decision `docs/decisions/0002-socket-activation.md`
- [x] Spike C: step-up approval prototype (`wbz approve`, root peercred) — decision `docs/decisions/0003-step-up-approval.md`
- [x] Frontend framework decision (SvelteKit static shell under `web/`)
- [x] Static site from `config.yaml` served by Caddy on the VM (`cmd/spike-m0-static`, lab HTTP / Host header; see `docs/spikes/m0-static-site.md`)

**Exit criteria:** from a config file, one static site is served by Caddy with a staging certificate in the VM, and each spike has a written decision.  
**Lab note:** staging certificate deferred — see §6 “Deferred to real VPS”. Lab verified HTTP `:8080` + `Host` header + `file_server` instead.

### Milestone 1: Core hosting and build pipelines (weeks 3 to 5)

**Status:** lab complete (VPS HTTPS / live OOM still deferred)

**Deliverables**

- [x] Site model: create and delete, per-site Linux user and directories, release layout with atomic `public`/`current` symlinks (Caddy still serves `public/` only)
- [x] Static hosting model: add site → `public/` document root; upload files / optional zip into `public/` (no git for static; git deferred with apps)
- [x] Domain checklist model (A, AAAA, CNAME, CAA warn) — `internal/dnscheck`
- [x] DNS checker: public + authoritative A, CAA via miekg/dns; `GET /api/sites/{name}/dns` + `can_issue_certificate` gating (live HTTPS enablement still VPS-deferred)
- [x] Authentication: setup token, admin account, sessions, TOTP, login rate limiting (`internal/auth`, `/api/auth/*`)
- [x] UI: setup + login + sites (upload + DNS checklist) + per-site dashboard next-steps; embedded in `webooza` via `go:embed` (`make web`)
- [x] Build MemoryMax/timeout wired in `RunBuild` (unit-tested; live cgroup OOM on VPS list)
- [x] File-isolation test scaffold (`test/isolation`, root/linux) — private logs not readable across site users

**Exit criteria:** a user can add a domain, add the records at their DNS provider, watch the checks turn green, and see the site live over HTTPS. A build that runs out of memory fails safely. File-isolation tests pass.  
**Lab note:** HTTPS issuance and live OOM still on §6 Deferred to real VPS.

### Milestone 2: Native apps, socket activation and jobs (weeks 6 to 8)

**Status:** in progress (slice 4: cron timers + run-now + status)

**Deliverables**

- [x] App unit rendering: per-site user service, systemd hardening subset, env file, Unix socket (`internal/providers/systemd`)
- [~] Socket activation with wake-on-request and cooperative idle-stop via `idle_timeout` → `WBZ_IDLE_TIMEOUT` (`make lab-idle`); proxyd/`--exit-idle-time=` fallback for non-cooperative apps still open; always-on when `socket_activation: false`
- [x] Deploys: helper + IPC + panel API + Svelte deploy UI (binary/git on app site detail) with atomic symlinks and probe rollback (`make lab-deploy`)
- [x] Node sample: socket wake + cooperative idle-stop + deploy/rollback (`examples/socket-activation/node-app`, `make lab-node`)
- [~] Logs: helper `JournalTail` for unit journals; full viewer UI + per-unit size limits still open
- [~] Cron: 5-field schedule → `OnCalendar` timers, install/enable, run-now, status/failure flags (`make lab-cron`); long-running job-as-service still open
- [ ] Resource monitoring: CPU, memory and disk usage from cgroups and filesystem checks

**Exit criteria:** sample Node and Go apps deploy, scale to zero and wake on request, roll back after a failed health check, and run a scheduled job. Process-isolation tests pass.

### Milestone 3: Installer and hardening (weeks 9 to 10)

**Deliverables**

- `install.sh`: OS and systemd checks, port 80/443 checks, public IP and PTR detection with forward-confirmed reverse DNS validation
- Prompts: initial domain (PTR as a suggestion), panel subdomain with DNS check, Podman toggle. Non-interactive mode through flags or `install.yaml`. Idempotent re-run and uninstall
- First-run flow: random high port, one-time setup token, temporary self-signed certificate, then move the panel behind Caddy on a real domain and close the extra port
- Service users, capabilities and systemd hardening for the panel, helper and Caddy
- Step-up approvals: TOTP verified by the helper, console approval through `wbz`, single-use and bound to the exact operation, plus the audit log
- Firewall detection with guidance, and no automatic firewall changes without approval

**Exit criteria:** from a clean VM to a first live site in under 10 minutes for someone who has not seen the project, and a completed security review of the helper.

### Milestone 4: Databases and cache (weeks 11 to 12)

**Deliverables**

- On-demand install of Postgres (default) or MariaDB as an approved operation
- One database and role per site, with Unix-socket peer authentication
- Valkey shared instance with per-site ACL users restricted to a key prefix, plus an optional dedicated instance per site
- UI to attach and detach a database or cache and inspect connection details

**Exit criteria:** cross-site database and cache isolation tests pass.

### Milestone 5: Containers, optional module (weeks 13 to 14)

**Deliverables**

- Podman installed on demand, rootless per site user
- Quadlet unit generation, with secure credential storage for registry logins and pulls from Docker/OCI registries
- Container traffic reaching Caddy over a Unix socket or an isolated per-site network. A spike at the start of this milestone settles which
- Volumes under the site directory, resource limits, image updates and rollbacks

**Exit criteria:** a sample container app deploys from a public registry image and passes the isolation tests.

### Milestone 6: Release gate (weeks 15 to 16)

**Deliverables**

- VPS rehearsal on two providers with real Let's Encrypt issuance
- Footprint benchmarks against the targets in section 12
- Upgrade and config-migration tests, including upgrading the panel while sites keep serving
- Documentation: install guide, DNS guide, config reference, troubleshooting
- Threat-model review and a public beta tag

## 8. After v1

| Phase | Scope | Rough size |
| --- | --- | --- |
| 2. Backups | restic to S3, scoped per site, scheduled, restores gated by approval | 3 weeks |
| 3. Mail, receive-only | Stalwart provider, MX/SPF/DKIM/DMARC rows in the checklist, mailbox UI, IMAP | 4 to 6 weeks |
| 4. Mail sending | Outbound relay support, optional direct send with port 25, PTR and blocklist preflight | 3 weeks |
| 5. Provider DNS APIs | Cloudflare and Route 53 tokens, wildcard certs, optional collaborator roles | 4 weeks |

## 9. Config schema sketch

```yaml
version: 1
panel:
  domain: panel.example.com
  port: 0              # 0 = random high port until a domain is set
sites:
  - name: example.com
    aliases: [www.example.com]
    type: static       # static | app | container
    root: public
    build_command: npm install && npm run build   # optional build step
    limits:
      disk_quota: 2G
      log_max: 200M
  - name: api.example.com
    type: app
    exec: ./bin/server
    socket_activation: true
    idle_timeout: 15m
    env_file: .env
    database: postgres
    cache: valkey
    limits:
      memory_max: 256M
    cron:
      - name: nightly-cleanup
        schedule: 0 3 * * *     # translated to a systemd OnCalendar timer
        exec: ./bin/cleanup
```

A `mail:` block is reserved in the schema from the start (per domain), so adding mail later does not break existing config files.

## 10. Helper operation allowlist

| Operation | Class |
| --- | --- |
| Create or remove a site user and directories | Routine (removal with data requires approval) |
| Write or remove a site's unit, socket or timer | Routine |
| Start, stop, restart a site unit | Routine |
| Run a build as the site user with limits | Routine |
| Reload Caddy with a validated config | Routine |
| Read a site's journal | Routine |
| Install or remove packages (Postgres, Podman, Valkey) | Approval |
| Change firewall rules | Approval |
| Change the panel port or domain | Approval |
| Delete a site together with its data | Approval |
| Restore a backup | Approval |

**Approval rules:** single-use, expire in minutes, bound to the exact operation and arguments, and verified by the helper, not the web process. The most destructive operations default to console approval through `wbz`, so a fully compromised panel cannot approve its own requests.

## 11. Security hardening and disk safeguards

- Unprivileged panel daemon. The root helper accepts typed JSON over a Unix socket, with the caller verified by `SO_PEERCRED`
- Strict validation of every helper argument, with fuzz tests
- Unix sockets for apps instead of loopback TCP ports
- `hidepid=2` on `/proc`, and `PrivateTmp`, `ProtectSystem` and `NoNewPrivileges` in site units
- Secrets in 0600 files or systemd credentials, never in world-readable config
- Login rate limiting and TOTP, with root recovery through `wbz`
- Append-only audit log for every privileged operation and approval, with each entry hash-chained to the previous one so tampering is detectable. A root attacker can still alter the files, so a remote log sink is a possible later addition
- **Disk safeguards:** `SystemMaxUse` caps on journald, per-site log and disk limits, periodic cgroup and filesystem checks, and UI warnings before a disk fills

## 12. Success targets for v1

These are targets to validate in Milestone 6, not measured results.

- Panel, Caddy and helper idle memory under roughly 150 MB on a 1 GB VPS
- Adding a static site takes under a minute, excluding DNS propagation
- Clean VM to first live site in under 10 minutes
- Dozens of idle static sites on a 1 GB VPS without noticeable pressure
- A build that exceeds its limits is stopped without affecting running sites
- All isolation tests green on every release
- Panel upgrade or restart causes no dropped requests on running sites

## 13. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Helper compromise gives root | Minimal allowlist, typed operations, fuzzing, audit log, dedicated review in Milestone 3 |
| Bad Caddy config locks out all sites | Validate before reload, keep last-good config, automatic rollback |
| Let's Encrypt rate limits | Issue only after DNS passes, use staging in development, back off on failure |
| Builds exhaust RAM or CPU on a small VPS | Per-build memory, CPU and time limits, global build queue, low priority |
| Logs or build artifacts fill the disk | Journal size caps, per-site limits, monitoring and warnings |
| Distribution differences | Limited supported OS list, CI matrix on Incus |
| Some apps cannot use socket activation | `systemd-socket-proxyd` fallback and an always-on mode |
| Scope creep (mail, DNS, Varnish) | Phase gates, and any new feature needs a short written decision |
| Admin locked out (lost TOTP) | Root recovery through `wbz` |
| Panel upgrade interrupts sites | Sites run independently of the panel, and upgrade tests check for dropped requests |
| Solo-developer schedule slip | 25 to 30 percent buffer, strict exit criteria, mail and containers stay optional |

## 14. Release and operations

- Semantic versioning, with stable and beta channels
- Versioned config migrations that run automatically and keep a backup of the previous config
- Signed release binaries and checksums, and an install script that verifies them
- Uninstall removes the panel and helper but leaves site data and users in place unless the admin approves removal
- No telemetry by default

## 15. Open questions

1. License: permissive (Apache-2.0 or MIT) or AGPL-3.0. — **resolved: Apache-2.0**
2. Run Caddy as a separate process (planned) or embed it later to ship one binary. — separate process for v1 (Spike A)
3. How far git-based builds go in v1: build command only, or language presets. — open
4. Final supported OS list. — open (lab is Debian 13; plan still targets Debian 12+ / Ubuntu 22.04+)
5. Domain, package-name and trademark checks for Webooza. — open
6. Confirm what "PQPM" refers to. — **resolved:** PQPM is a PM2-style user process manager; Webooza uses **systemd** as the site supervisor (PQPM not integrated for v1)
