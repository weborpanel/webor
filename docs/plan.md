# Webor: Development Plan (v2)

*Revised v2, November 2026. For a single senior developer building a production-grade, zero-Docker, systemd-native VPS management panel. Brand is Webor (`webor.dev`); binary and path names below are locked.*

*Progress note (2026-11-26): Milestone 0–5 lab-complete (including restic S3 backups, decision 0006). Live HTTPS verified on Debian and Alma across two VPS providers and EC2. Footprint / OOM / upgrade-while-serving checked. Operator docs + threat model written; public beta on the release gate (bootstrap defaults to beta until stable `v0.1.0`). Podman remains deferred (M5b).*

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
- Whole-system S3-compatible backups (restic): full snapshot, selective site restore after install (decision 0006)

**Out of scope for v1 (planned later):** mail, DNS provider APIs and wildcard certs, multi-user roles, Varnish, PHP, multi-server management, Podman containers.

**Design principle:** the panel being down must never mean the sites are down. Caddy and the site units run independently, so panel restarts and upgrades do not interrupt traffic.

## 2. Project setup and naming

The GitHub organization is already secured. Names below follow the Webor brand and can be changed in one place if the name changes.

| Item | Name |
| --- | --- |
| Panel daemon (API, UI) | `webor` |
| Root helper | `webor-helper` |
| Admin CLI (console approvals, recovery, diagnostics) | `wbr` |
| Config file | `/etc/webor/config.yaml` |
| Runtime state (SQLite, audit log) | `/var/lib/webor/` |
| Site data | `/srv/sites/<domain>/` |
| systemd units | `webor.service`, `webor-helper.service`, `webor-caddy.service` |
| Service users | `webor` (panel), `caddy` (web server), one user per site; the helper runs as root |
| Repositories | `weborpanel/webor` (monorepo) and `.github` (shared templates and security policy) |

**Setup tasks before the first commit**

1. Register the domain (installer URL and docs site depend on it). — **done: webor.dev**
2. Reserve the same name on npm, PyPI, crates.io and GHCR, using placeholder packages. — open
3. Choose the license. — **done: Apache-2.0**
4. Add `SECURITY.md` with a private vulnerability reporting route, since the project ships a root helper. — **done** (`security@webor.dev`)
5. Run a trademark search, paying attention to similar-sounding names such as Wowza. — **done** (name locked)
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
| Supported OS | Debian 12+, Ubuntu 22.04 LTS+, AlmaLinux 9 | Limits systemd and cgroup variation; Alma uses dnf + SELinux Enforcing |
| Mail engine (later) | Stalwart, driven through its management API | Avoids writing a mail server |
| DNS | None hosted. Record checklist plus live checker | Users keep their provider's DNS manager |

## 4. Architecture

| Component | Runs as | Responsibility |
| --- | --- | --- |
| `webor` (API and UI) | `webor`, unprivileged | Authentication, config editing, planning, DNS checker, status views |
| `webor-helper` | root, systemd unit | Executes a fixed allowlist of operations, verifies step-up approvals |
| `caddy` | `caddy`, with `CAP_NET_BIND_SERVICE` | TLS and routing for every site |
| `wbr` | root, over SSH | Console approvals, recovery, diagnostics |
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
  webor/         API server and embedded UI
  webor-helper/  root helper
  wbr/             admin CLI
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

- [ ] Let's Encrypt **staging** certificate for a real domain (M0 exit residual; production issuance already proven)
- [x] Public DNS checklist end-to-end: A/AAAA at registrar → checker green → Caddy issues cert (Debian + Alma; two VPS providers + EC2)
- [ ] Authoritative NS + CAA lookups against live nameservers (not only the fake resolver)
- [x] HTTP-01 / TLS-ALPN challenge path from the public internet (ports 80/443)
- [x] PTR / forward-confirmed reverse DNS for the installer suggestions (M3)
- [x] Build OOM kill under real cgroup pressure on a 1 GB machine (2026-11-23 Debian t3.micro; `oom-kill`, other site still 200)
- [x] Idle memory footprint target (~150 MB panel+Caddy+helper) on a 1 GB VPS — measured **~65 MB** RSS on Debian t3.micro after install+setup
- [x] Cross-provider rehearsal (at least two providers) before each release — first pass done (two VPS providers + EC2, Debian and Alma); repeat before each release

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
- [x] Spike C: step-up approval prototype (`wbr approve`, root peercred) — decision `docs/decisions/0003-step-up-approval.md`
- [x] Frontend framework decision (SvelteKit static shell under `web/`)
- [x] Static site from `config.yaml` served by Caddy on the VM (`cmd/spike-m0-static`, lab HTTP / Host header; see `docs/spikes/m0-static-site.md`)

**Exit criteria:** from a config file, one static site is served by Caddy with a staging certificate in the VM, and each spike has a written decision.  
**Lab note:** staging certificate deferred — see §6 “Deferred to real VPS”. Lab verified HTTP `:8080` + `Host` header + `file_server` instead.

### Milestone 1: Core hosting and build pipelines (weeks 3 to 5)

**Status:** lab complete; live HTTPS verified on real hosts

