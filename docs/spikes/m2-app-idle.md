# Milestone 2 slice 3 — idle-stop (scale to zero)

Socket-activated apps can exit after an idle period so the `.service` goes
inactive while the `.socket` keeps listening. The next request wakes the app.

## Convention

Config (`idle_timeout`, Go duration, requires `socket_activation: true`):

```yaml
socket_activation: true
idle_timeout: 15m
```

The unit renderer sets `Environment=WBZ_IDLE_TIMEOUT=<value>` and `Restart=no`.
Cooperative apps (`spike-socket-go`, and the Node sample under
`examples/socket-activation/node-app`) read `WBZ_IDLE_TIMEOUT` and shut down
when idle.

Non-cooperative apps use `socket_proxy: true`: public `.socket` activates
`systemd-socket-proxyd` with `--exit-idle-time=` from `idle_timeout`, forwarding
to a private always-on backend (`WBZ_SOCK`). Smoke: `make lab-proxyd`.

Note: Caddy upstream keep-alives can keep proxyd busy; the lab idle check uses
a direct Unix-socket curl so the connection fully closes.

For the full Node smoke (wake + idle + deploy) use `make lab-node`.

## On the Debian VM

Prereq: Caddy up (`make lab-start`).

```bash
cd ~/webooza
git pull
export GOTOOLCHAIN=auto
make lab-idle
```

Expect: after ~3s idle the service is `inactive`, socket stays `active`, then
`curl` wakes the service again.

```text
service after idle: inactive
…
webooza-spike-b-go ok pid=…
service after wake: active
ok: idle-stop + wake verified
```
