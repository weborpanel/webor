# Config reference (sketch)

File: `/etc/webor/config.yaml` (lab: `/tmp/webor-lab/etc/config.yaml`).

```yaml
version: 1
panel:
  domain: ""          # e.g. panel.example.com — Caddy reverse-proxies to the panel listen port
  port: 0             # usually set from the panel -listen port when domain is enabled
sites:
  - name: example.com
    aliases: [www.example.com]   # same docroot
    type: static                 # static | app | container
    root: public
    https: false                 # set true after DNS green (panel enables TLS)
    databases:
      - id: example_com          # default = slug(name); blank on create → this
        engine: postgres         # sqlite | postgres | mariadb
      - id: notes
        engine: sqlite
    cache: valkey                # shared Valkey ACL; omit if unused
    limits:
      memory_max: 256M           # also caps builds (OOM → clear panel error)
      cpu_quota: 100%            # systemd CPUQuota; 100% = 1 core (panel: 1c, 2c, …)
      disk_quota: 10M            # default for new sites; panel presets 10M–10G
      log_max: 200M
  - name: app.example.com
    parent: example.com          # subdomain: own public/, inherits DBs + cache
    type: static
  - name: api.example.com
    type: app
    exec: ./bin/server
    socket_activation: true
    idle_timeout: 15m
    cron:
      - name: seed
        # schedule optional — omit for Run once only (seed/migrate)
        exec: ./crons/seed/server
      - name: nightly
        schedule: 0 3 * * *   # 5-field cron → systemd OnCalendar timer
        exec: ./crons/nightly/cleanup
    jobs:                         # long-running processes (queues/workers)
      - name: worker
        exec: ./processes/worker/server
```

Rules:

- `panel.domain` must not collide with a site name or alias. Set it from the apex site’s Domains → **Panel access** UI (not as a child site).
- Children (`parent:`) cannot declare `databases` or `cache`.
- Blank database `id` on create becomes `slug(parent)`.
- Passwords are not stored in this file; they live under `state/secrets/`.
- `cron:` oneshots live under `crons/<name>/`; `jobs:` (Processes UI) under `processes/<name>/`. Both need systemd; timers need `timers.target` (`wbr doctor` checks it).
- Panel tabs **Crons** / **Processes** edit these via HTTP API (no step-up approval).