**Deliverables**

- [x] Site model: create and delete, per-site Linux user and directories, release layout with atomic `public`/`current` symlinks (Caddy still serves `public/` only)
- [x] Static hosting model: add site → `public/` document root; upload files / optional zip into `public/` (no git for static; git deferred with apps)
- [x] Domain checklist model (A, AAAA, CNAME, CAA warn) — `internal/dnscheck`
- [x] DNS checker: public + authoritative A, CAA via miekg/dns; `GET /api/sites/{name}/dns` + `can_issue_certificate` gating
- [x] Authentication: setup token, admin account, sessions, TOTP, login rate limiting (`internal/auth`, `/api/auth/*`)
- [x] UI: setup + login + sites (upload + DNS checklist) + per-site dashboard next-steps; embedded in `webor` via `go:embed` (`make web`)
- [x] Build MemoryMax/timeout wired in `RunBuild` (unit-tested; live cgroup OOM on VPS list)
- [x] File-isolation test scaffold (`test/isolation`, root/linux) — private logs not readable across site users

**Exit criteria:** a user can add a domain, add the records at their DNS provider, watch the checks turn green, and see the site live over HTTPS. A build that runs out of memory fails safely. File-isolation tests pass.  
**VPS note:** live HTTPS verified on Debian and Alma across two VPS providers and EC2. Live OOM under 1 GB cgroup pressure still on §6 Deferred to real VPS.

### Milestone 2: Native apps, socket activation and jobs (weeks 6 to 8)

**Status:** lab-complete

**Deliverables**

- [x] App unit rendering: per-site user service, systemd hardening subset, env file, Unix socket (`internal/providers/systemd`)
- [x] Socket activation with wake-on-request; cooperative idle-stop (`WBR_IDLE_TIMEOUT`, `make lab-idle`); proxyd/`--exit-idle-time=` fallback (`socket_proxy`, `make lab-proxyd`); always-on when `socket_activation: false`
- [x] Deploys: helper + IPC + panel API + Svelte deploy UI (binary/git on app site detail) with atomic symlinks and probe rollback (`make lab-deploy`)
- [x] Node sample: socket wake + cooperative idle-stop + deploy/rollback (`examples/socket-activation/node-app`, `make lab-node`)
- [x] Logs: helper `JournalTail` + `GET /api/sites/{name}/logs` + site detail Logs panel; `limits.log_max` rate-limit hints (global `SystemMaxUse` → M3)
- [x] Cron: 5-field schedule → `OnCalendar` timers (`make lab-cron`); long-running `jobs:` services (`make lab-job`)
- [x] Resource monitoring: disk + memory (+ optional CPU) via `GET /api/sites/{name}/resources` + Resources strip

**Exit criteria:** sample Node and Go apps deploy, scale to zero and wake on request, roll back after a failed health check, and run a scheduled job. Process-isolation tests pass. — **met in lab**

### Milestone 3: Installer and hardening (weeks 9 to 10)

**Deliverables**

- [x] `install.sh`: OS and systemd checks, port 80/443 checks, public IP and PTR detection with forward-confirmed reverse DNS validation (`install/install.sh`; spike `docs/spikes/m3-install.md`)
- [x] Non-interactive mode through flags or `install.yaml`; idempotent re-run and uninstall. Domain/Podman prompts: flags present (`--panel-domain`, reserved podman key); interactive wizard polish deferred
- [x] First-run flow: random high port, one-time setup token (`-setup-token`), optional self-signed TLS (`-tls-cert`/`-tls-key`); move behind Caddy documented in spike
- [x] Service users, capabilities and systemd hardening for the panel, helper and Caddy (`install/*.service`)
- [x] Step-up approvals: console approval through `wbr` (root peercred), single-use and bound to the exact operation, hash-chained audit log; `DeleteSiteUser`+`delete_data` wired. Helper TOTP verify still optional alongside console (decision 0003)
- [x] Firewall detection with guidance, and no automatic firewall changes (`install.sh` + `wbr doctor`)

**Exit criteria:** from a clean VM to a first live site in under 10 minutes for someone who has not seen the project, and a completed security review of the helper. **Lab-met (2026-11-20):** install→setup→static site on `:8080` under 10 minutes with prebuilt bins; helper review `docs/spikes/m3-helper-review.md`.

### Milestone 4: Databases and cache (weeks 11 to 12)

**Deliverables**

