# Lab SSH testing — agent handoff

How Cursor agents (and humans) drive the Debian VirtualBox lab over SSH.
VM setup/credentials live in [`virtualbox-debian13.md`](virtualbox-debian13.md). This doc is the **operational** playbook: connect, sync, smoke, and pick up mid-milestone work.

## Why SSH (not shared folder)

| Path | Status |
| --- | --- |
| `/media/sf_*` VirtualBox share | Unreliable here (`Permission denied` / odd folder name `sf_sf_webooza`) |
| `~/webooza` (or `~/webor`) on the guest | **Canonical checkout** — sync with `rsync` over SSH |
| Host workspace | WSL: typically `/home/rp/projects/webooza` (module may already be renamed to `webor`; see Naming) |

Do **not** assume the shared folder is mounted or readable. Always `rsync` before building or installing on the guest.

## Topology

```
Cursor agent (WSL)
    │  Shell tool + required_permissions: ["all"]
    ▼
ssh.exe  (Windows OpenSSH client)
    │  TCP 127.0.0.1:2222  (VirtualBox NAT port forward → guest :22)
    ▼
Debian 13 guest  hostname=debian
    user: webooza   (sudo)
    checkout: ~/webooza
```

WSL’s own `ssh` to `127.0.0.1:2222` usually **fails** (localhost is not the Windows NAT listener). Always use **`ssh.exe`**.

## Connectivity checklist

1. VirtualBox VM **running** (Debian, NAT, port forward `2222 → 22`).
2. From the agent Shell (needs unrestricted/`all` permissions — sandbox blocks `ssh.exe`):

```bash
ssh.exe -p 2222 -o BatchMode=yes -o ConnectTimeout=8 -o StrictHostKeyChecking=accept-new \
  webooza@127.0.0.1 'echo OK; hostname; whoami'
```

Expected: `OK` / `debian` / `webooza`.

3. Interactive (human) from PowerShell: `ssh -p 2222 webooza@127.0.0.1`  
   Password is in [`virtualbox-debian13.md`](virtualbox-debian13.md) (lab only).  
   Agents should prefer **BatchMode** (key-based). If BatchMode fails, the human must add an SSH key or run once interactively.

### Cursor / auto-review notes

- Remote `sudo`, `install.sh`, `systemctl`, and purge commands often trigger **Auto-review**. Retry the **exact** command with smart-mode approval when blocked; do not invent alternate hosts/exfiltration paths.
- Avoid `sudo cat /etc/webooza/panel.env` (or webor equivalent) in logs — it contains the setup token. Read tokens only inside the remote shell for API setup, do not echo them back to the transcript when avoidable.
- Prefer heredocs:

```bash
ssh.exe -p 2222 -o BatchMode=yes webooza@127.0.0.1 'bash -s' <<'EOF'
set -euo pipefail
# guest commands here
EOF
```

## Naming (host vs lab — important)

As of the M3 lab smoke (2026-11-20) and follow-on rename work:

| Layer | What you will see |
| --- | --- |
| Lab guest user | `webooza` (SSH login) |
| Lab binaries/units after M3 smoke | `webooza`, `webooza-helper`, `wbz`, units `webooza*.service`, paths `/etc/webooza`, `/var/lib/webooza` |
| Host workspace (may be mid-rename) | Module/binaries/docs trending to **webor** / **wbr** / `/etc/webor` |

**Before assuming names:** on the guest run `ls /usr/local/bin/webooza* /usr/local/bin/webor* 2>/dev/null; systemctl list-units 'webooza*' 'webor*' --no-pager`.  
After a full rsync + rebuild + reinstall, guest names should match the host tree. Until then, smoke scripts must use whatever is **installed on the guest**.

## Sync host → guest

From the **host** workspace root:

```bash
rsync -az \
  --exclude '.git' \
  --exclude 'node_modules' --exclude 'web/node_modules' \
  --exclude 'bin' \
  --exclude '.cursor' \
  -e 'ssh.exe -p 2222 -o BatchMode=yes' \
  ./ webooza@127.0.0.1:~/webooza/
```

- Use `--delete` only when you intend to mirror deletions (can wipe guest-only files under the checkout).
- Rebuild **on the guest** after sync (`make web` / `make build`) — do not rely on host `bin/` (different OS/paths; guest is the systemd host).

## Two ways to run the stack

### A. Lab Makefile (dev / M0–M2 style)

Fast iteration; units named `*-lab`; state under `/tmp/webooza-lab` (or current Makefile defaults).

```bash
ssh.exe -p 2222 -o BatchMode=yes webooza@127.0.0.1 'bash -s' <<'EOF'
set -euo pipefail
cd ~/webooza
export GOTOOLCHAIN=auto
make lab-stop || true
make web build
make lab-start
make lab-health
EOF
```

Panel typically `:8787`, sites Caddy `:8080`. See Makefile `help`.

### B. Production-shaped install (M3+)

Uses `install/install.sh`, real units, `/etc/webooza` + `/var/lib/webooza` (or webor paths after rename).

