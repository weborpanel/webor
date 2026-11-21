# Spike A: Caddy admin API

**Status:** accepted

## Context

Webor must drive TLS and routing through Caddy without dropping connections on
reload. The panel needs validate-then-reload behavior and a last-good config
rollback if a bad config is applied. Caddy runs as a separate process
(`webor-caddy.service`), not embedded in the panel binary for v1.

## Decision

Drive Caddy exclusively through its **JSON admin API** (`POST /load`,
`GET /config/`), bound to loopback (`127.0.0.1:2019`).

Apply path:

1. **Preflight** — parse and reject non-object JSON in Webor before calling Caddy.
2. **Load** — `POST /load` with `Content-Type: application/json`. Caddy provisions
   the new config and, if load fails, **keeps the previous live config with zero
   downtime** (built-in rollback).
3. **Probe** (Webor-owned) — optional HTTP check against a site URL after a
   successful load (catches “config loaded but serves 500 / wrong root”).
4. **Last-good file** — on probe success, atomically persist the config under
   `/var/lib/webor/` (spike uses `/tmp/...`). If the probe fails, `POST /load`
   the previous last-good file again.

Do **not** mix Caddyfile-on-disk edits with API loads as dual sources of truth.
Desired state lives in Webor `config.yaml`; rendered Caddy JSON is an
artifact. Prefer full `/load` replacements from the reconcile loop over
fine-grained `/config/...` mutations in v1 (simpler diff/plan/apply).

Admin API stays on loopback only; the unprivileged panel reaches Caddy locally,
never exposed on the public interface.

## Consequences

- Panel restarts do not interrupt sites; only Caddy owns data-plane traffic.
- Bad configs that fail provisioning never replace the live config (Caddy).
- Bad configs that provision but fail health checks are reverted by Webor
  using the last-good file.
- Staging ACME for the M0 exit criterion is configured in rendered JSON later;
  this spike proves reload/rollback on HTTP `:8080` without certificates.
- Implementation: [`internal/providers/caddy`](../../internal/providers/caddy),
  harness [`cmd/spike-caddy`](../../cmd/spike-caddy).

## VM verification (Debian 13)

See [docs/spikes/caddy-admin-api.md](../spikes/caddy-admin-api.md).
