# VirtualBox lab — Debian 13 (netinst)

Lab VM for Webooza/Webor spikes (helper, Caddy, systemd). Not production.

**Agent / SSH testing playbook:** [`lab-ssh-testing.md`](lab-ssh-testing.md) (connect with `ssh.exe`, rsync, smoke commands, M3 baseline, pitfalls).

**Snapshot:** take a clean snapshot after this setup (before installing Caddy/Go extras you may want to roll back).

## Credentials (lab only)

| Item | Value |
| ---- | ----- |
| Guest user | `webor` |
| Password | `789456` |
| Root password | same lab password if you set it during install |
| Hostname | `debian` (or as installed) |

Do not reuse this password outside the local VM.

## VM settings

- OS: **Debian 13 (Trixie) amd64 netinst** ISO  
  https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/
- RAM: 2 GB (or more)
- CPUs: 2
- Disk: ~20 GB
- Network: **NAT**
- Port forwarding (Adapter 1 → Advanced → Port Forwarding):

| Name | Protocol | Host IP | Host Port | Guest IP | Guest Port |
| ---- | -------- | ------- | --------- | -------- | ---------- |
| ssh  | TCP      | *(blank)* | **2222** | *(blank)* | **22** |
| http | TCP      | *(blank)* | **8080** | *(blank)* | **8080** |  (optional, for Spike A probe)

### Installer choices

- No desktop environment (server-style)
- **SSH server** enabled
- Standard system utilities enabled
- Create user `webor` with password above

## SSH access

### From Windows PowerShell (works with NAT forward)

```powershell
ssh -p 2222 webor@127.0.0.1
```

### From WSL

Linux `ssh` to `127.0.0.1:2222` often fails (WSL and Windows do not share localhost). Use Windows OpenSSH instead:

```bash
ssh.exe -p 2222 webor@127.0.0.1
```

Optional: enable WSL mirrored networking in `%UserProfile%\.wslconfig`:

```ini
[wsl2]
networkingMode=mirrored
```

Then `wsl --shutdown`, reopen WSL, and retry `ssh -p 2222 webor@127.0.0.1`.

The guest NAT address is typically `10.0.2.15`; host appears as `10.0.2.2`. Do not expect the guest on your LAN IP unless you switch to Bridged networking.

## sudo

```bash
sudo usermod -aG sudo webor
```

Log out of SSH and log back in, then:

```bash
groups          # includes sudo
sudo whoami     # root
```

## Shared folder (repo from the host)

Share the Webor checkout so the guest does not need rsync.

1. VirtualBox → Devices → Shared Folders → Add  
   - **Folder Path:** Windows path to the repo, e.g. `C:\Coding\webor` or a copy synced from WSL  
     Prefer a real Windows path over `\\wsl$\...` if mounts are flaky.  
   - **Folder Name:** `webor`  
   - Auto-mount: yes  
   - Make permanent: yes  

2. On the guest:

```bash
sudo apt update
sudo apt install -y virtualbox-guest-utils
# If that is too old for your VirtualBox version, install Guest Additions from the CD instead.

sudo usermod -aG vboxsf webor
```

3. **Fully disconnect SSH and reconnect** (group membership only applies to new sessions).

```bash
groups | grep vboxsf
ls /media/sf_webor
```

`newgrp vboxsf` only fixes the **current** shell; you should not need it after a fresh login.

Optional symlink:

```bash
ln -s /media/sf_webor ~/webor
```

### Permissions note

Mount is typically `root:vboxsf` mode `770`. Without `vboxsf` membership, only root can `ls` the folder.

## Quiet apt (if using the DVD leftover)

If `apt update` complains about `cdrom://`:

```bash
sudo sed -i '/^deb cdrom:/s/^/# /' /etc/apt/sources.list
sudo apt update
```

## After the clean snapshot

1. Install Go (official tarball matching `go.mod`), Caddy (Cloudsmith), Node/npm, make, and `gh` on the guest.
2. Prefer an isolated checkout: `gh repo clone webor/webor ~/webor` (shared folder optional).
3. Lab panel stack from the guest:

```bash
cd ~/webor
make lab-start   # builds UI + binaries, starts helper/caddy/panel
```

Open on the host via NAT forwards: `http://127.0.0.1:8787/` (panel UI), `http://127.0.0.1:8787/healthz`, and sites on `:8080` with a `Host` header.

4. Spike harnesses remain documented under `docs/spikes/`. Host-only Docker smoke: `./scripts/spike-caddy-docker.sh` (does not replace systemd spikes).

## Restore

VirtualBox → Snapshots → restore the clean snapshot taken after SSH, sudo, shared folder, and Guest Additions work.