```bash
ssh.exe -p 2222 -o BatchMode=yes webooza@127.0.0.1 'bash -s' <<'EOF'
set -euo pipefail
cd ~/webooza
export GOTOOLCHAIN=auto
make web build
# Optional clean slate:
# sudo ./install/install.sh --uninstall --purge --yes
sudo ./install/install.sh --yes --skip-ptr --bin-dir ./bin \
  --panel-port 28787 --listen-host 0.0.0.0
sleep 2
systemctl is-active webooza-helper webooza webooza-caddy   # or webor*
curl -fsS http://127.0.0.1:28787/healthz
EOF
```

First-run: setup token is printed by the installer and stored in `panel.env` on the guest. Complete setup via API (do not paste the token into the agent transcript):

```bash
# On guest — TOKEN read locally, not printed
TOKEN=$(sudo grep '^WBZ_SETUP_TOKEN=' /etc/webooza/panel.env | cut -d= -f2-)
# After rename expect WBOR_* / /etc/webor/panel.env — check the file.
curl -fsS -c /tmp/wbz-cookies -X POST http://127.0.0.1:28787/api/auth/setup \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"$TOKEN\",\"email\":\"admin@example.com\",\"password\":\"correct-horse-battery\"}"
curl -fsS -c /tmp/wbz-cookies -b /tmp/wbz-cookies -X POST http://127.0.0.1:28787/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"correct-horse-battery"}'
# If 2FA is enabled, the login response is { ok, mfa_required, mfa_token } (no session
# cookie). Complete with: POST /api/auth/login/mfa {"mfa_token":"…","totp":"123456"}
# Scripts may still one-shot: include "totp" on the first /api/auth/login body.
```

Lab admin password used in smokes: `correct-horse-battery` / `admin@example.com` (not a production secret).

## Useful guest probes

```bash
# Services
systemctl is-active webooza-helper webooza webooza-caddy
sudo journalctl -u webooza -u webooza-helper -u webooza-caddy -n 40 --no-pager

# Helper socket (panel user must be group webooza)
ls -la /run/webooza/helper.sock

# Site through Caddy (Host header required)
curl -fsS -H 'Host: example.com' http://127.0.0.1:8080/

# Approvals (root)
sudo wbz approve list
sudo wbz doctor
```

From the **Windows host** browser/NAT (optional forwards): panel high port is **not** forwarded by default — probe panel with `curl` **on the guest**, or add a VirtualBox forward for the chosen panel port. `:8080` may already be forwarded (see VM doc).

## M3 lab smoke status (baseline for next agents)

Completed 2026-11-20 on this VM:

1. `install.sh` → helper + panel active; install ~seconds with prebuilt bins  
2. First-run setup + login  
3. `POST /api/sites` static site via **helper IPC** (panel is non-root)  
4. Site served on `:8080` with `Host:` header  
5. `POST /api/approvals` → `wbz approve` → `DELETE ...?delete_data=1&approval_id=`  
6. Audit hash-chain rows present  

Review notes: [`docs/spikes/m3-helper-review.md`](../spikes/m3-helper-review.md)  
Install spike: [`docs/spikes/m3-install.md`](../spikes/m3-install.md)

### Pitfalls already fixed (do not reintroduce)

- Helper `ProtectSystem=full` → blocks `useradd` → use `ProtectSystem=true`
- Helper `UMask=0077` → Caddy 403 on `public/` → no restrictive umask; `chmod 0644` after write
- `/etc/<panel>/` must be traversable by user `caddy` (e.g. `0751`); `caddy.json` readable
- `/srv/sites` must be `0755` so Caddy can traverse
- Caddy unit: `caddy run --config …` **without** `--adapter json`
- Shared SQLite: helper opens as root first → chown state DB to panel user
- Panel must use helper IPC (`-helper-socket`) for privileged ops when running as `User=webooza`

## Suggested agent workflow (next session)

1. Connectivity probe (`ssh.exe … echo OK`).
2. Note naming: host tree vs guest installed units/binaries.
3. `rsync` host → `webooza@127.0.0.1:~/webooza/`.
4. On guest: `make web build` (or targeted `go build`).
5. Either `make lab-*` **or** reinstall with `install/install.sh` (stop the other stack first to avoid port/unit fights).
6. Run the milestone spike doc under `docs/spikes/`.
7. Update `docs/plan.md` progress note when exit criteria are met.

## Quick reference

| Item | Value |
| --- | --- |
| SSH | `ssh.exe -p 2222 -o BatchMode=yes webooza@127.0.0.1` |
| Guest checkout | `~/webooza` |
| Host checkout | WSL `/home/rp/projects/webooza` |
| Shell permissions | `required_permissions: ["all"]` for SSH/rsync |
| Panel smoke port (M3) | `28787` on guest loopback |
| Site HTTP | `8080` + `Host:` header |
| Plan | [`docs/plan.md`](../plan.md) |
| VM setup | [`docs/dev/virtualbox-debian13.md`](virtualbox-debian13.md) |
