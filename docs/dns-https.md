# DNS checklist and HTTPS

## Checklist

For each site hostname (and aliases), Webor checks:

- Public/authoritative **A** (and optional **AAAA**) against the VPS addresses (`ExpectedA` / `-expected-a`)
- **CAA** readiness for Let's Encrypt (warn/block via `can_issue_certificate`)

UI: site page → DNS checklist → **Check DNS**.

## Enable HTTPS

Prerequisites:

1. Panel **not** started with `-disable-https` (lab default is disabled).
2. `-expected-a` set to the VPS public IPv4 (comma-separated if needed).
3. DNS checklist green (`can_issue_certificate`).

Then **Enable HTTPS** on the site page. Config stores `https: true` for that site; Caddy reloads with automatic HTTPS (listen `:443` when any site is enabled).

**Disable HTTPS** clears the flag and returns that site to HTTP-only reconciliation when no other sites need TLS.

## Lab

Lab `make lab-start` forces HTTP (`-disable-https`). Enable HTTPS only on a real VPS with ports 80/443 open.
