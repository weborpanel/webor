# AlmaLinux 9 EC2 lab

Supported OS (with Debian 12+ / Ubuntu 22.04+). Smoke and regression on EL9 + SELinux.

## Instance used (2026-11-22)

| Field | Value |
|-------|--------|
| Region | `ap-south-1` |
| Instance | `i-024894af6b68fe0af` (`webor-alma9-test`) |
| AMI | `ami-0e83fc5bf703f432c` (AlmaLinux OS 9.9) |
| Type | `t3.small` |
| User | `ec2-user` |
| Key | `webor-dev` (matches local `~/.ssh/id_ed25519`) |
| SG | `sg-091aa3641d0e58edb` (`webor-vps-test`) |
| SELinux | **Enforcing** (no AVC denials during panel + static site smoke) |

## Launch (same VPC as Ubuntu lab)

```bash
export AWS_DEFAULT_REGION=ap-south-1
aws ec2 run-instances \
  --image-id ami-0e83fc5bf703f432c \
  --instance-type t3.small \
  --key-name webor-dev \
  --security-group-ids sg-091aa3641d0e58edb \
  --subnet-id subnet-064626356d68dd0ea \
  --associate-public-ip-address \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=webor-alma9-test}]'
```

## Prereqs on the guest

Minimal AMI has no firewalld (AWS SG is enough). Caddy: place at `/usr/bin/caddy`, or let `install/public/install` download the official binary.

Optional engines (helper `Install*` also supports `dnf`):

```bash
sudo dnf install -y postgresql-server postgresql
sudo postgresql-setup --initdb
sudo systemctl enable --now postgresql

sudo dnf install -y mariadb-server && sudo systemctl enable --now mariadb
# socket is typically /var/lib/mysql/mysql.sock

sudo dnf install -y redis && sudo systemctl enable --now redis

sudo dnf install -y epel-release && sudo dnf install -y restic
```

## Sync + install

```bash
make web build   # on workstation
rsync -az bin install ec2-user@$IP:~/webor/
ssh ec2-user@$IP "sudo ~/webor/install/install.sh --yes --skip-ptr --bin-dir ~/webor/bin --expected-a $IP"
```

Installer links `/usr/bin/wbr` so Alma `sudo secure_path` can find `wbr`.

### Node backends + SELinux

`install.sh` sets `/var/lib/webor` to **0755** (site users must traverse to `/var/lib/webor/runtimes/node/…`) and, when Enforcing, labels existing Node bins `bin_t` / libs `lib_t`. The helper reapplies the same after every `EnsureNodeRuntime` download so new versions do not stay `var_lib_t` (which yields systemd `status=203/EXEC`). Helper `OpenDB` must also keep the state dir at **0755** (not `0770`) — otherwise every `systemctl restart webor-helper` locks site users out until a cron recreate runs `prepareNodeRuntimeAccess` again.

## Smoke checklist

- [x] `install.sh` on Alma 9.9
- [x] `webor` / `webor-helper` / `webor-caddy` active under SELinux Enforcing
- [x] Panel HTTP 200 on high port
- [x] Admin setup + login
- [x] Static site create + Caddy `:8080` Host-header serve
- [x] Node HTTP backend under Enforcing (state dir 0755 + `bin_t` on runtime)
- [x] DNS / Let's Encrypt HTTPS
- [x] Panel restart while site HTTPS traffic continues (2026-11-23: 40/40 requests OK to fakerequest.com)
- [ ] Postgres / MariaDB / Redis via helper or manual dnf
- [ ] restic backup