- [x] On-demand install of Postgres (default) or MariaDB as an approved operation
- [x] Parent multi-DB + roles, Unix-socket peer auth, password rotate, subdomain inherit
- [x] Valkey **shared** instance with per-site ACL users restricted to a key prefix (dedicated instance deferred)
- [x] UI to attach and detach a database; cache UI with Valkey
- [x] Embedded [DBAdminer](https://github.com/dbadminer/dbadminer): per-DB Browse `/db/{slug}/` + Edit `/db/{slug}/edit/` (decisions 0004, 0012)

**Exit criteria:** cross-site database and cache isolation tests pass.

**Spike (2026-11-21):** Postgres/MariaDB + multi-DB + Adminer. Valkey is the remaining M4 piece in the native wrap. See `docs/spikes/m4-postgres.md`, `docs/spikes/m4-databases.md`.

### Milestone 5: S3 backups (restic)

**Status:** lab-complete (decision 0006). Replaced containers as the milestone ahead of Podman.

**Deliverables**

- [x] BYO S3-compatible restic repo; credentials in `/etc/webor/backup.env`
- [x] Full rebuild-ready snapshots (config, panel state, `/srv/sites`, DB/Valkey dumps) + `manifest.json`
- [x] Panel UI: configure, run now, schedule, list snapshots, selective site restore (approval-gated)
- [x] systemd timer + retention forget/prune

**Exit criteria:** full backup to an S3-compatible bucket; selective restore of one site on a fresh (or wiped) install. — **met in lab / Alma EC2**

### Milestone 5b: Containers, optional module — **DEFERRED**

**Status:** postponed until after backups (and native wrap). Not required for v1 exit.

**Deliverables** (unchanged when resumed)

- Podman installed on demand, rootless per site user
- Quadlet unit generation, with secure credential storage for registry logins and pulls from Docker/OCI registries
- Container traffic reaching Caddy over a Unix socket or an isolated per-site network. A spike at the start of this milestone settles which
- Volumes under the site directory, resource limits, image updates and rollbacks

**Exit criteria:** a sample container app deploys from a public registry image and passes the isolation tests.

### Milestone 6: Release gate (weeks 15 to 16)

**Status:** complete for public beta (Podman still deferred as M5b)

**Deliverables**

- [x] VPS rehearsal on two providers with real Let's Encrypt issuance (Debian + Alma; two VPS providers + EC2) — repeat before each release
- [x] Footprint benchmarks against the targets in section 12 — idle ~65 MB on 1 GB Debian t3.micro (2026-11-23); see `docs/vps-checklist.md`
- [x] Upgrade and config-migration tests, including upgrading the panel while sites keep serving — panel restart under load on Alma: 0 dropped HTTPS requests (2026-11-23)
- [x] Documentation: install guide, DNS guide, config reference, troubleshooting (`docs/install.md`, `dns-https.md`, `config-reference.md`, `troubleshooting.md`)
- [x] Threat-model review and a public beta tag (`docs/threat-model.md`, tag `v0.1.0-beta.1`)

## 8. After v1

| Phase | Scope | Rough size |
| --- | --- | --- |
| 2. Containers | Rootless Podman / Quadlet (former M5) | 3 to 4 weeks |
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
    aliases: [www.example.com]   # same public/ as parent (multi-host)
    type: static                 # static | app | container
    root: public
    databases:                   # parent-owned; blank id on create → slug(parent)
      - id: example_com          # default = slug(example.com)
        engine: postgres
      - id: notes
        engine: sqlite
    build_command: npm install && npm run build   # optional build step
    limits:
      disk_quota: 2G
      log_max: 200M
  - name: app.example.com
    parent: example.com          # subdomain site: own public/, inherits parent DBs
    type: static
  - name: api.example.com
    type: app
    exec: ./bin/server
    socket_activation: true
    idle_timeout: 15m
    env_file: .env
    databases:
      - id: api_example_com
        engine: postgres
    cache: valkey
    limits:
      memory_max: 256M
      cpu_quota: 100%            # 1c; panel presets only
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
| Install or remove packages (Postgres, Podman, Valkey) | Approval (`InstallPostgres` wired) |
| Ensure or drop a parent database (by id) | SQLite: routine. Postgres/MariaDB: step-up (`EnsureSiteDatabase` / `DropSiteDatabase`, bound `{domain,engine,id}`) |
| Rotate database password | Routine after panel auth (`RotateDatabasePassword`) |
| Change firewall rules | Approval |
| Change the panel port or domain | Approval |
| Delete a site together with its data | Approval |
| Restore a backup | Approval |

**Approval rules:** single-use, expire in minutes, bound to the exact operation and arguments, and verified by the helper, not the web process. The most destructive operations default to console approval through `wbr`, so a fully compromised panel cannot approve its own requests.

## 11. Security hardening and disk safeguards

- Unprivileged panel daemon. The root helper accepts typed JSON over a Unix socket, with the caller verified by `SO_PEERCRED`
- Strict validation of every helper argument, with fuzz tests
- Unix sockets for apps instead of loopback TCP ports
- `hidepid=2` on `/proc`, and `PrivateTmp`, `ProtectSystem` and `NoNewPrivileges` in site units
- Secrets in 0600 files or systemd credentials, never in world-readable config
- Login rate limiting and TOTP, with root recovery through `wbr`
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
| Admin locked out (lost TOTP) | Root recovery through `wbr` |
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
4. Final supported OS list. — Debian 12+, Ubuntu 22.04+, AlmaLinux 9 (lab: Debian 13 + Alma 9 EC2)
5. Domain, package-name and trademark checks for Webor. — **resolved for domain/name** (`webor.dev`); package-name placeholders still open
6. Confirm what "PQPM" refers to. — **resolved:** PQPM is a PM2-style user process manager; Webor uses **systemd** as the site supervisor (PQPM not integrated for v1)
