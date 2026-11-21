# Config reference (sketch)

File: `/etc/webor/config.yaml` (lab: `/tmp/webor-lab/etc/config.yaml`).

```yaml
version: 1
panel:
  domain: ""          # optional panel hostname
  port: 0             # 0 = high port until domain set
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

- Children (`parent:`) cannot declare `databases` or `cache`.
- Blank database `id` on create becomes `slug(parent)`.
- Passwords are not stored in this file; they live under `state/secrets/`.
