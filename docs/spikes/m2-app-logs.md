# Site and host file logs

Application, cron, job, API, access, and backup logs are **file-based** under:

- `/srv/sites/<domain>/logs/` — `access.log`, `app.log`, `backend.log`, `api-<id>.log`, `cron-<job>.log`, `job-<job>.log`
- `/var/log/webor/` — `backup.log`

systemd units append stdout/stderr to the matching stream file. Caddy writes
per-site JSON access logs to `access.log`. Rotation runs hourly via
`webor-logrotate.timer` → `wbr logs rotate` (date change and/or size ≥
`limits.log_max`, default `50M`; host backup default `100M`). Rotated names:
`<stream>.log.<YYYY-MM-DD>` and `.N` for same-day splits. Retention: 14 days.

Panel APIs (auth):

```http
GET /api/sites/{name}/logs?unit=app|backend|access|api:<id>|cron:<job>|job:<name>&lines=100
GET /api/sites/{name}/log-files
GET /api/sites/{name}/log-files/{file}?lines=200
GET /api/system/log-files
GET /api/system/log-files/{file}?lines=200
```

UI: site **Logs** (stream + date/file browser); host **Logs** (backup).
`backup.include_logs` (default false) optionally stages site `logs/` and
`/var/log/webor` into restic snapshots.

Journal tails remain a fallback when a current file is missing. Global journald
`SystemMaxUse` remains an installer disk safeguard (separate from file rotation).
