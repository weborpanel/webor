# Milestone 3 — installer and hardening

## What landed

- `install/install.sh`: preflight (OS/systemd, ports 80/443, public IP + PTR/FCrDNS), firewall detection (guidance only), service users, binary + unit install, first-run port/token, idempotent re-run, `--uninstall` / `--purge`
- Hardened units: `install/webor.service`, `webor-helper.service`, `webor-caddy.service`
- Non-interactive: `--yes`, `--config install.yaml` (see `install/install.yaml.example`)
- Step-up: `DeleteSiteUser` with `delete_data` consumes console approval; audit log hash-chain; `wbr approve list|grant`, `wbr doctor`
- Panel: `-setup-token`, `-helper-socket`, optional `-tls-cert`/`-tls-key`; `POST /api/approvals`; `DELETE /api/sites/{name}?delete_data=1&approval_id=`

## Lab install (Debian VM)

```bash
cd /media/sf_webor   # or your checkout
make web build
sudo ./install/install.sh --yes --skip-ptr --bin-dir ./bin
# note URL + setup token from the summary

# Complete first-run in the browser, then:
sudo wbr doctor
sudo wbr approve list
```

## Delete site data (approval path)

```bash
# as panel user / via API after login:
# POST /api/approvals {"operation":"DeleteSiteUser","args":{"domain":"example.com","delete_data":true}}

# or CLI request (any user who can reach the helper socket):
wbr request-approval -op DeleteSiteUser -domain example.com -delete-data

sudo wbr approve list
sudo wbr approve <approval_id>

# then DELETE /api/sites/example.com?delete_data=1&approval_id=<id>
```

## Uninstall

```bash
sudo ./install/install.sh --uninstall          # keep state
sudo ./install/install.sh --uninstall --purge --yes
```

## Notes

- First-run listens on a random high port with HTTP (or self-signed TLS if `--enable-https`). After adding an apex site, use Domains → **Panel access** to publish the panel on a subdomain via Caddy reverse-proxy; once DNS is green, **Restrict public access** binds the panel to `127.0.0.1` (no firewall change required).
- Installer never opens firewall ports automatically.
- Helper socket is `0660` `root:webor` so the panel user can call IPC without being root.
- Helper must not use `ProtectSystem=full` (blocks `useradd`). See `docs/spikes/m3-helper-review.md`.
- `/srv/sites` is `0755` so Caddy can traverse; site `public/` files are world-readable.
