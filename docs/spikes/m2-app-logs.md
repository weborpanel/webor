# Milestone 2 — site journal viewer

Helper `JournalTail` / `SiteJournalTail` tails `journalctl -u` for allowlisted
units. Panel API (auth):

```http
GET /api/sites/{name}/logs?unit=app|backend|cron:<job>|job:<name>&lines=100
```

UI: site detail → **Logs** panel (unit picker + refresh).

`limits.log_max` currently adds per-unit `LogRateLimit*` hints; global journald
`SystemMaxUse` remains an installer concern (M3).
