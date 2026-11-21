# Install Webor (native VPS)

Target: Debian 12+/13 or Ubuntu 22.04+ with systemd. No Docker for the core path.

## Quick path

```bash
# On the VPS, as a sudo user with a checkout of webor:
cd /path/to/webor
make web build
sudo ./install/install.sh --yes --skip-ptr --bin-dir ./bin
```

First boot prints a one-time setup token. Open the panel URL (high port until you set a panel domain), complete setup (email + password), then log in.

## After setup

1. **Add a site** (Sites → Add) — creates the Linux user and `/srv/sites/<domain>/`.
2. **Static:** use the site **Files** tab to browse `public/`, upload, and extract zip/tar/tar.gz. SQLite lives under `data/`, not in Files. **App:** deploy binary or git from Deploy; Files still manages `public/` assets.
3. **DNS:** point A/AAAA at the VPS; use the site DNS checklist until it is green.
4. **HTTPS:** on a real VPS, run the panel **without** `-disable-https` and with `-expected-a <public-ip>`. When DNS is green, click **Enable HTTPS** on the site page.
5. **Database:** parent site → Databases → add sqlite (immediate) or postgres/mariadb (approve with `sudo wbr approve <id>`). Password is shown once.
6. **Cache:** install Valkey as root (`sudo apt-get install -y valkey`), then parent site → Cache → Attach. Password shown once; keys under `wbr_<slug>:`.

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
