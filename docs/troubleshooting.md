# Troubleshooting

Quick fixes for common Webor install and runtime issues. Paths assume a production install (`/etc/webor`, `/var/lib/webor`, `/srv/sites`).

## First checks

```bash
sudo wbr doctor
sudo systemctl status webor webor-helper webor-caddy --no-pager
sudo journalctl -u webor -u webor-helper -u webor-caddy -n 80 --no-pager
# Site app/cron/API/access logs (files): panel → site Logs, or:
#   /srv/sites/<domain>/logs/
# Backup runs: panel → Logs, or /var/log/webor/backup.log
```

Confirm the panel listen port (sidebar or `systemctl show webor -p Environment`) and that the helper socket exists under `/var/lib/webor/`.

## Install / first boot

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Bootstrap curl fails | Network, DNS, or S3 unreachable | Retry; or download the tarball from [guides/release-s3.md](guides/release-s3.md) and `sudo ./install/install.sh --yes --bin-dir .` |
| Install aborts on PTR | Lab / dynamic IP without reverse DNS | Re-run with `--skip-ptr` |
| No setup token printed | Panel already initialized, or journal scrolled away | `sudo journalctl -u webor -b --no-pager \| grep -i token`; if already set up, use login or `sudo wbr recover` |
| Setup URL refuses connection | Firewall or wrong port | Open the high panel port; check `systemctl status webor` |

## DNS and HTTPS

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Check DNS stays red | A/AAAA not pointing at this VPS | Set records to the public IP; wait for TTL; re-check. Panel needs `-expected-a <public-ip>` |
| CAA blocks issuance | Restrictive CAA without Let's Encrypt | Add a CAA allowing `letsencrypt.org`, or remove CAA |
| Enable HTTPS greyed out / fails | Panel started with `-disable-https`, or DNS not green | Drop `-disable-https` on real VPS; fix checklist first |
| Browser shows HTTP-only after enable | Caddy reload failed, or port 443 closed | `sudo systemctl status webor-caddy`; open 80/443; `sudo journalctl -u webor-caddy -n 50` |
| Cert renewals fail | Port 80 blocked, or DNS drifted | Keep 80 open for ACME; re-run DNS check |

See [dns-https.md](dns-https.md).

## Panel access on a domain

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `panel.example.com` 502 / connection refused | Panel unit down, or panel domain not reconciled | `sudo systemctl restart webor`; confirm Domains → Panel access is Enabled |
| Locked out after Restrict public access | DNS/Caddy broken while panel is localhost-only | On the VPS: `sudo wbr panel listen -host 0.0.0.0`, then open `http://IP:PORT` from sidebar/`panel.env` |
| High port works, hostname does not | DNS for the panel label not green | Point A/AAAA for the label; Check DNS |
| Want IP:port again after locking | Intentional localhost bind | Domains → Disable Panel access, or `sudo wbr panel listen -host 0.0.0.0` |

## Sites and deploys

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Site 403 on static files | Permissions under `public/` | Deploy/upload again via the panel (helper sets modes); avoid root-owned files in `public/` |
| Build OOM / “killed” | `memory_max` too low for the build | Settings → raise Memory; rebuild. Other sites should keep serving |
| App / API never listens | Binary not binding `PORT`/`HOST`, or unit failed | Check site journal in the panel; for API backends ensure the process uses the injected `PORT` |
| Cron timer missing | `timers.target` unavailable | `sudo wbr doctor`; enable systemd timers |
| Cron / Node app `status=203/EXEC` or `Permission denied` on `/var/lib/webor/runtimes/node/…` | State dir not traversable (`0750`/`0770`), or SELinux `var_lib_t` on the binary | `sudo chmod 755 /var/lib/webor`; on Enforcing also `sudo chcon -R -t bin_t /var/lib/webor/runtimes/node/*/bin`. Fixed in ≥0.1.0-beta.2 (helper no longer re-locks the state dir on restart) |
| Delete site with data stuck pending | Approval not consumed | On the VPS: `sudo wbr approvals` then `sudo wbr approve <id>` |

## Databases and cache

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Add Postgres/MariaDB asks for install | Server packages not present | Run the distro commands shown in the panel as root, or approve the helper install request with `sudo wbr approve <id>` |
| Password lost | Shown once at create/rotate | Rotate from the panel; update app secrets |
| Cache attach fails | Valkey/Redis not installed or not running | Install per panel hint; check `systemctl status valkey` / `redis` |

## Backups

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Configure / run fails | Missing `restic`, bad S3 creds, or wrong endpoint | `apt install restic` or `dnf install epel-release restic`; verify bucket/region/keys |
| Restore pending | Destructive op needs console approval | `sudo wbr approve <id>` |
| After restore, routes wrong | Rare reconcile miss | `sudo systemctl reload webor-caddy` or restart panel; Webor normally reloads Caddy and re-ensures units after restore |

## Upgrades

Re-run the public bootstrap (or `install.sh` from a checkout). Units and sites keep serving while `webor` restarts; Caddy is independent.

```bash
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash
# beta:
curl -fsSL https://webor-releases-360386317256.s3.ap-south-1.amazonaws.com/webor/install | sudo bash -s -- --channel beta
# or from git:
make web build && sudo ./install/install.sh --yes --skip-ptr --bin-dir ./bin
sudo systemctl restart webor-helper webor
```

`WEBOR_CHANNEL=beta curl … | sudo bash` still downloads **stable** — the env var is only set for `curl`, not for `sudo bash`. Use `--channel beta` after `bash -s --`.

Prefer in-place updates:

```bash
wbr update check
sudo wbr update apply
```

Channel is read from `WBR_CHANNEL` in `/etc/webor/panel.env` (set by bootstrap `--channel`). Account → Updates can check and request approval; apply stays root CLI.

If the UI looks stale, hard-refresh the browser. Version appears in the panel sidebar (`Webor <version>`).

## Recovery

| Need | Command |
| --- | --- |
| Reset admin / unlock after lockout | `sudo wbr recover` (follow prompts; TOTP re-enroll if used) |
| Pending approvals | `sudo wbr approvals` / `sudo wbr approve <id>` |
| Config sanity | Inspect `/etc/webor/config.yaml`; see [config-reference.md](config-reference.md) |

## Still stuck

Collect `wbr doctor` output, the three unit journals, Webor version from the sidebar or `webor -version` if available, and whether this is Debian/Ubuntu or Alma. Prefer private reports for anything that looks like privilege escalation — see [SECURITY.md](../SECURITY.md).
