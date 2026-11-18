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
Cooperative apps (like `spike-socket-go`) read `WBZ_IDLE_TIMEOUT` and shut down
when idle. Non-cooperative apps need the proxyd/`--exit-idle-time=` path (still open).

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
