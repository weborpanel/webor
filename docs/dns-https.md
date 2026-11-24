# DNS checklist and HTTPS

Point DNS at your VPS yourself (Webor does not manage provider APIs in v1). The panel only checks records and drives Caddy once they look correct.

## Checklist

For each site hostname (and aliases), Webor checks:

- Public/authoritative **A** (and optional **AAAA**) against the VPS addresses (`ExpectedA` / `-expected-a`)
- **CAA** readiness for Let's Encrypt (warn/block via `can_issue_certificate`)

UI: site page → DNS checklist → **Check DNS**.

### Typical records

| Host | Type | Value |
| --- | --- | --- |
| `example.com` | A | VPS public IPv4 |
| `www.example.com` | A or CNAME | Same IPv4, or `example.com` |
| `panel.example.com` (or `cp`, …) | A | Same IPv4 (only if using Panel access) |
| (optional) same names | AAAA | VPS public IPv6 |

Keep **port 80** open for ACME HTTP-01 even after HTTPS is on. Port **443** serves the sites.

## Enable HTTPS

Prerequisites:

1. Panel **not** started with `-disable-https` (lab default is disabled).
2. `-expected-a` set to the VPS public IPv4 (comma-separated if needed). The public bootstrap sets this from the detected address.
3. DNS checklist green (`can_issue_certificate`).

Then **Enable HTTPS** on the site page. Config stores `https: true` for that site; Caddy reloads with automatic HTTPS (listen `:443` when any site is enabled).

**Disable HTTPS** clears the flag and returns that site to HTTP-only reconciliation when no other sites need TLS.

### Panel on a hostname

On the apex site → **Domains** → **Panel access**: pick a label (`panel`, `cp`, …), Enable, point DNS, **Check DNS**, then open that hostname. Caddy reverse-proxies to the panel. Leave the high port open until you confirm the hostname works, then restrict it in the firewall if you want.

## Lab

Lab `make lab-start` forces HTTP (`-disable-https`). Enable HTTPS only on a real VPS with ports 80/443 open.

## Failures

See [troubleshooting.md](troubleshooting.md) (DNS and HTTPS section).
