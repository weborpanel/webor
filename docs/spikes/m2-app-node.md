# Milestone 2 — Node sample (wake, idle-stop, deploy)

Runs the cooperative Node socket sample
(`examples/socket-activation/node-app/server.js`) as `app.example.com`'s
`bin/server` via the same apply/deploy path as the Go sample.

The script:

- inherits systemd's listening FD (`LISTEN_FDS` / fd 3)
- honors `WBZ_IDLE_TIMEOUT` (Go-style duration) and exits when idle
- responds with `webooza-spike-b-node ok pid=…`

## On the Debian VM

Prereq: Caddy + helper up (`make lab-start`), Node installed (`nodejs`).

```bash
cd ~/webooza
git pull
export GOTOOLCHAIN=auto
make lab-node
```

Expect wake → idle scale-to-zero → wake → good deploy → bad deploy rolls back:

```text
webooza-spike-b-node ok pid=…
service after idle: inactive
…
ok: deploy + rollback verified
ok: node wake + idle-stop + deploy/rollback verified
```

Manual checks:

```bash
curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/
head -1 /srv/sites/app.example.com/bin/server
systemctl status webooza-app-app-example-com.socket webooza-app-app-example-com.service
```

Note: `make lab-app` / `lab-deploy` / `lab-idle` reinstall the Go sample on the
same domain; use `lab-node` when verifying the Node path.
