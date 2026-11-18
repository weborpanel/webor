# Milestone 2 slice 1 — app units + socket activation

Applies `lab-app.yaml`: create site user, install a LISTEN_FDS-aware Go binary,
write matched `.socket`/`.service` units, enable the socket, render Caddy
`reverse_proxy` to the site Unix socket, reload Caddy.

## On the Debian VM

Prereq: Caddy admin API up (e.g. `make lab-start`).

```bash
cd ~/webooza
export GOTOOLCHAIN=auto
make lab-app
```

Expect:

```text
webooza-spike-b-go ok pid=…
service before: inactive
service after:  active
```

Manual checks:

```bash
systemctl is-active webooza-app-app-example-com.socket   # active
systemctl stop webooza-app-app-example-com.service
curl -sS -H 'Host: app.example.com' http://127.0.0.1:8080/
systemctl is-active webooza-app-app-example-com.service  # active
ls -l /run/webooza/sites/app.example.com/app.sock
```

Cleanup:

```bash
sudo systemctl disable --now webooza-app-app-example-com.socket webooza-app-app-example-com.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/webooza-app-app-example-com.*
sudo systemctl daemon-reload
```
