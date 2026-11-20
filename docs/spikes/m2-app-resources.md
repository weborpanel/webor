# Milestone 2 — site resource snapshot

```http
GET /api/sites/{name}/resources
```

Returns disk used under `/srv/sites/<name>`, filesystem free space, and
`MemoryCurrent` / optional CPU from the app (or proxyd backend) unit via
`systemctl show` and cgroup `cpu.stat` when available.

UI: site detail → **Resources** strip.
