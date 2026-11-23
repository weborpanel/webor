# Install Webor (native VPS)

Target: Debian 12+/13, Ubuntu 22.04+, or AlmaLinux 9 with systemd. No Docker for the core path.

AlmaLinux 9 notes: install Caddy to `/usr/bin/caddy` (or use the public bootstrap, which downloads the official binary). SELinux Enforcing is supported for panel + static sites; see [dev/almalinux9-ec2.md](dev/almalinux9-ec2.md). Package installs use `dnf` automatically.

## Quick path (public bootstrap)

```bash
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash
# no PTR / lab:
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash -s -- --skip-ptr
```

Releases: [guides/release-s3.md](guides/release-s3.md).

## From a git checkout

```bash
cd /path/to/webor
make web build
sudo ./install/install.sh --yes --skip-ptr --bin-dir ./bin
```

First boot prints a one-time setup token. Open the panel URL (high port until you set a panel domain), complete setup (email + password), then log in.

## After setup

1. **Add a site** (Sites → Add) — creates the Linux user and `/srv/sites/<domain>/`.
2. **Panel access (optional):** on the apex site → **Domains** → **Panel access**, choose a label (`panel`, `cp`, …) and Enable. Point A/AAAA for `label.yourdomain` at the VPS, use **Check DNS**, then open that hostname. Caddy reverse-proxies to the panel; the high port remains a fallback until you close it in the firewall.
3. **Static:** use the site **Files** tab to browse `public/`, upload, and extract zip/tar/tar.gz. SQLite lives under `data/`, not in Files. **App:** deploy binary or git from Deploy; Files still manages `public/` assets.
4. **DNS:** point A/AAAA at the VPS; use the site DNS checklist until it is green.
5. **HTTPS:** on a real VPS, run the panel **without** `-disable-https` and with `-expected-a <public-ip>`. When DNS is green, click **Enable HTTPS** on the site page.
6. **Database:** sqlite adds immediately. For Postgres/MariaDB, install the server as root first (panel shows distro-specific commands: `apt` on Debian/Ubuntu, `dnf` on Alma/RHEL), then Add and approve with `sudo wbr approve <id>`. Password is shown once.
7. **Cache:** install Valkey or Redis as root (panel hint), then parent site → Cache → Attach. Password shown once; keys under `wbr_<slug>:`.
8. **API backend (static sites, including subdomains):** site → **API** → Attach. Deploy a binary that binds to `PORT` (and `HOST`); Webor injects those into the unit — you never pick a port. Use path `/api` on the site, or `/` for a whole-host API (`api.example.com`). Optional extra paths (e.g. `/ws`) and WebSocket toggle are panel-only.
9. **Crons / Processes:** site → **Crons** (one-shot seed/migrate + optional schedule) or **Processes** (background queues). Artifacts land in `crons/<name>/` and `processes/<name>/` (not under `public/`). Needs systemd; `wbr doctor` checks `timers.target` for scheduled cron.
10. **Backups:** install restic as root (`apt install restic` or `dnf install epel-release restic`). Panel → **Backups** → configure S3-compatible bucket + password. Run a full snapshot or set a schedule; restore picks sites from a snapshot (requires `sudo wbr approve`). After restore, Webor reloads Caddy and re-ensures app/backend units so HTTPS and routes match the restored config.

## Lab (VirtualBox)

```bash
make lab-stop; make web build; make lab-start
```

Panel: `http://127.0.0.1:8787/` (NAT forward). Lab keeps HTTPS disabled; use Host-header HTTP on `:8080` for sites.

## Upgrade / restart

```bash
sudo systemctl restart webor-helper webor webor-caddy
# or lab:
make lab-restart
```

Sites keep serving while the panel restarts; Caddy and site units are independent.
