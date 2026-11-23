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
      disk_quota: 2G
      log_max: 200M
  - name: app.example.com
    parent: example.com          # subdomain: own public/, inherits DBs + cache
    type: static
  - name: api.example.com
    type: app
    exec: ./bin/server
    socket_activation: true
    idle_timeout: 15m
```

Rules:

- `panel.domain` must not collide with a site name or alias. Set it from the apex site’s Domains → **Panel access** UI (not as a child site).
- Children (`parent:`) cannot declare `databases` or `cache`.
- Blank database `id` on create becomes `slug(parent)`.
- Passwords are not stored in this file; they live under `state/secrets/`.
